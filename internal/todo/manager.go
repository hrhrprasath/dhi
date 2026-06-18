package todo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Item struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

type List struct {
	Items []Item `json:"items"`
}

type Manager struct {
	path string
}

func NewManager(dataDir string) *Manager {
	dir := filepath.Join(dataDir, "todos")
	_ = os.MkdirAll(dir, 0755)
	return &Manager{
		path: filepath.Join(dir, "todos.json"),
	}
}

func (m *Manager) load() List {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return List{}
	}
	var l List
	_ = json.Unmarshal(data, &l)
	return l
}

func (m *Manager) save(l *List) {
	data, _ := json.MarshalIndent(l, "", "  ")
	_ = os.WriteFile(m.path, data, 0644)
}

func (m *Manager) Add(text string) string {
	l := m.load()
	id := time.Now().Format("150405") + "-" + strings.ReplaceAll(strings.ToLower(text[:min(len(text), 10)]), " ", "_")
	l.Items = append(l.Items, Item{
		ID:        id,
		Text:      text,
		Done:      false,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
	m.save(&l)
	return id
}

func (m *Manager) Complete(id string) bool {
	l := m.load()
	for i := range l.Items {
		if l.Items[i].ID == id || strings.HasPrefix(l.Items[i].ID, id) {
			l.Items[i].Done = true
			m.save(&l)
			return true
		}
	}
	return false
}

func (m *Manager) Uncomplete(id string) bool {
	l := m.load()
	for i := range l.Items {
		if l.Items[i].ID == id || strings.HasPrefix(l.Items[i].ID, id) {
			l.Items[i].Done = false
			m.save(&l)
			return true
		}
	}
	return false
}

func (m *Manager) List(includeDone bool) string {
	l := m.load()
	if len(l.Items) == 0 {
		return "No todos."
	}
	var sb strings.Builder
	for _, item := range l.Items {
		if !includeDone && item.Done {
			continue
		}
		status := "☐"
		if item.Done {
			status = "☑"
		}
		sb.WriteString(status + " " + item.Text + "\n")
	}
	if sb.Len() == 0 {
		return "No pending todos."
	}
	return sb.String()
}

func (m *Manager) ListItems() []Item {
	l := m.load()
	out := make([]Item, len(l.Items))
	copy(out, l.Items)
	return out
}

func (m *Manager) Delete(id string) bool {
	l := m.load()
	for i := range l.Items {
		if l.Items[i].ID == id {
			l.Items = append(l.Items[:i], l.Items[i+1:]...)
			m.save(&l)
			return true
		}
	}
	return false
}

func (m *Manager) ActiveCount() int {
	l := m.load()
	n := 0
	for _, item := range l.Items {
		if !item.Done {
			n++
		}
	}
	return n
}

func (m *Manager) FormatForPrompt() string {
	return m.List(false)
}
