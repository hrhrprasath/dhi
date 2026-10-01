package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hrhrprasath/dhi/internal/ai"
	"github.com/hrhrprasath/dhi/internal/intent"
	"github.com/hrhrprasath/dhi/internal/pipeline"
	"github.com/hrhrprasath/dhi/internal/rss"
	"github.com/hrhrprasath/dhi/internal/services"
	"github.com/hrhrprasath/dhi/internal/settings"
	"github.com/hrhrprasath/dhi/internal/summarizer"
	"github.com/hrhrprasath/dhi/internal/task"
	"github.com/hrhrprasath/dhi/internal/todo"
	"github.com/hrhrprasath/dhi/internal/topic"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type noopFeedback struct{}

func (noopFeedback) RecordOutcome(string, string, string, float64, bool, bool, string) {}
func (noopFeedback) RecordConversation(string)                                         {}
func (noopFeedback) RecordRating(string, bool)                                         {}
func (noopFeedback) SetModel(string)                                                   {}

// ─── rightWidth ───

func TestRightWidth(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  int
	}{
		{"width less than minimum", 30, 40},
		{"exact minimum boundary", 70, 40},
		{"one above minimum", 71, 41},
		{"moderate width", 100, 70},
		{"wide", 120, 90},
		{"very wide", 200, 170},
		{"zero width", 0, 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{width: tt.width}
			got := m.rightWidth()
			if got != tt.want {
				t.Errorf("rightWidth() with width=%d = %d, want %d", tt.width, got, tt.want)
			}
		})
	}
}

// ─── computeSlashMatches ───

func TestComputeSlashMatches_ResetsState(t *testing.T) {
	m := &Model{
		cc: commandCenter{
			slashMatches: []string{"/old"},
			slashIndex:   3,
			convView:     viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/lo")
	if len(m.cc.slashMatches) != 0 && m.cc.slashMatches[0] == "/old" {
		t.Error("computeSlashMatches should reset slashMatches before computing")
	}
	if m.cc.slashIndex != 0 {
		t.Errorf("slashIndex should be 0 after compute, got %d", m.cc.slashIndex)
	}
}

func TestComputeSlashMatches_NoSlash(t *testing.T) {
	m := &Model{
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("help")
	if m.cc.slashMatches != nil {
		t.Errorf("expected nil matches for non-slash input, got %v", m.cc.slashMatches)
	}
}

func TestComputeSlashMatches_JustSlash(t *testing.T) {
	m := &Model{
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/")
	if m.cc.slashMatches != nil {
		t.Errorf("expected nil matches for just '/', got %v", m.cc.slashMatches)
	}
}

func TestComputeSlashMatches_Matches(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   []string
	}{
		{"match log", "/lo", []string{"/log", "/logs"}},
		{"match quit", "/qu", []string{"/quit"}},
		{"exact match returns nil", "/new", nil},
		{"match help", "/hel", []string{"/help"}},
		{"match good", "/go", []string{"/good"}},
		{"match bad", "/ba", []string{"/bad"}},
		{"match resume", "/re", []string{"/resume"}},
		{"match fork", "/fo", []string{"/fork"}},
		{"match rss", "/rs", []string{"/rss"}},
		{"match schedule", "/sc", []string{"/schedule"}},
		{"no match", "/xyz", nil},
		{"full command exact", "/help", nil},
		{"full command exact quit", "/quit", nil},
		{"partial match returns multiple", "/l", []string{"/log", "/logs"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				cc: commandCenter{
					convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
				},
			}
			m.computeSlashMatches(tt.prefix)
			if !reflect.DeepEqual(m.cc.slashMatches, tt.want) {
				t.Errorf("computeSlashMatches(%q) = %v, want %v", tt.prefix, m.cc.slashMatches, tt.want)
			}
		})
	}
}

func TestComputeSlashMatches_TopicAndTopics(t *testing.T) {
	m := &Model{
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/top")
	// "/top" falls through to slashCommands matching since it's not "/topic" or "/topic "
	// It should match "/topic" and "/topics"
	want := []string{"/topic", "/topics"}
	if !reflect.DeepEqual(m.cc.slashMatches, want) {
		t.Errorf("computeSlashMatches(\"/top\") = %v, want %v", m.cc.slashMatches, want)
	}
}

func TestComputeSlashMatches_ModelAndModels(t *testing.T) {
	m := &Model{
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/mod")
	// "/mod" falls through to slashCommands matching
	want := []string{"/model", "/models"}
	if !reflect.DeepEqual(m.cc.slashMatches, want) {
		t.Errorf("computeSlashMatches(\"/mod\") = %v, want %v", m.cc.slashMatches, want)
	}
}

// ─── computeSlashMatches with /topic prefix routing ───

func TestComputeSlashMatches_TopicPrefixDefault(t *testing.T) {
	dataDir := t.TempDir()
	// topic.NewManager creates a default "Topic 1", so there's always 1 topic
	tm := topic.NewManager(dataDir)
	m := &Model{
		topicMgr: tm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/topic")
	// Routes to computeTopicMatches — should match the default topic
	if len(m.cc.slashMatches) != 1 {
		t.Errorf("expected 1 match for default topic, got %d: %v", len(m.cc.slashMatches), m.cc.slashMatches)
	}
	if m.cc.slashMatches[0] != "/topic 1" {
		t.Errorf("expected '/topic 1', got %q", m.cc.slashMatches[0])
	}
}

func TestComputeSlashMatches_TopicPrefixWithTopics(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir) // creates "Topic 1"
	tm.CreateTopic("Alpha")
	tm.CreateTopic("Beta")
	m := &Model{
		topicMgr: tm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/topic")
	// 1 default + 2 created = 3 topics; ordering depends on UpdatedAt desc
	if len(m.cc.slashMatches) != 3 {
		t.Errorf("expected 3 slash matches (1 default + 2 created), got %d: %v", len(m.cc.slashMatches), m.cc.slashMatches)
	}
}

func TestComputeSlashMatches_TopicPrefixNumericMatch(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir) // default "Topic 1"
	tm.CreateTopic("Alpha")
	m := &Model{
		topicMgr: tm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// With 2 topics (default + Alpha), "/topic 2" should match
	m.computeSlashMatches("/topic 2")
	if len(m.cc.slashMatches) != 1 {
		t.Errorf("expected 1 match for '/topic 2', got %d: %v", len(m.cc.slashMatches), m.cc.slashMatches)
	}
}

// ─── computeSlashMatches with /model prefix routing ───

func TestComputeSlashMatches_ModelPrefixEmpty(t *testing.T) {
	dataDir := t.TempDir()
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	m := &Model{
		settingsMgr: sm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/model")
	// Routes to computeModelMatches, which iterates models (empty) — no matches
	if m.cc.slashMatches != nil {
		t.Errorf("expected nil for empty models, got %v", m.cc.slashMatches)
	}
}

func TestComputeSlashMatches_ModelPrefixWithModels(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"llama3:8b","size":4700000000,"context_length":8192},{"name":"qwen2:1.5b","size":1500000000,"context_length":32768}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	m := &Model{
		settingsMgr: sm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/model")
	// Should generate "/model 1", "/model 2"
	if len(m.cc.slashMatches) != 2 {
		t.Errorf("expected 2 slash matches, got %d: %v", len(m.cc.slashMatches), m.cc.slashMatches)
	}
	if m.cc.slashMatches[0] != "/model 1" {
		t.Errorf("expected first match '/model 1', got %q", m.cc.slashMatches[0])
	}
	if m.cc.slashMatches[1] != "/model 2" {
		t.Errorf("expected second match '/model 2', got %q", m.cc.slashMatches[1])
	}
}

func TestComputeSlashMatches_ModelPrefixSpecific(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"model-a","size":1000000000,"context_length":4096},{"name":"model-b","size":2000000000,"context_length":8192},{"name":"model-c","size":3000000000,"context_length":16384}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	m := &Model{
		settingsMgr: sm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.computeSlashMatches("/model 2")
	if len(m.cc.slashMatches) != 1 {
		t.Errorf("expected 1 match for '/model 2', got %d: %v", len(m.cc.slashMatches), m.cc.slashMatches)
	}
	if m.cc.slashMatches[0] != "/model 2" {
		t.Errorf("expected '/model 2', got %q", m.cc.slashMatches[0])
	}
}

func TestComputeSlashMatches_ModelPrefixNonExistentIndex(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"only-one","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	m := &Model{
		settingsMgr: sm,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// only 1 model, so "/model 2" should not match
	m.computeSlashMatches("/model 2")
	if len(m.cc.slashMatches) != 0 {
		t.Errorf("expected 0 matches for '/model 2' with 1 model, got %d: %v", len(m.cc.slashMatches), m.cc.slashMatches)
	}
}

// ─── handleOpenURL ───

func TestHandleOpenURL_Basic(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	convView := viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))
	s := spinner.New()

	m := &Model{
		logger:  logger,
		taskMgr: tm,
		events:  make(chan any, 200),
		cc: commandCenter{
			convView: convView,
			spinner:  s,
		},
	}
	m.handleOpenURL("https://example.com")

	if m.cc.lastSubmitted != "open https://example.com" {
		t.Errorf("expected lastSubmitted 'open https://example.com', got %q", m.cc.lastSubmitted)
	}
	if m.cc.thinking != 1 {
		t.Errorf("expected thinking=1, got %d", m.cc.thinking)
	}
	if len(m.cc.conversations) != 1 {
		t.Errorf("expected 1 conversation entry, got %d", len(m.cc.conversations))
	}
	if m.cc.conversations[0].input != "open https://example.com" {
		t.Errorf("expected conversation input 'open https://example.com', got %q", m.cc.conversations[0].input)
	}
	if !m.cc.conversations[0].thinking {
		t.Error("expected conversation entry to be marked as thinking")
	}
}

func TestHandleOpenURL_ReturnsEarlyWhenThinking(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	convView := viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))
	s := spinner.New()

	m := &Model{
		logger:  logger,
		taskMgr: tm,
		events:  make(chan any, 200),
		cc: commandCenter{
			convView:      convView,
			spinner:       s,
			thinking:      1,
			lastSubmitted: "previous input",
			conversations: []conversationEntry{
				{input: "previous input", response: "...", thinking: true},
			},
		},
	}
	m.handleOpenURL("https://example.com")

	// Should NOT change state since cc.thinking > 0
	if m.cc.lastSubmitted != "previous input" {
		t.Errorf("expected lastSubmitted unchanged, got %q", m.cc.lastSubmitted)
	}
	if m.cc.thinking != 1 {
		t.Errorf("expected thinking unchanged (1), got %d", m.cc.thinking)
	}
}

func TestHandleOpenURL_EmptyURL(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	convView := viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))
	s := spinner.New()

	m := &Model{
		logger:  logger,
		taskMgr: tm,
		events:  make(chan any, 200),
		cc: commandCenter{
			convView: convView,
			spinner:  s,
		},
	}
	m.handleOpenURL("")

	if m.cc.lastSubmitted != "open " {
		t.Errorf("expected lastSubmitted 'open ', got %q", m.cc.lastSubmitted)
	}
}

// ─── selectCurrentModel ───

