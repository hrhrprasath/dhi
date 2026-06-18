package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeOnce      Type = "once"
	TypeScheduled Type = "scheduled"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusWaiting    Status = "waiting"
)

type Task struct {
	ID           string     `json:"id"`
	Type         Type       `json:"type"`
	RawInput     string     `json:"raw_input"`
	Status       Status     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	IntervalSec  int        `json:"interval_sec,omitempty"`
	RetryCount   int        `json:"retry_count"`
	Result       string     `json:"result,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type ScheduleEntry struct {
	When    string `json:"when"`
	Command string `json:"command"`
}

type Manager struct {
	mu         sync.Mutex
	tasks      []Task
	path       string
	parser     TimeParser
	looperStop chan struct{}
}

func NewManager(dataDir string, parser TimeParser) *Manager {
	if parser == nil {
		parser = &DefaultParser{}
	}
	dir := filepath.Join(dataDir, "tasks")
	_ = os.MkdirAll(dir, 0755)
	m := &Manager{
		path:       filepath.Join(dir, "tasks.json"),
		parser:     parser,
		looperStop: make(chan struct{}),
	}
	m.tasks = m.load()
	return m
}

func (m *Manager) AddOnce(rawInput string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := Task{
		ID:        uuid.New().String(),
		Type:      TypeOnce,
		RawInput:  rawInput,
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
	m.tasks = append(m.tasks, t)
	m.save()
	return &t
}

func (m *Manager) AddScheduled(rawInput string, scheduledFor time.Time, intervalSec int) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := Task{
		ID:           uuid.New().String(),
		Type:         TypeScheduled,
		RawInput:     rawInput,
		Status:       StatusWaiting,
		CreatedAt:    time.Now(),
		ScheduledFor: &scheduledFor,
		IntervalSec:  intervalSec,
	}
	m.tasks = append(m.tasks, t)
	m.save()
	return &t
}

func (m *Manager) AddScheduledFromEntries(entries []ScheduleEntry) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	var created []*Task
	for _, e := range entries {
		sched, err := m.parser.Parse(e.When)
		if err != nil {
			continue
		}
		t := Task{
			ID:           uuid.New().String(),
			Type:         TypeScheduled,
			RawInput:     e.Command,
			Status:       StatusWaiting,
			CreatedAt:    time.Now(),
			ScheduledFor: &sched.ScheduledFor,
			IntervalSec:  sched.IntervalSec,
		}
		m.tasks = append(m.tasks, t)
		created = append(created, &t)
	}
	if len(created) > 0 {
		m.save()
	}
	return created
}

func (m *Manager) NextDue() *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for i := range m.tasks {
		t := &m.tasks[i]
		if t.Status == StatusPending {
			t.Status = StatusProcessing
			m.save()
			clone := *t
			return &clone
		}
	}
	for i := range m.tasks {
		t := &m.tasks[i]
		if t.Status == StatusWaiting && t.ScheduledFor != nil && !t.ScheduledFor.After(now) {
			t.Status = StatusProcessing
			m.save()
			clone := *t
			return &clone
		}
	}
	return nil
}

func (m *Manager) Update(id string, fn func(t *Task)) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tasks {
		if m.tasks[i].ID == id {
			fn(&m.tasks[i])
			m.save()
			clone := m.tasks[i]
			return &clone
		}
	}
	return nil
}

func (m *Manager) Parser() TimeParser {
	return m.parser
}

func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tasks {
		if m.tasks[i].ID == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			m.save()
			return true
		}
	}
	return false
}

func (m *Manager) DeleteByPrefix(prefix string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.tasks {
		if strings.HasPrefix(m.tasks[i].ID, prefix) {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			m.save()
			return true
		}
	}
	return false
}

func (m *Manager) ListByStatus(status ...Status) []Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(status) == 0 {
		out := make([]Task, len(m.tasks))
		copy(out, m.tasks)
		return out
	}

	statusSet := make(map[Status]bool)
	for _, s := range status {
		statusSet[s] = true
	}
	var out []Task
	for _, t := range m.tasks {
		if statusSet[t.Status] {
			out = append(out, t)
		}
	}
	return out
}

func (m *Manager) CountByStatus(status Status) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	n := 0
	for _, t := range m.tasks {
		if t.Status == status {
			n++
		}
	}
	return n
}

func (m *Manager) CheckSchedule() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	activated := 0
	for i := range m.tasks {
		t := &m.tasks[i]
		if t.Status == StatusWaiting && t.ScheduledFor != nil && !t.ScheduledFor.After(now) {
			t.Status = StatusPending
			activated++
		}
	}
	if activated > 0 {
		m.save()
	}
	return activated
}

func (m *Manager) StartLooper(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.CheckSchedule()
			case <-m.looperStop:
				return
			}
		}
	}()
}

func (m *Manager) StopLooper() {
	close(m.looperStop)
}

func (m *Manager) load() []Task {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil
	}
	return tasks
}

func (m *Manager) save() {
	data, err := json.MarshalIndent(m.tasks, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.path, data, 0644)
}
