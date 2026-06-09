package topic

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Turn struct {
	Input    string `json:"input"`
	Response string `json:"response"`
	Time     string `json:"time"`
}

type Topic struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TopicList struct {
	Topics  []Topic `json:"topics"`
	Current string  `json:"current"`
	Previos string  `json:"previous"`
}

type TopicData struct {
	Topic
	Turns []Turn `json:"turns"`
}

type Manager struct {
	dataDir string
	list    *TopicList
	mu      sync.Mutex
}

func NewManager(dataDir string) *Manager {
	dir := filepath.Join(dataDir, "topics")
	os.MkdirAll(dir, 0755)

	m := &Manager{
		dataDir: dataDir,
		list:    &TopicList{},
	}

	m.loadTopics()
	if len(m.list.Topics) == 0 {
		m.migrateOrCreate()
	}

	return m
}

func (m *Manager) migrateOrCreate() {
	summaryPath := filepath.Join(m.dataDir, "conversation", "summary.json")
	if data, err := os.ReadFile(summaryPath); err == nil {
		var old struct {
			Turns   []Turn `json:"turns"`
			Overall string `json:"overall"`
		}
		if json.Unmarshal(data, &old) == nil {
			title := "Migrated conversation"
			if len(old.Turns) > 0 {
				title = old.Turns[0].Input
				if len(title) > 60 {
					title = title[:60]
				}
			}
			id := generateID()
			now := time.Now().Format(time.RFC3339)
			t := Topic{
				ID:        id,
				Title:     title,
				CreatedAt: now,
				UpdatedAt: now,
			}
			m.list.Topics = []Topic{t}
			m.list.Current = id

			td := TopicData{Topic: t, Turns: old.Turns}
			m.writeTopicJSON(&td)
			m.writeTopicMD(&td)
			m.saveTopics()

			os.Rename(summaryPath, summaryPath+".migrated")
			return
		}
	}

	m.createDefault()
}