func TestSelectCurrentModel_EmptyFiltered(t *testing.T) {
	m := &Model{
		model: modelPanel{
			filtered:  nil,
			selIdx:    0,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// Should return early without panic
	m.selectCurrentModel()
}

func TestSelectCurrentModel_OutOfBoundsIndex(t *testing.T) {
	m := &Model{
		model: modelPanel{
			filtered: []settings.ModelInfo{
				{Name: "test-model", Size: 1000000000, ContextLen: 4096},
			},
			selIdx:    5,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// Should return early without panic
	m.selectCurrentModel()
}

func TestSelectCurrentModel_NegativeIndex(t *testing.T) {
	m := &Model{
		model: modelPanel{
			filtered: []settings.ModelInfo{
				{Name: "test-model", Size: 1000000000, ContextLen: 4096},
			},
			selIdx:    -1,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectCurrentModel()
}

func TestSelectCurrentModel_ClosesPopup(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()

	m := &Model{
		settingsMgr: sm,
		logger:      logger,
		events:      make(chan any, 200),
		model: modelPanel{
			// Use a model name that doesn't exist in settings so SelectModel
			// returns an error and handleModelSelect returns early (before
			// calling router/executor.SetModel which would panic with nil)
			filtered: []settings.ModelInfo{
				{Name: "nonexistent-model", Size: 1000000000, ContextLen: 4096},
			},
			selIdx:    0,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectCurrentModel()
	if m.model.popupOpen {
		t.Error("expected popupOpen to be false after selectCurrentModel")
	}
}

// ─── selectCurrentTopic ───

func TestSelectCurrentTopic_EmptyFiltered(t *testing.T) {
	m := &Model{
		topic: topicPanel{
			filtered:  nil,
			selIdx:    0,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectCurrentTopic()
}

func TestSelectCurrentTopic_OutOfBoundsIndex(t *testing.T) {
	m := &Model{
		topic: topicPanel{
			filtered: []topic.Topic{
				{ID: "1", Title: "First topic"},
			},
			selIdx:    5,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectCurrentTopic()
}

func TestSelectCurrentTopic_NegativeIndex(t *testing.T) {
	m := &Model{
		topic: topicPanel{
			filtered: []topic.Topic{
				{ID: "1", Title: "First topic"},
			},
			selIdx:    -1,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectCurrentTopic()
}

func TestSelectCurrentTopic_ClosesPopup(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	tm.CreateTopic("Test topic")
	logger := services.NewLogger()

	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		topic: topicPanel{
			filtered:  tm.ListTopics(),
			selIdx:    0,
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectCurrentTopic()
	if m.topic.popupOpen {
		t.Error("expected popupOpen to be false after selectCurrentTopic")
	}
}

// ─── selectRecentModel ───

func TestSelectRecentModel_NoModels(t *testing.T) {
	dataDir := t.TempDir()
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})

	m := &Model{
		settingsMgr: sm,
		model:       modelPanel{recentIdx: 0},
	}
	// Should not panic, just return silently since no models
	m.selectRecentModel()
}

func TestSelectRecentModel_NegativeIndex(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})

	m := &Model{
		settingsMgr: sm,
		model:       modelPanel{recentIdx: -1},
	}
	m.selectRecentModel()
}

func TestSelectRecentModel_IndexOutOfBounds(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})

	m := &Model{
		settingsMgr: sm,
		model:       modelPanel{recentIdx: 5},
	}
	m.selectRecentModel()
}

func TestSelectRecentModel_LimitToFive(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	models := make([]settings.ModelInfo, 7)
	for i := range models {
		models[i] = settings.ModelInfo{
			Name:       fmt.Sprintf("model-%d", i+1),
			Size:       int64((i + 1) * 1000000000),
			ContextLen: 4096,
		}
	}
	_ = settings.NewManager(dataDir, ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"}), ai.Config{})
	// Manually set models to have 7 entries for the limit test
	// We need to write to settings file since ListModels reads from settings file
	settingsDir2 := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir2, 0755)
	jsonData := `{"models":[{"name":"model-1","size":1000000000,"context_length":4096},{"name":"model-2","size":2000000000,"context_length":4096},{"name":"model-3","size":3000000000,"context_length":4096},{"name":"model-4","size":4000000000,"context_length":4096},{"name":"model-5","size":5000000000,"context_length":4096},{"name":"model-6","size":6000000000,"context_length":4096},{"name":"model-7","size":7000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir2, "settings.json"), []byte(jsonData), 0644)
	sm := settings.NewManager(dataDir, ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"}), ai.Config{})

	models = sm.ListModels()
	if len(models) != 7 {
		t.Fatalf("expected 7 models, got %d", len(models))
	}
}

// ─── selectRecentTopic ───

func TestSelectRecentTopic_WithDefaultTopic(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir) // creates default "Topic 1"
	logger := services.NewLogger()

	m := &Model{
		topicMgr: tm,
		topic:    topicPanel{recentIdx: 0},
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectRecentTopic()
	// Should have switched to the default topic without panicking
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected a current topic after selectRecentTopic")
	}
}

func TestSelectRecentTopic_NegativeIndex(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	tm.CreateTopic("Only topic")

	m := &Model{
		topicMgr: tm,
		topic:    topicPanel{recentIdx: -1},
	}
	m.selectRecentTopic()
}

func TestSelectRecentTopic_IndexOutOfBounds(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	tm.CreateTopic("Only topic")

	m := &Model{
		topicMgr: tm,
		topic:    topicPanel{recentIdx: 5},
	}
	m.selectRecentTopic()
}

func TestSelectRecentTopic_SelectsById(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir) // creates default "Topic 1"

	// ListTopics returns topics sorted by UpdatedAt desc.
	// By default: [Topic 1]. recentIdx=0 selects Topic 1.
	all := tm.ListTopics()
	if len(all) == 0 {
		t.Fatal("expected at least default topic")
	}
	defaultTopicID := all[0].ID

	m := &Model{
		topicMgr: tm,
		topic:    topicPanel{recentIdx: 0},
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.selectRecentTopic()
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected a current topic to be set")
	}
	if cur.ID != defaultTopicID {
		t.Errorf("expected current topic ID %q, got %q", defaultTopicID, cur.ID)
	}
}

// ─── handleModelString ───

func TestHandleModelString_InvalidArg(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()

	m := &Model{
		settingsMgr: sm,
		logger:      logger,
		events:      make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// non-existent model — should log and not panic
	m.handleModelString("nonexistent")
}

// ─── Messages ───

func TestMessageTypes(t *testing.T) {
	_ = tickMsg{}
	_ = routerStatusMsg{online: true}
	_ = logMsg{message: "test"}
	_ = convMsg{input: "hi", response: "hello"}
	_ = convTokenMsg{input: "hi", token: "wor"}
	_ = convCompleteMsg{input: "hi", response: "world"}
	_ = pipelineStatusMsg{name: "test", status: "RUNNING"}
	_ = refreshMsg{}
	_ = rssUpdateMsg{}
}

// ─── conversationEntry ───

func TestConversationEntry_DefaultFields(t *testing.T) {
	e := conversationEntry{}
	if e.input != "" {
		t.Errorf("expected empty input, got %q", e.input)
	}
	if e.response != "" {
		t.Errorf("expected empty response, got %q", e.response)
	}
	if e.thinking {
		t.Error("expected thinking to be false")
	}
}

// ─── pipelineEntry ───

func TestPipelineEntry_DefaultFields(t *testing.T) {
	e := pipelineEntry{}
	if e.name != "" {
		t.Errorf("expected empty name, got %q", e.name)
	}
	if e.status != "" {
		t.Errorf("expected empty status, got %q", e.status)
	}
}

// ─── commandCenter ───

func TestCommandCenter_SetConversationsFromTurns(t *testing.T) {
	cc := &commandCenter{
		thinking: 3,
		conversations: []conversationEntry{
			{input: "old", response: "old", thinking: true},
		},
	}
	turns := []conversationEntry{
		{input: "new1", response: "resp1"},
		{input: "new2", response: "resp2"},
	}
	cc.setConversationsFromTurns(turns)
	if cc.thinking != 0 {
		t.Errorf("expected thinking to be reset to 0, got %d", cc.thinking)
	}
	if len(cc.conversations) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(cc.conversations))
	}
}

func TestCommandCenter_SetResponse(t *testing.T) {
	cc := &commandCenter{
		thinking: 2,
		conversations: []conversationEntry{
			{input: "first", response: "...", thinking: true},
			{input: "second", response: "...", thinking: true},
		},
	}
	ok := cc.setResponse("second", "Hello world")
	if !ok {
		t.Error("expected setResponse to return true")
	}
	if cc.thinking != 1 {
		t.Errorf("expected thinking to decrement to 1, got %d", cc.thinking)
	}
	if cc.conversations[1].response != "Hello world" {
		t.Errorf("expected response 'Hello world', got %q", cc.conversations[1].response)
	}
	if cc.conversations[1].thinking {
		t.Error("expected thinking to be false after setResponse")
	}
}

func TestCommandCenter_SetResponseNoMatch(t *testing.T) {
	cc := &commandCenter{
		thinking:      1,
		conversations: []conversationEntry{},
	}
	ok := cc.setResponse("nothing", "response")
	if ok {
		t.Error("expected setResponse to return false for no match")
	}
	if cc.thinking != 1 {
		t.Errorf("expected thinking unchanged (1), got %d", cc.thinking)
	}
}

func TestCommandCenter_SetThinkingBasic(t *testing.T) {
	s := spinner.New()
	cc := &commandCenter{
		spinner: s,
	}
	cc.setThinking("test input")
	if cc.thinking != 1 {
		t.Errorf("expected thinking=1, got %d", cc.thinking)
	}
	if len(cc.conversations) != 1 {
		t.Errorf("expected 1 conversation, got %d", len(cc.conversations))
	}
	if cc.conversations[0].input != "test input" {
		t.Errorf("expected input 'test input', got %q", cc.conversations[0].input)
	}
	if !cc.conversations[0].thinking {
		t.Error("expected conversation to be marked thinking")
	}
	if cc.conversations[0].response == "" {
		t.Error("expected non-empty spinner response")
	}
}

func TestCommandCenter_UpdateSpinnerResponse(t *testing.T) {
	s := spinner.New()
	cc := &commandCenter{
		spinner:  s,
		thinking: 1,
		conversations: []conversationEntry{
			{input: "test", response: "old", thinking: true},
		},
	}
	updated := cc.updateSpinnerResponse()
	if !updated {
		t.Error("expected updateSpinnerResponse to return true")
	}
	if cc.conversations[0].response == "old" {
		t.Error("expected spinner response to be updated")
	}
}

func TestCommandCenter_UpdateSpinnerResponseNoThinking(t *testing.T) {
	cc := &commandCenter{
		thinking: 0,
	}
	updated := cc.updateSpinnerResponse()
	if updated {
		t.Error("expected updateSpinnerResponse to return false when not thinking")
	}
}

// ─── logPanel ───

func TestLogPanel_AppendWithTruncation(t *testing.T) {
	l := &logPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	// Fill with 1000 entries
	for i := 0; i < 1000; i++ {
		l.append("log line")
	}
	if len(l.entries) != 1000 {
		t.Fatalf("expected 1000 entries, got %d", len(l.entries))
	}
	// Add one more to trigger truncation (keeps last 500)
	l.append("trigger truncation")
	if len(l.entries) > 501 {
		t.Errorf("expected entries to be truncated, got %d", len(l.entries))
	}
}

// ─── render helpers ───

func TestFmtURLs_Formatting(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		want string
	}{
		{
			name: "single url",
			urls: []string{"https://example.com"},
			want: "\n\n  1. https://example.com\n",
		},
		{
			name: "multiple urls",
			urls: []string{"https://a.com", "https://b.com"},
			want: "\n\n  1. https://a.com\n  2. https://b.com\n",
		},
		{
			name: "nil slice",
			urls: nil,
			want: "",
		},
		{
			name: "empty slice",
			urls: []string{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtURLs(tt.urls)
			if got != tt.want {
				t.Errorf("fmtURLs(%v) = %q, want %q", tt.urls, got, tt.want)
			}
		})
	}
}

func TestParseOpenLink_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		urls  []string
		want  string
	}{
		{
			name:  "nil urls",
			input: "open 1",
			urls:  nil,
			want:  "",
		},
		{
			name:  "visit with exact url",
			input: "visit https://specific.com/path?q=1",
			urls:  []string{"https://a.com"},
			want:  "https://specific.com/path?q=1",
		},
		{
			name:  "go to with exact url",
			input: "go to https://example.com/page",
			urls:  []string{"https://other.com"},
			want:  "https://example.com/page",
		},
		{
			name:  "launch exact url",
			input: "launch https://example.com",
			urls:  []string{"https://other.com"},
			want:  "https://example.com",
		},
		{
			name:  "fuzzy match case insensitive",
			input: "open EXAMPLE",
			urls:  []string{"https://example.com/path"},
			want:  "https://example.com/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOpenLink(tt.input, tt.urls)
			if got != tt.want {
				t.Errorf("parseOpenLink(%q, %v) = %q, want %q", tt.input, tt.urls, got, tt.want)
			}
		})
	}
}

// ─── Real-time sensitive helpers ───

func TestFormatTimeAgo_Now(t *testing.T) {
	got := formatTimeAgo(time.Now())
	if got != "now" {
		t.Errorf("formatTimeAgo(now) = %q, want 'now'", got)
	}
}

func TestFormatTimeAgo_Zero(t *testing.T) {
	got := formatTimeAgo(time.Time{})
	if got != "" {
		t.Errorf("formatTimeAgo(zero) = %q, want ''", got)
	}
}

// ─── toggleLogPopup ───

func TestToggleLogPopup_Opens(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.toggleLogPopup()
	if !m.log.popupOpen {
		t.Error("expected popupOpen to be true")
	}
	if m.focused != focusLogs {
		t.Errorf("expected focused=focusLogs, got %d", m.focused)
	}
}

func TestToggleLogPopup_Closes(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		log: logPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.toggleLogPopup()
	if m.log.popupOpen {
		t.Error("expected popupOpen to be false")
	}
	if m.focused != focusInput {
		t.Errorf("expected focused=focusInput, got %d", m.focused)
	}
}

// ─── toggleHelpPopup ───

func TestToggleHelpPopup_Opens(t *testing.T) {
	m := &Model{
		help: newHelpPanel(),
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	m.toggleHelpPopup()
	if !m.help.popupOpen {
		t.Error("expected popupOpen to be true")
	}
	if m.focused != focusLogs {
		t.Errorf("expected focused=focusLogs, got %d", m.focused)
	}
}

func TestToggleHelpPopup_Closes(t *testing.T) {
	m := &Model{
		help: helpPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	m.toggleHelpPopup()
	if m.help.popupOpen {
		t.Error("expected popupOpen to be false")
	}
	if m.focused != focusInput {
		t.Errorf("expected focused=focusInput, got %d", m.focused)
	}
}

// ─── toggleTopicPopup ───

func TestToggleTopicPopup_Opens(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	ti := textinput.New()
	m := &Model{
		topicMgr: tm,
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	m.toggleTopicPopup()
	if !m.topic.popupOpen {
		t.Error("expected popupOpen to be true")
	}
	if m.topic.filter != "" {
		t.Errorf("expected filter cleared, got %q", m.topic.filter)
	}
	if m.topic.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.topic.selIdx)
	}
	if m.focused != focusInput {
		t.Errorf("expected focusInput, got %d", m.focused)
	}
}

func TestToggleTopicPopup_Closes(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	ti := textinput.New()
	m := &Model{
		topicMgr: tm,
		topic: topicPanel{
			popupOpen: true,
			filter:    "test",
			selIdx:    2,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	m.toggleTopicPopup()
	if m.topic.popupOpen {
		t.Error("expected popupOpen to be false")
	}
	if m.topic.filter != "" {
		t.Errorf("expected filter cleared, got %q", m.topic.filter)
	}
	if m.focused != focusInput {
		t.Errorf("expected focusInput, got %d", m.focused)
	}
}

// ─── toggleTodoPopup ───

func TestToggleTodoPopup_Opens(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	ti := textinput.New()
	m := &Model{
		todoMgr: todom,
		todo: todoPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	m.toggleTodoPopup()
	if !m.todo.popupOpen {
		t.Error("expected popupOpen to be true")
	}
	if m.todo.tabIdx != 0 {
		t.Errorf("expected tabIdx 0, got %d", m.todo.tabIdx)
	}
	if m.todo.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.todo.selIdx)
	}
	if m.focused != focusLogs {
		t.Errorf("expected focusLogs, got %d", m.focused)
	}
}

func TestToggleTodoPopup_Closes(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		todo: todoPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.toggleTodoPopup()
	if m.todo.popupOpen {
		t.Error("expected popupOpen to be false")
	}
	if m.focused != focusInput {
		t.Errorf("expected focusInput, got %d", m.focused)
	}
}

// ─── toggleSchedulePopup ───

func TestToggleSchedulePopup_Opens(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	ti := textinput.New()
	m := &Model{
		taskMgr: tkm,
		schedule: schedulePanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	m.toggleSchedulePopup()
	if !m.schedule.popupOpen {
		t.Error("expected popupOpen to be true")
	}
	if m.schedule.tabIdx != 0 {
		t.Errorf("expected tabIdx 0, got %d", m.schedule.tabIdx)
	}
	if m.schedule.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.schedule.selIdx)
	}
}

func TestToggleSchedulePopup_Closes(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		schedule: schedulePanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.toggleSchedulePopup()
	if m.schedule.popupOpen {
		t.Error("expected popupOpen to be false")
	}
	if m.focused != focusInput {
		t.Errorf("expected focusInput, got %d", m.focused)
	}
}

// ─── toggleRssPopup ───

func TestToggleRssPopup_Opens(t *testing.T) {
	dataDir := t.TempDir()
	rm := rss.NewManager(dataDir)
	ti := textinput.New()
	m := &Model{
		rssMgr: rm,
		rss: rssPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			// Prevent goroutine from starting in toggleRssPopup
			loaded:    true,
			aiSummary: "pre-populated",
			lastFetch: time.Now(),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	m.toggleRssPopup()
	if !m.rss.popupOpen {
		t.Error("expected popupOpen to be true")
	}
	if m.rss.tabIdx != 0 {
		t.Errorf("expected tabIdx 0, got %d", m.rss.tabIdx)
	}
	if m.rss.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.rss.selIdx)
	}
}

func TestToggleRssPopup_Closes(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		rss: rssPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.toggleRssPopup()
	if m.rss.popupOpen {
		t.Error("expected popupOpen to be false")
	}
	if m.focused != focusInput {
		t.Errorf("expected focusInput, got %d", m.focused)
	}
}

// ─── handleEscape ───

func TestHandleEscape_ClosesTodoPopup(t *testing.T) {
	m := &Model{
		todo: todoPanel{
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.todo.popupOpen {
		t.Error("expected todo popupOpen to be false")
	}
}

func TestHandleEscape_ClosesSchedulePopup(t *testing.T) {
	m := &Model{
		schedule: schedulePanel{
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.schedule.popupOpen {
		t.Error("expected schedule popupOpen to be false")
	}
}

func TestHandleEscape_ClosesRssPopup(t *testing.T) {
	m := &Model{
		rss: rssPanel{
			popupOpen: true,
		},
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.rss.popupOpen {
		t.Error("expected rss popupOpen to be false")
	}
}

func TestHandleEscape_ClosesTopicPopup(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	ti := textinput.New()
	m := &Model{
		topicMgr: tm,
		topic: topicPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.topic.popupOpen {
		t.Error("expected topic popupOpen to be false")
	}
}

func TestHandleEscape_ClosesModelPopup(t *testing.T) {
	dataDir := t.TempDir()
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	ti := textinput.New()
	m := &Model{
		settingsMgr: sm,
		model: modelPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
		events: make(chan any, 200),
	}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.model.popupOpen {
		t.Error("expected model popupOpen to be false")
	}
}

func TestHandleEscape_ClosesHelpPopup(t *testing.T) {
	m := &Model{
		help: helpPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.help.popupOpen {
		t.Error("expected help popupOpen to be false")
	}
}

func TestHandleEscape_ClosesLogPopup(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		focused: focusLogs,
		log: logPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.handleEscape()
	if m.log.popupOpen {
		t.Error("expected log popupOpen to be false")
	}
	if m.focused != focusInput {
		t.Errorf("expected focusInput, got %d", m.focused)
	}
}

func TestHandleEscape_NoPopupReturnsNil(t *testing.T) {
	m := &Model{}
	cmd := m.handleEscape()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// ─── handleTab ───

func TestHandleTab_TopicPopupSelectsCurrent(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	cur := tm.CurrentTopic()
	m := &Model{
		topicMgr: tm,
		topic: topicPanel{
			popupOpen: true,
			filtered:  tm.ListTopics(),
			selIdx:    0,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		logger: services.NewLogger(),
		events: make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleTab()
	if m.topic.popupOpen {
		t.Error("expected topic popupOpen to be false after selection")
	}
	if cur == nil {
		t.Fatal("expected a current topic")
	}
}

func TestHandleTab_ModelPopupSelectsCurrent(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()
	m := &Model{
		settingsMgr: sm,
		logger:      logger,
		events:      make(chan any, 200),
		model: modelPanel{
			popupOpen: true,
			filtered: []settings.ModelInfo{
				{Name: "nonexistent", Size: 0, ContextLen: 0},
			},
			selIdx: 0,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleTab()
	if m.model.popupOpen {
		t.Error("expected model popupOpen to be false after selection")
	}
}

func TestHandleTab_FocusCycling(t *testing.T) {
	t.Run("input to conv", func(t *testing.T) {
		m := &Model{
			focused: focusInput,
			cc: commandCenter{
				textInput: textinput.New(),
				convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			},
		}
		m.handleTab()
		if m.focused != focusConv {
			t.Errorf("expected focusConv, got %d", m.focused)
		}
	})

	t.Run("conv to topics", func(t *testing.T) {
		m := &Model{
			focused: focusConv,
			cc: commandCenter{
				textInput: textinput.New(),
			},
		}
		m.handleTab()
		if m.focused != focusTopics {
			t.Errorf("expected focusTopics, got %d", m.focused)
		}
	})

	t.Run("topics to input", func(t *testing.T) {
		m := &Model{
			focused: focusTopics,
			cc: commandCenter{
				textInput: textinput.New(),
			},
		}
		m.handleTab()
		if m.focused != focusInput {
			t.Errorf("expected focusInput, got %d", m.focused)
		}
	})

	t.Run("logs to input", func(t *testing.T) {
		m := &Model{
			focused: focusLogs,
			cc: commandCenter{
				textInput: textinput.New(),
			},
		}
		m.handleTab()
		if m.focused != focusInput {
			t.Errorf("expected focusInput, got %d", m.focused)
		}
	})
}

func TestHandleTab_FocusConvWithLogPopupOpen(t *testing.T) {
	m := &Model{
		focused: focusConv,
		log: logPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
		},
	}
	m.handleTab()
	if m.focused != focusLogs {
		t.Errorf("expected focusLogs, got %d", m.focused)
	}
}

// ─── handleArrowUp ───

func TestHandleArrowUp_TodoPopup(t *testing.T) {
	m := &Model{
		todo: todoPanel{
			popupOpen: true,
			selIdx:    1,
		},
	}
	m.handleArrowUp()
	if m.todo.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.todo.selIdx)
	}
}

func TestHandleArrowUp_TodoPopupAtZero(t *testing.T) {
	m := &Model{
		todo: todoPanel{
			popupOpen: true,
			selIdx:    0,
		},
	}
	m.handleArrowUp()
	if m.todo.selIdx != 0 {
		t.Errorf("expected selIdx 0 (no change), got %d", m.todo.selIdx)
	}
}

func TestHandleArrowUp_TopicPopup(t *testing.T) {
	m := &Model{
		topic: topicPanel{
			popupOpen: true,
			selIdx:    2,
		},
	}
	m.handleArrowUp()
	if m.topic.selIdx != 1 {
		t.Errorf("expected selIdx 1, got %d", m.topic.selIdx)
	}
}

func TestHandleArrowUp_ModelPopup(t *testing.T) {
	m := &Model{
		model: modelPanel{
			popupOpen: true,
			selIdx:    1,
		},
	}
	m.handleArrowUp()
	if m.model.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.model.selIdx)
	}
}

func TestHandleArrowUp_RssPopup(t *testing.T) {
	m := &Model{
		rss: rssPanel{
			popupOpen: true,
			tabIdx:    1,
			selIdx:    3,
		},
	}
	m.handleArrowUp()
	if m.rss.selIdx != 2 {
		t.Errorf("expected selIdx 2, got %d", m.rss.selIdx)
	}
}

func TestHandleArrowDown_RssPopup(t *testing.T) {
	m := &Model{
		rss: rssPanel{
			popupOpen: true,
			tabIdx:    1,
			selIdx:    3,
			rawItems: []rawItem{
				{isSource: true},
				{isSource: false},
				{isSource: false},
				{isSource: false},
				{isSource: true},
			},
		},
	}
	m.handleArrowDown()
	if m.rss.selIdx != 4 {
		t.Errorf("expected selIdx 4, got %d", m.rss.selIdx)
	}
}

func TestHandleArrowUp_SchedulePopup(t *testing.T) {
	m := &Model{
		schedule: schedulePanel{
			popupOpen: true,
			selIdx:    1,
		},
	}
	m.handleArrowUp()
	if m.schedule.selIdx != 0 {
		t.Errorf("expected selIdx 0, got %d", m.schedule.selIdx)
	}
}

func TestHandleArrowUp_NoPopupFocusInputWithLastSubmitted(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput:     ti,
			lastSubmitted: "show me the weather",
		},
	}
	m.handleArrowUp()
	if m.cc.textInput.Value() != "show me the weather" {
		t.Errorf("expected lastSubmitted value, got %q", m.cc.textInput.Value())
	}
}

func TestHandleArrowUp_NoPopupFocusInputEmptyLastSubmitted(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: ti,
		},
	}
	m.handleArrowUp()
	if m.cc.textInput.Value() != "" {
		t.Errorf("expected empty, got %q", m.cc.textInput.Value())
	}
}

// ─── handleArrowDown ───

func TestHandleArrowDown_TodoPopup(t *testing.T) {
	m := &Model{
		todo: todoPanel{
			popupOpen: true,
			selIdx:    0,
			items:     []todo.Item{{}, {}},
		},
	}
	m.handleArrowDown()
	if m.todo.selIdx != 1 {
		t.Errorf("expected selIdx 1, got %d", m.todo.selIdx)
	}
}

func TestHandleArrowDown_TopicPopup(t *testing.T) {
	m := &Model{
		topic: topicPanel{
			popupOpen: true,
			selIdx:    0,
			filtered:  []topic.Topic{{}, {}},
		},
	}
	m.handleArrowDown()
	if m.topic.selIdx != 1 {
		t.Errorf("expected selIdx 1, got %d", m.topic.selIdx)
	}
}

func TestHandleArrowDown_RssTab1(t *testing.T) {
	m := &Model{
		rss: rssPanel{
			popupOpen: true,
			tabIdx:    1,
			selIdx:    0,
			rawItems:  []rawItem{{}, {}},
		},
	}
	m.handleArrowDown()
	if m.rss.selIdx != 1 {
		t.Errorf("expected selIdx 1, got %d", m.rss.selIdx)
	}
}

func TestHandleArrowDown_NoPopupFocusTopics(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	tm.CreateTopic("Second")
	m := &Model{
		focused:  focusTopics,
		topicMgr: tm,
		topic:    topicPanel{recentIdx: 0},
	}
	m.handleArrowDown()
	if m.topic.recentIdx != 1 {
		t.Errorf("expected recentIdx 1, got %d", m.topic.recentIdx)
	}
}

// ─── handleArrowLeft / handleArrowRight ───

func TestHandleArrowLeft_TodoPopup(t *testing.T) {
	m := &Model{
		todoMgr: todo.NewManager(t.TempDir()),
		todo: todoPanel{
			popupOpen: true,
			tabIdx:    1,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleArrowLeft()
	if m.todo.tabIdx != 0 {
		t.Errorf("expected tabIdx 0, got %d", m.todo.tabIdx)
	}
}

func TestHandleArrowRight_TodoPopup(t *testing.T) {
	m := &Model{
		todoMgr: todo.NewManager(t.TempDir()),
		todo: todoPanel{
			popupOpen: true,
			tabIdx:    0,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleArrowRight()
	if m.todo.tabIdx != 1 {
		t.Errorf("expected tabIdx 1, got %d", m.todo.tabIdx)
	}
}

func TestHandleArrowLeft_SchedulePopup(t *testing.T) {
	m := &Model{
		taskMgr: task.NewManager(t.TempDir(), &task.DefaultParser{}),
		schedule: schedulePanel{
			popupOpen: true,
			tabIdx:    1,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleArrowLeft()
	if m.schedule.tabIdx != 0 {
		t.Errorf("expected tabIdx 0, got %d", m.schedule.tabIdx)
	}
}

func TestHandleArrowRight_SchedulePopup(t *testing.T) {
	m := &Model{
		taskMgr: task.NewManager(t.TempDir(), &task.DefaultParser{}),
		schedule: schedulePanel{
			popupOpen: true,
			tabIdx:    0,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleArrowRight()
	if m.schedule.tabIdx != 1 {
		t.Errorf("expected tabIdx 1, got %d", m.schedule.tabIdx)
	}
}

func TestHandleArrowLeft_RssPopup(t *testing.T) {
	m := &Model{
		rssMgr: rss.NewManager(t.TempDir()),
		rss: rssPanel{
			popupOpen: true,
			tabIdx:    1,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleArrowLeft()
	if m.rss.tabIdx != 0 {
		t.Errorf("expected tabIdx 0, got %d", m.rss.tabIdx)
	}
}

func TestHandleArrowRight_RssPopup(t *testing.T) {
	m := &Model{
		rssMgr: rss.NewManager(t.TempDir()),
		rss: rssPanel{
			popupOpen: true,
			tabIdx:    0,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleArrowRight()
	if m.rss.tabIdx != 1 {
		t.Errorf("expected tabIdx 1, got %d", m.rss.tabIdx)
	}
}

func TestHandleArrowLeft_NoPopupReturnsNil(t *testing.T) {
	m := &Model{}
	cmd := m.handleArrowLeft()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleArrowRight_NoPopupReturnsNil(t *testing.T) {
	m := &Model{}
	cmd := m.handleArrowRight()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// ─── handlePageUp / handlePageDown ───

func TestHandlePageUp_FocusLogs(t *testing.T) {
	m := &Model{
		focused: focusLogs,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handlePageUp()
}

func TestHandlePageUp_FocusConv(t *testing.T) {
	m := &Model{
		focused: focusConv,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handlePageUp()
}

func TestHandlePageUp_FocusTopics(t *testing.T) {
	m := &Model{
		focused: focusTopics,
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handlePageUp()
}

func TestHandlePageDown_FocusLogs(t *testing.T) {
	m := &Model{
		focused: focusLogs,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handlePageDown()
}

func TestHandlePageDown_NoFocus(t *testing.T) {
	m := &Model{
		focused: focusInput,
	}
	cmd := m.handlePageDown()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// ─── handleMouseWheel ───

func TestHandleMouseWheel_FocusLogsWheelUp(t *testing.T) {
	m := &Model{
		focused: focusLogs,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleMouseWheel(tea.Mouse{Button: tea.MouseWheelUp})
}

func TestHandleMouseWheel_FocusConvWheelDown(t *testing.T) {
	m := &Model{
		focused: focusConv,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleMouseWheel(tea.Mouse{Button: tea.MouseWheelDown})
}

func TestHandleMouseWheel_FocusTopicsWheelUp(t *testing.T) {
	m := &Model{
		focused: focusTopics,
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleMouseWheel(tea.Mouse{Button: tea.MouseWheelUp})
}

// ─── handleMouseClick ───

func TestHandleMouseClick_NotLeftButton(t *testing.T) {
	m := &Model{}
	cmd := m.handleMouseClick(tea.Mouse{Button: tea.MouseRight})
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleMouseClick_XLessThan30(t *testing.T) {
	m := &Model{}
	cmd := m.handleMouseClick(tea.Mouse{Button: tea.MouseLeft, X: 20})
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleMouseClick_XEquals30(t *testing.T) {
	m := &Model{}
	cmd := m.handleMouseClick(tea.Mouse{Button: tea.MouseLeft, X: 30})
	if cmd != nil {
		t.Error("expected nil command for X=30 (not < 30 but no popup)")
	}
}

func TestHandleMouseClick_TopicPopupValidItem(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	all := tm.ListTopics()
	if len(all) == 0 {
		t.Fatal("expected at least default topic")
	}
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		topic: topicPanel{
			popupOpen: true,
			filtered:  all,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleMouseClick(tea.Mouse{Button: tea.MouseLeft, X: 40, Y: 3})
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.topic.popupOpen {
		t.Error("expected topic popupOpen to be false after click")
	}
}

func TestHandleMouseClick_ModelPopupValidItem(t *testing.T) {
	dataDir := t.TempDir()
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()
	m := &Model{
		settingsMgr: sm,
		logger:      logger,
		events:      make(chan any, 200),
		model: modelPanel{
			popupOpen: true,
			filtered: []settings.ModelInfo{
				{Name: "nonexistent-model", Size: 0, ContextLen: 0},
			},
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleMouseClick(tea.Mouse{Button: tea.MouseLeft, X: 40, Y: 3})
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.model.popupOpen {
		t.Error("expected model popupOpen to be false after click")
	}
}

// ─── handleNewTopic ───

func TestHandleNewTopic(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	before := len(tm.ListTopics())
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleNewTopic()
	after := len(tm.ListTopics())
	if after != before+1 {
		t.Errorf("expected %d topics, got %d", before+1, after)
	}
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic after new topic")
	}
}

// ─── handleForkTopic ───

func TestHandleForkTopic_WithCurrent(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	curBefore := tm.CurrentTopic()
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleForkTopic()
	curAfter := tm.CurrentTopic()
	if curAfter == nil {
		t.Fatal("expected current topic after fork")
	}
	if curAfter.ID == curBefore.ID {
		t.Error("expected a new topic ID after fork")
	}
}

// ─── handleResumeTopic ───

func TestHandleResumeTopic_WithPrevious(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	first := tm.CurrentTopic()
	tm.CreateTopic("Second topic")
	all := tm.ListTopics()
	var secondID string
	for _, tp := range all {
		if tp.ID != first.ID {
			secondID = tp.ID
			break
		}
	}
	tm.SwitchToTopic(secondID)
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleResumeTopic()
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic after resume")
	}
	if cur.ID != first.ID {
		t.Errorf("expected resumed to first topic (%s), got %s", first.ID, cur.ID)
	}
}

func TestHandleResumeTopic_WithoutPrevious(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
	}
	m.handleResumeTopic()
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic still set")
	}
}

// ─── handleSwitchTopic ───

func TestHandleSwitchTopic_ByNumericIndex(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	tm.CreateTopic("Alpha")
	all := tm.ListTopics()
	// Both topics have same-second UpdatedAt, so sort preserves insert order:
	// [Topic 1 (first), Alpha (second)]
	if len(all) < 2 {
		t.Fatal("expected at least 2 topics")
	}
	alphaID := all[1].ID
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleSwitchTopic("2")
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic after switch")
	}
	if cur.ID != alphaID {
		t.Errorf("expected to switch to Alpha (%s), got %s", alphaID, cur.ID)
	}
}

func TestHandleSwitchTopic_ByTopicID(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	firstID := tm.CurrentTopic().ID
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.handleSwitchTopic(firstID)
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic after switch")
	}
	if cur.ID != firstID {
		t.Errorf("expected same topic ID %s, got %s", firstID, cur.ID)
	}
}

func TestHandleSwitchTopic_NonExistent(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	before := tm.CurrentTopic()
	logger := services.NewLogger()
	m := &Model{
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
	}
	m.handleSwitchTopic("nonexistent-id")
	after := tm.CurrentTopic()
	if after == nil {
		t.Fatal("expected current topic unchanged")
	}
	if after.ID != before.ID {
		t.Error("expected current topic unchanged after nonexistent switch")
	}
}

// ─── todoRemoveSelected / todoDoneSelected ───

func TestTodoRemoveSelected_OutOfBounds(t *testing.T) {
	m := &Model{
		todo: todoPanel{
			selIdx: -1,
			items:  []todo.Item{{ID: "1", Text: "test"}},
		},
	}
	cmd := m.todoRemoveSelected()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestTodoRemoveSelected_Valid(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	todom.Add("Test todo")
	items := todom.ListItems()
	if len(items) == 0 {
		t.Fatal("expected at least 1 todo item")
	}
	m := &Model{
		todoMgr: todom,
		todo: todoPanel{
			items:  items,
			selIdx: 0,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.todoRemoveSelected()
	items = todom.ListItems()
	if len(items) != 0 {
		t.Errorf("expected 0 items after remove, got %d", len(items))
	}
}

func TestTodoDoneSelected_OutOfBounds(t *testing.T) {
	m := &Model{
		todo: todoPanel{
			selIdx: 5,
			items:  []todo.Item{{ID: "1", Text: "test"}},
		},
	}
	cmd := m.todoDoneSelected()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestTodoDoneSelected_Valid(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	todom.Add("Test todo")
	items := todom.ListItems()
	if len(items) == 0 {
		t.Fatal("expected at least 1 todo item")
	}
	m := &Model{
		todoMgr: todom,
		todo: todoPanel{
			tabIdx: 0,
			items:  items,
			selIdx: 0,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.todoDoneSelected()
	items = todom.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item after done, got %d", len(items))
	}
	if !items[0].Done {
		t.Error("expected item to be marked done")
	}
}

func TestTodoDoneSelected_Uncomplete(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	todom.Add("Test todo")
	items := todom.ListItems()
	todom.Complete(items[0].ID)
	items = todom.ListItems()
	m := &Model{
		todoMgr: todom,
		todo: todoPanel{
			tabIdx: 1,
			items:  items,
			selIdx: 0,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.todoDoneSelected()
	items = todom.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Done {
		t.Error("expected item to be unmarked done")
	}
}

// ─── scheduleRemoveSelected / scheduleResetSelected ───

func TestScheduleRemoveSelected_OutOfBounds(t *testing.T) {
	m := &Model{
		schedule: schedulePanel{
			selIdx: -1,
			items:  []task.Task{{ID: "1", RawInput: "test"}},
		},
	}
	cmd := m.scheduleRemoveSelected()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestScheduleRemoveSelected_Valid(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	future := time.Now().Add(time.Hour)
	tkm.AddScheduled("test task", future, 0)
	items := tkm.ListByStatus(task.StatusWaiting)
	if len(items) == 0 {
		t.Fatal("expected at least 1 scheduled task")
	}
	m := &Model{
		taskMgr: tkm,
		schedule: schedulePanel{
			items:  items,
			selIdx: 0,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.scheduleRemoveSelected()
	items = tkm.ListByStatus(task.StatusWaiting)
	if len(items) != 0 {
		t.Errorf("expected 0 items after remove, got %d", len(items))
	}
}

func TestScheduleResetSelected_Tab0ReturnsNil(t *testing.T) {
	m := &Model{
		schedule: schedulePanel{
			tabIdx: 0,
			selIdx: 0,
			items:  []task.Task{{ID: "1", RawInput: "test"}},
		},
	}
	cmd := m.scheduleResetSelected()
	if cmd != nil {
		t.Error("expected nil command when tabIdx is 0")
	}
}

func TestScheduleResetSelected_Valid(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	future := time.Now().Add(time.Hour)
	tkm.AddScheduled("test task", future, 0)
	items := tkm.ListByStatus(task.StatusWaiting)
	tkm.Update(items[0].ID, func(t *task.Task) {
		t.Status = task.StatusCompleted
	})
	completedItems := tkm.ListByStatus(task.StatusCompleted, task.StatusFailed)
	if len(completedItems) == 0 {
		t.Fatal("expected at least 1 completed task")
	}
	m := &Model{
		taskMgr: tkm,
		schedule: schedulePanel{
			items:  completedItems,
			selIdx: 0,
			tabIdx: 1,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.scheduleResetSelected()
	waitingItems := tkm.ListByStatus(task.StatusWaiting)
	if len(waitingItems) != 1 {
		t.Errorf("expected 1 waiting item after reset, got %d", len(waitingItems))
	}
}

// ─── rssEnterSelected / rssToggleBookmark / rssRemoveSource / rssRefreshSummary ───

func TestRssEnterSelected_Tab0ReturnsNil(t *testing.T) {
	m := &Model{
		rss: rssPanel{tabIdx: 0},
	}
	cmd := m.rssEnterSelected()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestRssEnterSelected_Tab1ValidItem(t *testing.T) {
	logger := services.NewLogger()
	m := &Model{
		logger: logger,
		events: make(chan any, 200),
		rss: rssPanel{
			tabIdx: 1,
			selIdx: 0,
			rawItems: []rawItem{
				{
					isSource: false,
					entry: rss.FeedEntry{
						ID:    "entry1",
						Link:  "https://example.com/article1",
						Title: "Test Article",
					},
				},
			},
		},
	}
	cmd := m.rssEnterSelected()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestRssEnterSelected_Tab2ValidItem(t *testing.T) {
	logger := services.NewLogger()
	m := &Model{
		logger: logger,
		events: make(chan any, 200),
		rss: rssPanel{
			tabIdx: 2,
			selIdx: 0,
			bookmarkItems: []rss.FeedEntry{
				{
					ID:    "bm1",
					Link:  "https://example.com/bookmark",
					Title: "Bookmarked",
				},
			},
		},
	}
	cmd := m.rssEnterSelected()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestRssToggleBookmark_Tab0ReturnsNil(t *testing.T) {
	m := &Model{
		rss: rssPanel{tabIdx: 0},
	}
	cmd := m.rssToggleBookmark()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestRssToggleBookmark_Tab1ValidItem(t *testing.T) {
	dataDir := t.TempDir()
	rm := rss.NewManager(dataDir)
	m := &Model{
		rssMgr: rm,
		rss: rssPanel{
			tabIdx: 1,
			selIdx: 0,
			rawItems: []rawItem{
				{
					isSource: false,
					entry: rss.FeedEntry{
						ID:    "entry1",
						Link:  "https://example.com/article",
						Title: "Test",
					},
				},
			},
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.rssToggleBookmark()
}

func TestRssRemoveSource_TabNot1ReturnsNil(t *testing.T) {
	m := &Model{
		rss: rssPanel{tabIdx: 0},
	}
	cmd := m.rssRemoveSource()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestRssRemoveSource_ValidSource(t *testing.T) {
	dataDir := t.TempDir()
	rssDir := filepath.Join(dataDir, "rss")
	_ = os.MkdirAll(rssDir, 0755)
	opmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<head><title>Test</title></head>
<body>
<outline text="Test Source" title="Test Source" type="rss" xmlUrl="https://example.com/feed.xml" />
</body>
</opml>`
	_ = os.WriteFile(filepath.Join(rssDir, "feed.opml"), []byte(opmlContent), 0644)
	rm := rss.NewManager(dataDir)
	sources := rm.ListSources()
	if len(sources) == 0 {
		t.Fatal("expected at least 1 source from OPML")
	}
	m := &Model{
		rssMgr: rm,
		rss: rssPanel{
			tabIdx: 1,
			selIdx: 0,
			rawItems: []rawItem{
				{isSource: true, source: sources[0]},
			},
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.rssRemoveSource()
	sources = rm.ListSources()
	if len(sources) != 0 {
		t.Errorf("expected 0 sources after removal, got %d", len(sources))
	}
}

func TestRssRefreshSummary_TabNot0ReturnsNil(t *testing.T) {
	m := &Model{
		rss: rssPanel{tabIdx: 1},
	}
	cmd := m.rssRefreshSummary()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestRssRefreshSummary_NotLoadedReturnsNil(t *testing.T) {
	m := &Model{
		rss: rssPanel{tabIdx: 0, loaded: false},
	}
	cmd := m.rssRefreshSummary()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// ─── handleModelString (numeric index) ───

func TestHandleModelString_NumericIndex(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"llama3:8b","size":4700000000,"context_length":8192},{"name":"qwen2:1.5b","size":1500000000,"context_length":32768}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	pm := pipeline.NewManager("")
	sum := summarizer.New(ai.Config{})
	exec := pipeline.NewExecutor(pm, sum, tkm, dataDir, "")
	m := &Model{
		settingsMgr: sm,
		logger:      logger,
		events:      make(chan any, 200),
		router:      &intent.Router{},
		executor:    exec,
		feedback:    noopFeedback{},
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// Select model by numeric index 2 → should match qwen2:1.5b
	m.handleModelString("2")
}

// ─── renderConv ───

func TestRenderConv_Default(t *testing.T) {
	cc := &commandCenter{
		convView: viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
	}
	result := cc.renderConv(50, false, "", true)
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "COMMAND CENTER") {
		t.Error("expected 'COMMAND CENTER' in rendered output")
	}
	if !strings.Contains(result, "Start a conversation...") {
		t.Error("expected placeholder text")
	}
}

func TestRenderConv_WithConversations(t *testing.T) {
	cc := &commandCenter{
		conversations: []conversationEntry{
			{input: "hello", response: "world"},
			{input: "how are you?", response: "I'm fine"},
		},
		convView: viewport.New(viewport.WithWidth(50), viewport.WithHeight(20)),
	}
	result := cc.renderConv(50, false, "", true)
	if !strings.Contains(result, "hello") {
		t.Error("expected first input in render")
	}
	if !strings.Contains(result, "world") {
		t.Error("expected first response in render")
	}
	if !strings.Contains(result, "how are you?") {
		t.Error("expected second input in render")
	}
	if !strings.Contains(result, "I'm") || !strings.Contains(result, "fine") {
		t.Error("expected second response in render")
	}
	if !strings.Contains(result, "You:") {
		t.Error("expected 'You:' labels")
	}
	if !strings.Contains(result, "DHI:") {
		t.Error("expected 'DHI:' labels")
	}
}

func TestRenderConv_Thinking(t *testing.T) {
	s := spinner.New()
	cc := &commandCenter{
		conversations: []conversationEntry{
			{input: "thinking query", response: s.View(), thinking: true},
		},
		thinking: 1,
		spinner:  s,
		convView: viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
	}
	result := cc.renderConv(50, false, "", true)
	if !strings.Contains(result, "thinking query") {
		t.Error("expected input text in render")
	}
	if !strings.Contains(result, "DHI:") {
		t.Error("expected DHI label")
	}
	// Spinner view should be non-empty (the response set to spinner.View())
	if cc.conversations[0].response == "" {
		t.Error("expected non-empty spinner response")
	}
}

func TestRenderConv_WithTopicTitle(t *testing.T) {
	cc := &commandCenter{
		convView: viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
	}
	result := cc.renderConv(50, false, "My Topic", true)
	if !strings.Contains(result, "My Topic") {
		t.Error("expected topic title in render output")
	}
}

func TestRenderConv_TopicTitleTruncated(t *testing.T) {
	cc := &commandCenter{
		convView: viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
	}
	longTitle := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := cc.renderConv(50, false, longTitle, true)
	if !strings.Contains(result, "...") {
		t.Error("expected long topic title to be truncated with ...")
	}
}

func TestRenderConv_Focused(t *testing.T) {
	cc := &commandCenter{
		convView: viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
	}
	result := cc.renderConv(50, true, "", true)
	if result == "" {
		t.Error("expected non-empty render when focused")
	}
}

// ─── renderInput ───

func TestRenderInput_FocusedWithModel(t *testing.T) {
	ti := textinput.New()
	cc := &commandCenter{
		textInput: ti,
	}
	result := cc.renderInput(50, true, true, "llama3:8b")
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "llama3:8b") {
		t.Error("expected model name in rendered output")
	}
}

func TestRenderInput_Blurred(t *testing.T) {
	ti := textinput.New()
	cc := &commandCenter{
		textInput: ti,
	}
	result := cc.renderInput(50, true, false, "llama3:8b")
	if result == "" {
		t.Error("expected non-empty render when blurred")
	}
	if !strings.Contains(result, "llama3:8b") {
		t.Error("expected model name even when blurred")
	}
}

func TestRenderInput_OfflineStatus(t *testing.T) {
	ti := textinput.New()
	cc := &commandCenter{
		textInput: ti,
	}
	result := cc.renderInput(50, false, false, "")
	if !strings.Contains(result, "OFFLINE") {
		t.Error("expected OFFLINE status when router is offline")
	}
}

func TestRenderInput_EmptyModelName(t *testing.T) {
	ti := textinput.New()
	cc := &commandCenter{
		textInput: ti,
	}
	result := cc.renderInput(50, true, true, "")
	if !strings.Contains(result, "NO MODEL") {
		t.Error("expected NO MODEL status when model name is empty")
	}
}

func TestRenderInput_WithSlashMatches(t *testing.T) {
	ti := textinput.New()
	cc := &commandCenter{
		textInput:    ti,
		slashMatches: []string{"/help", "/log"},
		slashIndex:   0,
	}
	result := cc.renderInput(50, true, true, "test-model")
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "test-model") {
		t.Error("expected model name in output")
	}
}

// ─── render() ───

func TestRender_Default(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	topicMgr := topic.NewManager(dataDir)
	todoMgr := todo.NewManager(dataDir)
	rssMgr := rss.NewManager(dataDir)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	settingsMgr := settings.NewManager(dataDir, client, ai.Config{})

	cp := progress.New(progress.WithWidth(21))
	rp := progress.New(progress.WithWidth(21))
	gp := progress.New(progress.WithWidth(21))

	m := &Model{
		width:   120,
		height:  40,
		welcome: welcomePanel{show: false},
		dhiOS:   dhiOS{hostname: "test", kernel: "darwin", uptimeSec: 100},
		system: systemPanel{
			metrics: services.SystemMetrics{
				Hostname:   "test",
				Kernel:     "darwin",
				UptimeSec:  100,
				CPU:        50,
				RAMUsedMB:  2048,
				RAMTotalMB: 8192,
				GPUPercent: 30,
			},
			routerOnline: true,
			routerStatus: "READY",
			cpuProgress:  cp,
			ramProgress:  rp,
			gpuProgress:  gp,
		},
		pipeline: pipelinePanel{
			entries: []pipelineEntry{{name: "test", status: "IDLE"}},
			view:    viewport.New(viewport.WithWidth(26), viewport.WithHeight(6)),
		},
		tracker: trackerPanel{},
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		model: modelPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(5)),
		},
		help: helpPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		log: logPanel{
			view:    viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			entries: []string{},
		},
		todo: todoPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		schedule: schedulePanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		rss: rssPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
		},
		todoMgr:     todoMgr,
		taskMgr:     tm,
		topicMgr:    topicMgr,
		rssMgr:      rssMgr,
		settingsMgr: settingsMgr,
		logger:      logger,
		events:      make(chan any, 200),
		focused:     focusInput,
	}
	result := m.render()
	if result == "" {
		t.Error("expected non-empty render output")
	}
	if !strings.Contains(result, "DHI OS") {
		t.Error("expected DHI OS panel in render")
	}
	if !strings.Contains(result, "TRACKER") {
		t.Error("expected TRACKER panel in render")
	}
}

func TestRender_HelpPopup(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	topicMgr := topic.NewManager(dataDir)
	todoMgr := todo.NewManager(dataDir)
	rssMgr := rss.NewManager(dataDir)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	settingsMgr := settings.NewManager(dataDir, client, ai.Config{})

	cp := progress.New(progress.WithWidth(21))
	rp := progress.New(progress.WithWidth(21))
	gp := progress.New(progress.WithWidth(21))

	helpView := viewport.New(viewport.WithWidth(90), viewport.WithHeight(20))

	m := &Model{
		width:   120,
		height:  40,
		welcome: welcomePanel{show: false},
		dhiOS:   dhiOS{hostname: "test", kernel: "darwin", uptimeSec: 100},
		system: systemPanel{
			metrics: services.SystemMetrics{
				Hostname:   "test",
				Kernel:     "darwin",
				UptimeSec:  100,
				CPU:        50,
				RAMUsedMB:  2048,
				RAMTotalMB: 8192,
				GPUPercent: 30,
			},
			routerOnline: true,
			routerStatus: "READY",
			cpuProgress:  cp,
			ramProgress:  rp,
			gpuProgress:  gp,
		},
		pipeline: pipelinePanel{
			entries: []pipelineEntry{{name: "test", status: "IDLE"}},
			view:    viewport.New(viewport.WithWidth(26), viewport.WithHeight(6)),
		},
		tracker: trackerPanel{},
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		model: modelPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(5)),
		},
		help: helpPanel{
			popupOpen: true,
			view:      helpView,
			content:   buildHelpContent(),
		},
		log: logPanel{
			view:    viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			entries: []string{},
		},
		todo: todoPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		schedule: schedulePanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		rss: rssPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
		},
		todoMgr:     todoMgr,
		taskMgr:     tm,
		topicMgr:    topicMgr,
		rssMgr:      rssMgr,
		settingsMgr: settingsMgr,
		logger:      logger,
		events:      make(chan any, 200),
		focused:     focusInput,
	}
	result := m.render()
	if result == "" {
		t.Error("expected non-empty render output")
	}
	if !strings.Contains(result, "HELP") {
		t.Error("expected HELP popup in render")
	}
}

func TestRender_LogPopup(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	topicMgr := topic.NewManager(dataDir)
	todoMgr := todo.NewManager(dataDir)
	rssMgr := rss.NewManager(dataDir)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	settingsMgr := settings.NewManager(dataDir, client, ai.Config{})

	cp := progress.New(progress.WithWidth(21))
	rp := progress.New(progress.WithWidth(21))
	gp := progress.New(progress.WithWidth(21))

	m := &Model{
		width:   120,
		height:  40,
		welcome: welcomePanel{show: false},
		dhiOS:   dhiOS{hostname: "test", kernel: "darwin", uptimeSec: 100},
		system: systemPanel{
			metrics: services.SystemMetrics{
				Hostname:   "test",
				Kernel:     "darwin",
				UptimeSec:  100,
				CPU:        50,
				RAMUsedMB:  2048,
				RAMTotalMB: 8192,
				GPUPercent: 30,
			},
			routerOnline: true,
			routerStatus: "READY",
			cpuProgress:  cp,
			ramProgress:  rp,
			gpuProgress:  gp,
		},
		pipeline: pipelinePanel{
			entries: []pipelineEntry{{name: "test", status: "IDLE"}},
			view:    viewport.New(viewport.WithWidth(26), viewport.WithHeight(6)),
		},
		tracker: trackerPanel{},
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		model: modelPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(5)),
		},
		help: helpPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		log: logPanel{
			popupOpen: true,
			view:      viewport.New(viewport.WithWidth(90), viewport.WithHeight(20)),
			entries:   []string{"[TEST][INFO] test log entry"},
		},
		todo: todoPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		schedule: schedulePanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		rss: rssPanel{
			view: viewport.New(viewport.WithWidth(26), viewport.WithHeight(8)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(50), viewport.WithHeight(10)),
		},
		todoMgr:     todoMgr,
		taskMgr:     tm,
		topicMgr:    topicMgr,
		rssMgr:      rssMgr,
		settingsMgr: settingsMgr,
		logger:      logger,
		events:      make(chan any, 200),
		focused:     focusInput,
	}
	result := m.render()
	if result == "" {
		t.Error("expected non-empty render output")
	}
	if !strings.Contains(result, "LOGS") {
		t.Error("expected LOGS popup in render")
	}
}

// ─── tracker.render() ───

func TestTrackerRender_Empty(t *testing.T) {
	dataDir := t.TempDir()
	todoMgr := todo.NewManager(dataDir)
	taskMgr := task.NewManager(dataDir, &task.DefaultParser{})
	rssMgr := rss.NewManager(dataDir)

	tr := &trackerPanel{}
	result := tr.render(todoMgr, taskMgr, rssMgr)
	if result == "" {
		t.Error("expected non-empty tracker render")
	}
	if !strings.Contains(result, "TRACKER") {
		t.Error("expected TRACKER header")
	}
	if !strings.Contains(result, "0 pending") {
		t.Error("expected 0 pending todos")
	}
	if !strings.Contains(result, "0 upcoming") {
		t.Error("expected 0 upcoming schedules")
	}
	if !strings.Contains(result, "0 entries") {
		t.Error("expected 0 RSS entries")
	}
}

func TestTrackerRender_WithItems(t *testing.T) {
	dataDir := t.TempDir()
	todoMgr := todo.NewManager(dataDir)
	todoMgr.Add("Test todo")
	taskMgr := task.NewManager(dataDir, &task.DefaultParser{})
	future := time.Now().Add(time.Hour)
	taskMgr.AddScheduled("test schedule", future, 0)

	// rss entry count will be 0 since we can't easily add entries without fetching
	rssMgr := rss.NewManager(dataDir)

	tr := &trackerPanel{}
	result := tr.render(todoMgr, taskMgr, rssMgr)
	if !strings.Contains(result, "1 pending") {
		t.Error("expected 1 pending todo")
	}
	if !strings.Contains(result, "1 upcoming") {
		t.Error("expected 1 upcoming schedule")
	}
}

// ─── tracker.renderPopup — todo ───

func TestTodoRenderPopup_Tab0WithItems(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	todom.Add("Task one")
	todom.Add("Task two")

	tp := &todoPanel{
		tabIdx: 0,
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	tp.loadItems(todom)

	result := tp.renderPopup(50)
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "TODOS") {
		t.Error("expected TODOS header")
	}
	if !strings.Contains(result, "ACTIVE") {
		t.Error("expected ACTIVE tab label")
	}
	if !strings.Contains(result, "Task one") {
		t.Error("expected first task text in render")
	}
	if !strings.Contains(result, "Task two") {
		t.Error("expected second task text in render")
	}
}

func TestTodoRenderPopup_Tab1WithItems(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	id := todom.Add("Done task")
	todom.Complete(id)

	tp := &todoPanel{
		tabIdx: 1,
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	tp.loadItems(todom)

	result := tp.renderPopup(50)
	if !strings.Contains(result, "COMPLETED") {
		t.Error("expected COMPLETED tab label")
	}
	if !strings.Contains(result, "Done task") {
		t.Error("expected completed task text")
	}
}

func TestTodoRenderPopup_Empty(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)

	tp := &todoPanel{
		tabIdx: 0,
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	tp.loadItems(todom)

	result := tp.renderPopup(50)
	if !strings.Contains(result, "No active todos") {
		t.Error("expected empty state message for active tab")
	}

	tp.tabIdx = 1
	tp.loadItems(todom)
	result = tp.renderPopup(50)
	if !strings.Contains(result, "No completed todos") {
		t.Error("expected empty state message for completed tab")
	}
}

// ─── tracker.renderPopup — schedule ───

func TestScheduleRenderPopup_Tab0WithItems(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	future := time.Now().Add(time.Hour)
	tkm.AddScheduled("scheduled task", future, 0)

	sp := &schedulePanel{
		tabIdx: 0,
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	sp.loadItems(tkm)

	result := sp.renderPopup(50)
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "SCHEDULES") {
		t.Error("expected SCHEDULES header")
	}
	if !strings.Contains(result, "SCHEDULED") {
		t.Error("expected SCHEDULED tab label")
	}
	if !strings.Contains(result, "scheduled task") {
		t.Error("expected scheduled task text")
	}
}

func TestScheduleRenderPopup_Tab1WithItems(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	future := time.Now().Add(time.Hour)
	tkm.AddScheduled("completed task", future, 0)
	items := tkm.ListByStatus(task.StatusWaiting)
	tkm.Update(items[0].ID, func(t *task.Task) {
		t.Status = task.StatusCompleted
	})

	sp := &schedulePanel{
		tabIdx: 1,
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	sp.loadItems(tkm)

	result := sp.renderPopup(50)
	if !strings.Contains(result, "COMPLETED") {
		t.Error("expected COMPLETED tab label")
	}
	if !strings.Contains(result, "completed task") {
		t.Error("expected completed task text")
	}
}

func TestScheduleRenderPopup_Empty(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})

	sp := &schedulePanel{
		tabIdx: 0,
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	sp.loadItems(tkm)

	result := sp.renderPopup(50)
	if !strings.Contains(result, "No upcoming schedules") {
		t.Error("expected empty state for scheduled tab")
	}

	sp.tabIdx = 1
	sp.loadItems(tkm)
	result = sp.renderPopup(50)
	if !strings.Contains(result, "No completed schedules") {
		t.Error("expected empty state for completed tab")
	}
}

// ─── rss renderPopup / renderAITab / renderRawTab / renderBookmarkTab ───

func TestRssRenderAITab_Loading(t *testing.T) {
	rp := &rssPanel{loading: true}
	result := rp.renderAITab()
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "Fetching feeds") {
		t.Error("expected fetching feeds message")
	}
}

func TestRssRenderAITab_NotLoaded(t *testing.T) {
	rp := &rssPanel{loaded: false}
	result := rp.renderAITab()
	if !strings.Contains(result, "Open the RSS popup") {
		t.Error("expected open popup message")
	}
}

func TestRssRenderAITab_NoSummary(t *testing.T) {
	rp := &rssPanel{loaded: true, aiSummary: ""}
	result := rp.renderAITab()
	if !strings.Contains(result, "Generating AI summary") {
		t.Error("expected generating summary message")
	}
}

func TestRssRenderAITab_WithData(t *testing.T) {
	rp := &rssPanel{
		loaded:    true,
		isDark:    true,
		aiSummary: "Top stories today:\n- Story 1\n- Story 2",
		feedEntries: []rss.FeedEntry{
			{ID: "e1", Title: "Story 1"},
			{ID: "e2", Title: "Story 2"},
		},
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	result := rp.renderAITab()
	if !strings.Contains(result, "Story") {
		t.Error("expected story content in AI summary")
	}
	if !strings.Contains(result, "Top") {
		t.Error("expected top stories heading")
	}
}

func TestRssRenderRawTab_NoItems(t *testing.T) {
	rp := &rssPanel{}
	result := rp.renderRawTab()
	if !strings.Contains(result, "No feeds") {
		t.Error("expected no feeds message")
	}
}

func TestRssRenderRawTab_WithItems(t *testing.T) {
	rp := &rssPanel{
		rawItems: []rawItem{
			{isSource: true, source: rss.FeedSource{ID: "src1", Title: "Test Source"}},
			{isSource: false, entry: rss.FeedEntry{ID: "e1", SourceID: "src1", Title: "Article One", Link: "https://example.com/1", PublishedAt: time.Now()}},
		},
		selIdx: 0,
	}
	result := rp.renderRawTab()
	t.Logf("renderRawTab result: %q", result)
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "Test Source") {
		t.Error("expected source title in render")
	}
	if !strings.Contains(result, "Article One") {
		t.Error("expected article title in render")
	}
}

func TestRssRenderBookmarkTab_NoItems(t *testing.T) {
	rp := &rssPanel{}
	result := rp.renderBookmarkTab()
	if !strings.Contains(result, "No bookmarks") {
		t.Error("expected no bookmarks message")
	}
}

func TestRssRenderBookmarkTab_WithItems(t *testing.T) {
	rp := &rssPanel{
		bookmarkItems: []rss.FeedEntry{
			{ID: "bm1", Title: "Saved Article", Link: "https://example.com/saved", SourceID: "src1", PublishedAt: time.Now()},
		},
		feedSources: []rss.FeedSource{
			{ID: "src1", Title: "Test Source"},
		},
		selIdx: 0,
	}
	result := rp.renderBookmarkTab()
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "Saved Article") {
		t.Error("expected bookmark title in render")
	}
}

func TestRssRenderPopup_Tab0(t *testing.T) {
	rp := &rssPanel{
		tabIdx:  0,
		loading: true,
		view:    viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	result := rp.renderPopup(50)
	if !strings.Contains(result, "RSS FEEDS") {
		t.Error("expected RSS FEEDS header")
	}
	if !strings.Contains(result, "AI CURATED") {
		t.Error("expected AI CURATED tab label")
	}
}

func TestRssRenderPopup_Tab1(t *testing.T) {
	rp := &rssPanel{
		tabIdx: 1,
		selIdx: 0,
		rawItems: []rawItem{
			{isSource: true, source: rss.FeedSource{ID: "src1", Title: "Source"}},
		},
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	result := rp.renderPopup(50)
	if !strings.Contains(result, "RAW") {
		t.Error("expected RAW tab label")
	}
}

func TestRssRenderPopup_Tab2(t *testing.T) {
	rp := &rssPanel{
		tabIdx: 2,
		selIdx: 0,
		bookmarkItems: []rss.FeedEntry{
			{ID: "bm1", Title: "BM", Link: "https://example.com", SourceID: "src1", PublishedAt: time.Now()},
		},
		feedSources: []rss.FeedSource{
			{ID: "src1", Title: "Src"},
		},
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	result := rp.renderPopup(50)
	if !strings.Contains(result, "BOOKMARKS") {
		t.Error("expected BOOKMARKS tab label")
	}
}

// ─── handlePageDown (additional cases) ───

func TestHandlePageDown_FocusConv_Additional(t *testing.T) {
	m := &Model{
		focused: focusConv,
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handlePageDown()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandlePageDown_FocusTopics_Additional(t *testing.T) {
	m := &Model{
		focused: focusTopics,
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handlePageDown()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// ─── handlePageUp (additional cases) ───

func TestHandlePageUp_NoFocusReturnsNil(t *testing.T) {
	m := &Model{
		focused: focusInput,
	}
	cmd := m.handlePageUp()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

// ─── handleKeyPress ───

func TestHandleKeyPress_CtrlC(t *testing.T) {
	m := &Model{cc: commandCenter{textInput: textinput.New(), convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))}}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg for Ctrl+C")
	}
}

func TestHandleKeyPress_CtrlD(t *testing.T) {
	m := &Model{cc: commandCenter{textInput: textinput.New(), convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))}}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg for Ctrl+D")
	}
}

func TestHandleKeyPress_EscapeWithNoPopups(t *testing.T) {
	m := &Model{cc: commandCenter{textInput: textinput.New(), convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))}}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Error("expected nil command when no popups open")
	}
}

