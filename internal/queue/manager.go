package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Intent struct {
	Pipeline string          `json:"pipeline"`
	Action   string          `json:"action"`
	Params   json.RawMessage `json:"params,omitempty"`
}

type InputItem struct {
	ID                 string    `json:"id"`
	RawInput           string    `json:"raw_input"`
	Intent             *Intent   `json:"intent,omitempty"`
	Status             Status    `json:"status"`
	RetryCount         int       `json:"retry_count"`
	CreatedAt          time.Time `json:"created_at"`
	ProcessingStartedAt *time.Time `json:"processing_started_at,omitempty"`
}

func NewInputItem(raw string) InputItem {
	return InputItem{
		ID:        uuid.New().String(),
		RawInput:  raw,
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
}

type OutputItem struct {
	ID        string    `json:"id"`
	Pipeline  string    `json:"pipeline"`
	InputID   string    `json:"input_id"`
	RawData   string    `json:"raw_data"`
	Summary   string    `json:"summary"`
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

func NewOutputItem(pipeline, inputID, rawData, summary string) OutputItem {
	return OutputItem{
		ID:        uuid.New().String(),
		Pipeline:  pipeline,
		InputID:   inputID,
		RawData:   rawData,
		Summary:   summary,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
	}
}

type Manager struct {
	mu              sync.Mutex
	dataDir         string
	inputPath       string
	outputPath      string
	inputQueue      []InputItem
}

func NewManager(dataDir string) *Manager {
	d := dataDir
	if d == "" {
		d = ".data"
	}
	queueDir := filepath.Join(d, "queue")
	os.MkdirAll(queueDir, 0755)

	m := &Manager{
		dataDir:    d,
		inputPath:  filepath.Join(queueDir, "input.json"),
		outputPath: filepath.Join(queueDir, "output.json"),
	}
	m.inputQueue = loadJSON[[]InputItem](m.inputPath)
	if m.inputQueue == nil {
		m.inputQueue = make([]InputItem, 0)
	}
	return m
}

func loadJSON[T any](path string) T {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero
	}
	json.Unmarshal(data, &zero)
	return zero
}

func (m *Manager) saveJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	return os.WriteFile(path, data, 0644)
}

func (m *Manager) EnqueueInput(item InputItem) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := item.ID
	m.inputQueue = append(m.inputQueue, item)
	m.trimAndSaveInput()
	return id
}

func (m *Manager) DequeueNextPending() *InputItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.inputQueue {
		item := &m.inputQueue[i]
		if (item.Status == StatusPending || item.Status == StatusFailed) && item.RetryCount < 3 {
			item.Status = StatusProcessing
			now := time.Now()
			item.ProcessingStartedAt = &now
			clone := item.Clone()
			m.saveInputQueue()
			return &clone
		}
	}
	return nil
}

func (m *Manager) UpdateItem(item *InputItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.inputQueue {
		if m.inputQueue[i].ID == item.ID {
			m.inputQueue[i] = *item
			m.saveInputQueue()
			return
		}
	}
}

func (m *Manager) RecoverStaleItems() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	recovered := 0
	now := time.Now()
	for i := range m.inputQueue {
		item := &m.inputQueue[i]
		if item.Status == StatusProcessing && item.ProcessingStartedAt != nil {
			if now.Sub(*item.ProcessingStartedAt) > 60*time.Second {
				item.Status = StatusPending
				item.RetryCount++
				item.ProcessingStartedAt = nil
				recovered++
			}
		}
	}
	if recovered > 0 {
		m.saveInputQueue()
	}
	return recovered
}

func (m *Manager) saveInputQueue() {
	m.trimAndSaveInput()
}

func (m *Manager) trimAndSaveInput() {
	if len(m.inputQueue) > 100 {
		m.inputQueue = m.inputQueue[len(m.inputQueue)-100:]
	}
	m.saveJSON(m.inputPath, m.inputQueue)
}

func (m *Manager) EnqueueOutput(item OutputItem) error {
	queue := loadJSON[[]OutputItem](m.outputPath)
	if queue == nil {
		queue = make([]OutputItem, 0)
	}
	queue = append(queue, item)
	if err := m.saveJSON(m.outputPath, queue); err != nil {
		return err
	}
	m.persistAsMarkdown(&item)
	return nil
}

func (m *Manager) persistAsMarkdown(item *OutputItem) {
	pipelineDir := filepath.Join(m.dataDir, item.Pipeline)
	os.MkdirAll(pipelineDir, 0755)
	filename := item.Timestamp.Format("2006-01-02T15-04-05") + ".md"
	content := "# " + item.Pipeline + " - " + item.ID + "\n\n"
	content += "**Pipeline:** " + item.Pipeline + "\n"
	content += "**Time:** " + item.Timestamp.Format(time.RFC3339) + "\n\n"
	content += "## Summary\n" + item.Summary + "\n\n"
	content += "## Raw Data\n```\n" + item.RawData + "\n```\n"
	os.WriteFile(filepath.Join(pipelineDir, filename), []byte(content), 0644)
}

func (m *Manager) RecentOutputs(limit int) []OutputItem {
	queue := loadJSON[[]OutputItem](m.outputPath)
	if queue == nil {
		return nil
	}
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].Timestamp.After(queue[j].Timestamp)
	})
	if len(queue) > limit {
		queue = queue[:limit]
	}
	return queue
}

func (m *Manager) InputLen() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.inputQueue)
}

func (m *Manager) OutputLen() int {
	queue := loadJSON[[]OutputItem](m.outputPath)
	return len(queue)
}

func (i *InputItem) Clone() InputItem {
	c := *i
	if i.Intent != nil {
		intentCopy := *i.Intent
		c.Intent = &intentCopy
	}
	if i.ProcessingStartedAt != nil {
		t := *i.ProcessingStartedAt
		c.ProcessingStartedAt = &t
	}
	return c
}