func (m *Manager) createDefault() {
	id := generateID()
	now := time.Now().Format(time.RFC3339)
	t := Topic{
		ID:        id,
		Title:     "Topic 1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.list.Topics = []Topic{t}
	m.list.Current = id
	m.saveTopics()

	td := TopicData{Topic: t}
	m.writeTopicJSON(&td)
	m.writeTopicMD(&td)
}

func (m *Manager) topicsPath() string {
	return filepath.Join(m.dataDir, "topics", "topics.json")
}

func (m *Manager) topicJSONPath(id string) string {
	return filepath.Join(m.dataDir, "topics", id+".json")
}

func (m *Manager) topicMDPath(id string) string {
	return filepath.Join(m.dataDir, "topics", id+".md")
}

func (m *Manager) loadTopics() error {
	data, err := os.ReadFile(m.topicsPath())
	if err != nil {
		return err
	}
	return json.Unmarshal(data, m.list)
}

func (m *Manager) saveTopics() error {
	sort.Slice(m.list.Topics, func(i, j int) bool {
		return m.list.Topics[i].UpdatedAt > m.list.Topics[j].UpdatedAt
	})
	data, err := json.MarshalIndent(m.list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.topicsPath(), data, 0644)
}

func (m *Manager) CurrentTopic() *Topic {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentTopic()
}

func (m *Manager) currentTopic() *Topic {
	for i := range m.list.Topics {
		if m.list.Topics[i].ID == m.list.Current {
			return &m.list.Topics[i]
		}
	}
	return nil
}

func (m *Manager) CreateTopic(title string) *Topic {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	now := time.Now().Format(time.RFC3339)

	t := Topic{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	oldCurrent := m.list.Current
	m.list.Topics = append(m.list.Topics, t)
	m.list.Previos = oldCurrent
	m.list.Current = id
	m.saveTopics()

	td := TopicData{Topic: t}
	m.writeTopicJSON(&td)
	m.writeTopicMD(&td)

	return &t
}

func (m *Manager) ForkTopic(title string) *Topic {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentTurns := m.readTopicTurns(m.list.Current)

	id := generateID()
	now := time.Now().Format(time.RFC3339)

	t := Topic{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	oldCurrent := m.list.Current
	m.list.Topics = append(m.list.Topics, t)
	m.list.Previos = oldCurrent
	m.list.Current = id
	m.saveTopics()

	td := TopicData{Topic: t, Turns: currentTurns}
	m.writeTopicJSON(&td)
	m.writeTopicMD(&td)

	return &t
}

func (m *Manager) ResumeTopic() *Topic {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.list.Previos == "" {
		return nil
	}

	oldCurrent := m.list.Current
	m.list.Current = m.list.Previos
	m.list.Previos = oldCurrent
	m.saveTopics()

	return m.currentTopic()
}

func (m *Manager) SwitchToTopic(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.list.Topics {
		if t.ID == id {
			if m.list.Current != id {
				m.list.Previos = m.list.Current
				m.list.Current = id
				m.saveTopics()
			}
			return true
		}
	}
	return false
}

func (m *Manager) AppendTurn(id, input, response string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	td := m.loadTopicData(id)

	turn := Turn{
		Input:    input,
		Response: response,
		Time:     time.Now().Format(time.RFC3339),
	}
	td.Turns = append(td.Turns, turn)
	td.UpdatedAt = turn.Time

	for i := range m.list.Topics {
		if m.list.Topics[i].ID == id {
			if strings.HasPrefix(m.list.Topics[i].Title, "Topic ") && len(td.Turns) == 1 {
				m.list.Topics[i].Title = truncate(input, 60)
				td.Title = m.list.Topics[i].Title
			}
			m.list.Topics[i].UpdatedAt = turn.Time
			break
		}
	}

	m.saveTopics()
	m.writeTopicJSON(td)
	m.writeTopicMD(td)

	return nil
}

func (m *Manager) UpdateTitle(id, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.list.Topics {
		if m.list.Topics[i].ID == id {
			m.list.Topics[i].Title = title
			m.list.Topics[i].UpdatedAt = time.Now().Format(time.RFC3339)
			m.saveTopics()
			td := m.loadTopicData(id)
			td.Title = title
			td.UpdatedAt = m.list.Topics[i].UpdatedAt
			m.writeTopicJSON(td)
			m.writeTopicMD(td)
			return
		}
	}
}

func (m *Manager) LoadTopicTurns(id string) []Turn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readTopicTurns(id)
}

func (m *Manager) readTopicTurns(id string) []Turn {
	td := m.loadTopicData(id)
	return td.Turns
}

func (m *Manager) loadTopicData(id string) *TopicData {
	data, err := os.ReadFile(m.topicJSONPath(id))
	if err != nil {
		return &TopicData{Topic: Topic{ID: id, Title: "Unknown"}}
	}
	var td TopicData
	if json.Unmarshal(data, &td) != nil {
		return &TopicData{Topic: Topic{ID: id, Title: "Unknown"}}
	}
	return &td
}

func (m *Manager) writeTopicJSON(td *TopicData) error {
	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.topicJSONPath(td.ID), data, 0644)
}

func (m *Manager) writeTopicMD(td *TopicData) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Topic: %s\n", td.Title))
	sb.WriteString(fmt.Sprintf("Created: %s\n", td.CreatedAt))
	sb.WriteString(fmt.Sprintf("Updated: %s\n\n", td.UpdatedAt))

	for _, t := range td.Turns {
		sb.WriteString(fmt.Sprintf("## User: %s\n", t.Input))
		sb.WriteString(fmt.Sprintf("## DHI: %s\n\n", t.Response))
		sb.WriteString("---\n\n")
	}

	return os.WriteFile(m.topicMDPath(td.ID), []byte(sb.String()), 0644)
}

func (m *Manager) FormatForPrompt() string {
	td := m.loadTopicData(m.list.Current)

	if len(td.Turns) == 0 {
		return "No prior conversation."
	}

	var sb strings.Builder
	start := 0
	if len(td.Turns) > 5 {
		start = len(td.Turns) - 5
	}
	for i := start; i < len(td.Turns); i++ {
		t := td.Turns[i]
		sb.WriteString(fmt.Sprintf("[USER] %s\n[ASSISTANT] %s\n\n", t.Input, t.Response))
	}

	return sb.String()
}

func (m *Manager) ListTopics() []Topic {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]Topic, len(m.list.Topics))
	copy(result, m.list.Topics)
	return result
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("topic_%x", b)
}

func ExtractSearchTerms(params json.RawMessage) []string {
	if params == nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(params, &raw) != nil {
		return nil
	}
	termsRaw, ok := raw["search_terms"]
	if !ok {
		return nil
	}
	var terms []string
	if json.Unmarshal(termsRaw, &terms) != nil {
		return nil
	}
	return terms
}

func SearchPastData(terms []string) (string, error) {
	if len(terms) == 0 {
		return "", nil
	}
	pattern := strings.Join(terms, "|")
	args := append([]string{"-i", "-l", pattern, ".data"}, "-r")

	cmd := exec.Command("rg", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}

	var results strings.Builder
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	limit := 5
	for _, f := range files {
		if f == "" || limit <= 0 {
			continue
		}
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		results.WriteString("\n--- " + f + ":\n")
		lines := strings.Split(string(content), "\n")
		maxLines := 20
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		for _, l := range lines {
			if strings.Contains(strings.ToLower(l), strings.ToLower(terms[0])) {
				results.WriteString("  > " + strings.TrimSpace(l) + "\n")
			}
		}
		limit--
	}
	return results.String(), nil
}