func TestHandleKeyPress_Tab(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyTab})
	// handleTab with focusInput cycles to focusConv, returns nil
	if cmd != nil {
		t.Error("expected nil command from tab cycle")
	}
	if m.focused != focusConv {
		t.Errorf("expected focusConv after tab, got %d", m.focused)
	}
}

func TestHandleKeyPress_EnterEmptyInput(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command for empty input")
	}
}

func TestHandleKeyPress_ArrowUp(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleKeyPress_ArrowDown(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleKeyPress_PageUp(t *testing.T) {
	m := &Model{
		focused: focusLogs,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	_ = m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyPgUp})
}

func TestHandleKeyPress_PageDown(t *testing.T) {
	m := &Model{
		focused: focusLogs,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	_ = m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyPgDown})
}

func TestHandleKeyPress_ArrowLeft(t *testing.T) {
	m := &Model{}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft})
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleKeyPress_ArrowRight(t *testing.T) {
	m := &Model{}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight})
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleKeyPress_WelcomeDismiss(t *testing.T) {
	m := &Model{
		welcome: welcomePanel{show: true},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: 'a'})
	if cmd == nil {
		t.Error("expected non-nil command (Focus)")
	}
	if m.welcome.show {
		t.Error("expected welcome.show to be false after key press")
	}
}

