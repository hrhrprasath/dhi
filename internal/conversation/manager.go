package conversation

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Turn struct {
	Input    string `json:"input"`
	Response string `json:"response"`
	Time     string `json:"time"`
}

type Summary struct {
	Turns      []Turn `json:"turns"`
	Overall    string `json:"overall"`
	RecentNote string `json:"recent_note"`
	TurnCount  int    `json:"turn_count"`
	LastUpdate string `json:"last_updated"`
}

func NewSummary() Summary {
	return Summary{
		LastUpdate: time.Now().Format(time.RFC3339),
	}
}

type Manager struct {
	summaryPath string
}

func NewManager(dataDir string) *Manager {
	dir := filepath.Join(dataDir, "conversation")
	os.MkdirAll(dir, 0755)
	return &Manager{
		summaryPath: filepath.Join(dir, "summary.json"),
	}
}

func (m *Manager) LoadSummary() Summary {
	data, err := os.ReadFile(m.summaryPath)
	if err != nil {
		return NewSummary()
	}
	var s Summary
	if json.Unmarshal(data, &s) != nil {
		return NewSummary()
	}
	return s
}

func (m *Manager) SaveSummary(s *Summary) {
	s.LastUpdate = time.Now().Format(time.RFC3339)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(m.summaryPath, data, 0644)
}

func (m *Manager) FormatForPrompt(s *Summary) string {
	var sb strings.Builder

	if s.Overall != "" {
		sb.WriteString("Conversation summary: " + s.Overall + "\n")
	}

	if len(s.Turns) > 0 {
		sb.WriteString("\nRecent exchanges:\n")
		start := 0
		if len(s.Turns) > 5 {
			start = len(s.Turns) - 5
		}
		for i := start; i < len(s.Turns); i++ {
			t := s.Turns[i]
			sb.WriteString(fmt.Sprintf("[USER] %s\n[ASSISTANT] %s\n\n", t.Input, t.Response))
		}
	}

	if sb.Len() == 0 {
		sb.WriteString("No prior conversation.")
	}

	return sb.String()
}

func (m *Manager) UpdateSummary(userInput, response string) {
	s := m.LoadSummary()
	s.TurnCount++

	entry := "User: " + userInput + "\nDHI: " + response

	if s.Overall == "" {
		s.Overall = entry
	} else {
		s.Overall = s.Overall + "\n---\n" + entry
		if len(s.Overall) > 600 {
			s.Overall = "..." + s.Overall[len(s.Overall)-597:]
		}
	}

	s.Turns = append(s.Turns, Turn{
		Input:    userInput,
		Response: response,
		Time:     time.Now().Format(time.RFC3339),
	})
	if len(s.Turns) > 50 {
		s.Turns = s.Turns[len(s.Turns)-50:]
	}

	m.SaveSummary(&s)
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