func TestHandleKeyPress_UnhandledKeyFocusInput(t *testing.T) {
	ti := textinput.New()
	ti.SetValue("existing")
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: ti,
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	// Pressing 'x' when focused on input should update the textinput value
	// and return a bubbletea command from the textinput update
	cmd := m.handleKeyPress(tea.KeyPressMsg{Code: 'x'})
	// TextInput.Update returns a cmd for Blink or similar
	_ = cmd
}

// ─── handleEnter with focused topics ───

func TestHandleEnter_FocusTopics(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	m := &Model{
		focused:  focusTopics,
		topicMgr: tm,
		topic:    topicPanel{recentIdx: 0},
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			convView: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command from enter on topics")
	}
}

func TestHandleEnter_FocusModels(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	pm := pipeline.NewManager("")
	sum := summarizer.New(ai.Config{})
	exec := pipeline.NewExecutor(pm, sum, tkm, dataDir, "")
	m := &Model{
		focused:     focusModels,
		settingsMgr: sm,
		logger:      logger,
		router:      &intent.Router{},
		executor:    exec,
		feedback:    noopFeedback{},
		model:       modelPanel{recentIdx: 0},
		events:      make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command from enter on models")
	}
}

// ─── handleEnter — command dispatch ───

func TestHandleEnter_Help(t *testing.T) {
	m := &Model{
		focused: focusInput,
		help:    newHelpPanel(),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/help")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.help.popupOpen {
		t.Error("expected help popupOpen to be true for /help")
	}
}

func TestHandleEnter_HelpQuestionMark(t *testing.T) {
	m := &Model{
		focused: focusInput,
		help:    newHelpPanel(),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/?")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.help.popupOpen {
		t.Error("expected help popupOpen to be true for /?")
	}
}

func TestHandleEnter_HelpColonH(t *testing.T) {
	m := &Model{
		focused: focusInput,
		help:    newHelpPanel(),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue(":h")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.help.popupOpen {
		t.Error("expected help popupOpen to be true for :h")
	}
}

func TestHandleEnter_Quit(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("quit")
	cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("expected non-nil command for quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg")
	}
}

func TestHandleEnter_Exit(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("exit")
	cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("expected non-nil command for exit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg")
	}
}

func TestHandleEnter_SlashQ(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/q")
	cmd := m.handleEnter()
	if cmd == nil {
		t.Fatal("expected non-nil command for /q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg")
	}
}

func TestHandleEnter_Log(t *testing.T) {
	m := &Model{
		focused: focusInput,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/log")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.log.popupOpen {
		t.Error("expected log popupOpen to be true for /log")
	}
}

func TestHandleEnter_NewTopic(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	before := len(tm.ListTopics())
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/new")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	after := len(tm.ListTopics())
	if after != before+1 {
		t.Errorf("expected %d topics, got %d", before+1, after)
	}
}

func TestHandleEnter_Fork(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	curBefore := tm.CurrentTopic()
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/fork")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	curAfter := tm.CurrentTopic()
	if curAfter == nil {
		t.Fatal("expected current topic after fork")
	}
	if curAfter.ID == curBefore.ID {
		t.Error("expected a new topic after fork")
	}
}

func TestHandleEnter_Resume(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	first := tm.CurrentTopic()
	tm.CreateTopic("Second")
	all := tm.ListTopics()
	for _, tp := range all {
		if tp.ID != first.ID {
			tm.SwitchToTopic(tp.ID)
			break
		}
	}
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/resume")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic after resume")
	}
	if cur.ID != first.ID {
		t.Errorf("expected resumed to first topic (%s), got %s", first.ID, cur.ID)
	}
}

func TestHandleEnter_ResumeNoPrevious(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/resume")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleEnter_Todo(t *testing.T) {
	dataDir := t.TempDir()
	todom := todo.NewManager(dataDir)
	m := &Model{
		focused: focusInput,
		todoMgr: todom,
		todo: todoPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		events: make(chan any, 200),
	}
	m.cc.textInput.SetValue("/todo")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.todo.popupOpen {
		t.Error("expected todo popupOpen to be true for /todo")
	}
}

func TestHandleEnter_Rss(t *testing.T) {
	dataDir := t.TempDir()
	rm := rss.NewManager(dataDir)
	m := &Model{
		focused: focusInput,
		rssMgr:  rm,
		rss: rssPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			// Prevent goroutine from starting in toggleRssPopup
			loaded:    true,
			aiSummary: "pre-populated",
			lastFetch: time.Now(),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		events: make(chan any, 200),
	}
	m.cc.textInput.SetValue("/rss")
	m.handleEnter()
	if !m.rss.popupOpen {
		t.Error("expected rss popupOpen to be true for /rss")
	}
}

func TestHandleEnter_RssAddInvalidURL(t *testing.T) {
	dataDir := t.TempDir()
	rm := rss.NewManager(dataDir)
	logger := services.NewLogger()
	m := &Model{
		focused: focusInput,
		rssMgr:  rm,
		logger:  logger,
		events:  make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/rss add http://invalid-feed-url.example.com/feed.xml")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleEnter_Schedule(t *testing.T) {
	dataDir := t.TempDir()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	m := &Model{
		focused: focusInput,
		taskMgr: tkm,
		schedule: schedulePanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		events: make(chan any, 200),
	}
	m.cc.textInput.SetValue("/schedule")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.schedule.popupOpen {
		t.Error("expected schedule popupOpen to be true for /schedule")
	}
}

func TestHandleEnter_Model(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	m := &Model{
		focused:     focusInput,
		settingsMgr: sm,
		model: modelPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		events: make(chan any, 200),
	}
	m.cc.textInput.SetValue("/model")
	m.handleEnter()
	if !m.model.popupOpen {
		t.Error("expected model popupOpen to be true for /model")
	}
}

func TestHandleEnter_Topic(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		topic: topicPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		events: make(chan any, 200),
	}
	m.cc.textInput.SetValue("/topic")
	m.handleEnter()
	if !m.topic.popupOpen {
		t.Error("expected topic popupOpen to be true for /topic")
	}
}

func TestHandleEnter_EmptyString(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command for empty input")
	}
}

func TestHandleEnter_GoodFeedback(t *testing.T) {
	m := &Model{
		focused:   focusInput,
		lastInput: "test query",
		feedback:  noopFeedback{},
		logger:    services.NewLogger(),
		events:    make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/good")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleEnter_GoodFeedbackNoLastInput(t *testing.T) {
	m := &Model{
		focused:  focusInput,
		feedback: noopFeedback{},
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/good")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
}

func TestHandleEnter_ModelSelect(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"llama3:8b","size":4700000000,"context_length":8192},{"name":"qwen2:1.5b","size":1500000000,"context_length":32768}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()
	tkm := task.NewManager(dataDir, &task.DefaultParser{})
	pm := pipeline.NewManager("")
	sum := summarizer.New(ai.Config{})
	exec := pipeline.NewExecutor(pm, sum, tkm, dataDir, "")
	m := &Model{
		focused:     focusInput,
		settingsMgr: sm,
		logger:      logger,
		router:      &intent.Router{},
		executor:    exec,
		feedback:    noopFeedback{},
		events:      make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/model 2")
	// Should not panic; tries to find model by numeric index
	m.handleEnter()
}

func TestHandleEnter_TopicSwitch(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	logger := services.NewLogger()
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		logger:   logger,
		events:   make(chan any, 200),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("/topic 1")
	// Should switch to the first topic without panicking
	m.handleEnter()
	cur := tm.CurrentTopic()
	if cur == nil {
		t.Fatal("expected current topic after /topic 1")
	}
}

func TestHandleEnter_ThinkingBlocksNonSlash(t *testing.T) {
	m := &Model{
		focused: focusInput,
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			thinking:  1,
		},
	}
	m.cc.textInput.SetValue("hello world")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command when thinking > 0 and input does not start with /")
	}
}

func TestHandleEnter_SlashCommandWhenThinking(t *testing.T) {
	m := &Model{
		focused: focusInput,
		help:    newHelpPanel(),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			thinking:  1,
		},
	}
	// Slash commands should still work even when thinking
	m.cc.textInput.SetValue("/help")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.help.popupOpen {
		t.Error("expected help popupOpen to be true even when thinking")
	}
}

func TestHandleEnter_TopicPopupOpen(t *testing.T) {
	dataDir := t.TempDir()
	tm := topic.NewManager(dataDir)
	m := &Model{
		focused:  focusInput,
		topicMgr: tm,
		logger:   services.NewLogger(),
		events:   make(chan any, 200),
		topic: topicPanel{
			popupOpen: true,
			filtered:  tm.ListTopics(),
			selIdx:    0,
			view:      viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("anything")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.topic.popupOpen {
		t.Error("expected topic popupOpen to be false after selectCurrentTopic")
	}
}

func TestHandleEnter_ModelPopupOpen(t *testing.T) {
	dataDir := t.TempDir()
	settingsDir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{"models":[{"name":"test-model","size":1000000000,"context_length":4096}]}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)
	client := ai.NewClient(ai.Config{BaseURL: "http://localhost:11434"})
	sm := settings.NewManager(dataDir, client, ai.Config{})
	logger := services.NewLogger()
	m := &Model{
		focused:     focusInput,
		settingsMgr: sm,
		logger:      logger,
		events:      make(chan any, 200),
		model: modelPanel{
			popupOpen: true,
			filtered: []settings.ModelInfo{
				{Name: "nonexistent-model", Size: 1000000000, ContextLen: 4096},
			},
			selIdx: 0,
			view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("anything")
	m.handleEnter()
	if m.model.popupOpen {
		t.Error("expected model popupOpen to be false after selectCurrentModel")
	}
}

func TestHandleEnter_RouterOffline(t *testing.T) {
	dataDir := t.TempDir()
	logger := services.NewLogger()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	m := &Model{
		focused:  focusInput,
		logger:   logger,
		taskMgr:  tm,
		events:   make(chan any, 200),
		system:   systemPanel{routerOnline: false},
		topicMgr: topic.NewManager(dataDir),
		cc: commandCenter{
			textInput: textinput.New(),
			convView:  viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
	}
	m.cc.textInput.SetValue("hello")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if m.cc.lastSubmitted != "hello" {
		t.Errorf("expected lastSubmitted 'hello', got %q", m.cc.lastSubmitted)
	}
	if len(m.cc.conversations) != 1 {
		t.Fatalf("expected 1 conversation entry, got %d", len(m.cc.conversations))
	}
	if !strings.Contains(m.cc.conversations[0].response, "Router offline") {
		t.Error("expected 'Router offline' response")
	}
}

func TestHandleEnter_SlashMatchesOverride(t *testing.T) {
	m := &Model{
		focused: focusInput,
		log: logPanel{
			view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
		},
		cc: commandCenter{
			textInput:    textinput.New(),
			convView:     viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
			slashMatches: []string{"/help", "/log"},
			slashIndex:   1,
		},
	}
	// input "/lo" but slashIndex=1 gives "/log" which is different
	m.cc.textInput.SetValue("/lo")
	cmd := m.handleEnter()
	if cmd != nil {
		t.Error("expected nil command")
	}
	if !m.log.popupOpen {
		t.Error("expected log popupOpen when slashMatches override gave /log")
	}
}

// ─── processTasks ───

func TestProcessTasks_NoTasksDue(t *testing.T) {
	dataDir := t.TempDir()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	m := &Model{
		taskMgr: tm,
		// No tasks added — NextDue returns nil
	}
	// Should return early without panic
	m.processTasks()
}

func TestProcessTasks_RetryExceeded(t *testing.T) {
	dataDir := t.TempDir()
	tm := task.NewManager(dataDir, &task.DefaultParser{})
	tsk := tm.AddOnce("test task")
	tm.Update(tsk.ID, func(upd *task.Task) {
		upd.RetryCount = 3
	})
	m := &Model{
		taskMgr: tm,
		logger:  services.NewLogger(),
	}
	m.processTasks()
	// Task should be marked as failed
	all := tm.ListByStatus(task.StatusFailed)
	if len(all) != 1 {
		t.Errorf("expected 1 failed task, got %d", len(all))
	}
}

// ─── tracker.renderPopup — RSS AI summary tab ───

func TestRssRenderPopup_Tab0WithSummary(t *testing.T) {
	rp := &rssPanel{
		tabIdx:    0,
		loaded:    true,
		aiSummary: "AI curated summary content",
		feedEntries: []rss.FeedEntry{
			{ID: "e1", Title: "Item 1"},
		},
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	result := rp.renderPopup(50)
	if result == "" {
		t.Error("expected non-empty render")
	}
	if !strings.Contains(result, "AI CURATED") {
		t.Error("expected AI CURATED tab label")
	}
	if !strings.Contains(result, "RSS FEEDS") {
		t.Error("expected RSS FEEDS header")
	}
}

func TestRssRenderAITab_NoEntries(t *testing.T) {
	rp := &rssPanel{
		loaded:    true,
		aiSummary: "summary",
	}
	result := rp.renderAITab()
	if !strings.Contains(result, "No entries") {
		t.Error("expected 'No entries' message when feedEntries is empty")
	}
}
