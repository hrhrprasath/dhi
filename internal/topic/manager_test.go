package topic

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	topicsDir := filepath.Join(t.TempDir(), "topics")
	m2 := NewManager(topicsDir)
	if m2 == nil {
		t.Fatal("NewManager with nested dirs returned nil")
	}
}

func TestNewManagerInitializesDefaultTopic(t *testing.T) {
	m := NewManager(t.TempDir())
	if len(m.list.Topics) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(m.list.Topics))
	}
	if m.list.Topics[0].Title != "Topic 1" {
		t.Fatalf("expected title 'Topic 1', got %q", m.list.Topics[0].Title)
	}
	if m.list.Current == "" {
		t.Fatal("expected current to be set")
	}
}

func TestNewManagerCreatesTopicsDirectory(t *testing.T) {
	dir := t.TempDir()
	NewManager(dir)
	topicsDir := filepath.Join(dir, "topics")
	info, err := os.Stat(topicsDir)
	if err != nil {
		t.Fatalf("topics directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("topics path is not a directory")
	}
}

func TestNewManagerMigratesFromSummary(t *testing.T) {
	dir := t.TempDir()
	convDir := filepath.Join(dir, "conversation")
	_ = os.MkdirAll(convDir, 0755)
	summary := map[string]interface{}{
		"turns": []map[string]string{
			{"input": "Hello", "response": "Hi there", "time": "2024-01-01T00:00:00Z"},
		},
		"overall": "A conversation",
	}
	data, _ := json.Marshal(summary)
	_ = os.WriteFile(filepath.Join(convDir, "summary.json"), data, 0644)

	m := NewManager(dir)
	if len(m.list.Topics) != 1 {
		t.Fatalf("expected 1 migrated topic, got %d", len(m.list.Topics))
	}
	if m.list.Topics[0].Title != "Hello" {
		t.Fatalf("expected title 'Hello' from first input, got %q", m.list.Topics[0].Title)
	}

	turns := m.readTopicTurns(m.list.Current)
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Input != "Hello" {
		t.Fatalf("expected turn input 'Hello', got %q", turns[0].Input)
	}

	if _, err := os.Stat(filepath.Join(convDir, "summary.json.migrated")); os.IsNotExist(err) {
		t.Fatal("expected summary.json to be renamed to summary.json.migrated")
	}
}

func TestCurrentTopic(t *testing.T) {
	m := NewManager(t.TempDir())
	ct := m.CurrentTopic()
	if ct == nil {
		t.Fatal("CurrentTopic returned nil")
	}
	if ct.ID != m.list.Current {
		t.Fatalf("expected current ID %q, got %q", m.list.Current, ct.ID)
	}
}

func TestCurrentTopicConcurrentSafety(t *testing.T) {
	m := NewManager(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct := m.CurrentTopic()
			if ct == nil {
				t.Error("CurrentTopic returned nil in goroutine")
			}
		}()
	}
	wg.Wait()
}

func TestCreateTopic(t *testing.T) {
	m := NewManager(t.TempDir())
	originalID := m.list.Current

	topic := m.CreateTopic("Test Topic")
	if topic == nil {
		t.Fatal("CreateTopic returned nil")
	}
	if topic.Title != "Test Topic" {
		t.Fatalf("expected title 'Test Topic', got %q", topic.Title)
	}
	if topic.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if topic.CreatedAt == "" {
		t.Fatal("expected non-empty CreatedAt")
	}
	if topic.UpdatedAt == "" {
		t.Fatal("expected non-empty UpdatedAt")
	}

	if m.list.Current != topic.ID {
		t.Fatalf("expected current to be new topic ID %q, got %q", topic.ID, m.list.Current)
	}
	if m.list.Previos != originalID {
		t.Fatalf("expected previous to be %q, got %q", originalID, m.list.Previos)
	}
	if len(m.list.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(m.list.Topics))
	}
}

func TestCreateMultipleTopics(t *testing.T) {
	m := NewManager(t.TempDir())
	topics := make([]*Topic, 5)
	for i := 0; i < 5; i++ {
		topics[i] = m.CreateTopic("Topic")
	}
	if len(m.list.Topics) != 6 {
		t.Fatalf("expected 6 topics (1 default + 5 created), got %d", len(m.list.Topics))
	}
}

func TestAppendTurn(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	err := m.AppendTurn(id, "user input", "assistant response")
	if err != nil {
		t.Fatalf("AppendTurn failed: %v", err)
	}

	td := m.loadTopicData(id)
	if len(td.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(td.Turns))
	}
	if td.Turns[0].Input != "user input" {
		t.Fatalf("expected input 'user input', got %q", td.Turns[0].Input)
	}
	if td.Turns[0].Response != "assistant response" {
		t.Fatalf("expected response 'assistant response', got %q", td.Turns[0].Response)
	}
	if td.Turns[0].Time == "" {
		t.Fatal("expected non-empty Time")
	}
}

func TestAppendTurnAutoTitle(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	_ = m.AppendTurn(id, "This is the first user input that should become the title", "response")

	if m.list.Topics[0].Title != "This is the first user input that should become the title" {
		t.Fatalf("expected auto-title, got %q", m.list.Topics[0].Title)
	}

	_ = m.AppendTurn(id, "second input", "second response")
	if len(m.loadTopicData(id).Turns) != 2 {
		t.Fatalf("expected 2 turns after second append")
	}
}

func TestAppendTurnDoesNotOverrideCustomTitle(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current
	m.UpdateTitle(id, "Custom Title")

	_ = m.AppendTurn(id, "first input", "response")
	if m.list.Topics[0].Title != "Custom Title" {
		t.Fatalf("expected title to remain 'Custom Title', got %q", m.list.Topics[0].Title)
	}
}

func TestAppendTurnUpdatesTimestamp(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current
	original, _ := time.Parse(time.RFC3339, m.list.Topics[0].UpdatedAt)

	_ = m.AppendTurn(id, "input", "response")
	updated, err := time.Parse(time.RFC3339, m.list.Topics[0].UpdatedAt)
	if err != nil {
		t.Fatalf("failed to parse updated timestamp: %v", err)
	}
	if updated.Before(original) {
		t.Fatal("expected UpdatedAt to not be before the original timestamp")
	}
}

func TestListTopics(t *testing.T) {
	m := NewManager(t.TempDir())
	m.CreateTopic("A")
	m.CreateTopic("B")

	topics := m.ListTopics()
	if len(topics) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(topics))
	}

	topics[0].Title = "Mutated"
	if m.list.Topics[0].Title == "Mutated" {
		t.Fatal("ListTopics returned a slice that shares backing array with internal state")
	}
}

func TestListTopicsEmpty(t *testing.T) {
	m := &Manager{
		dataDir: t.TempDir(),
		list:    &TopicList{},
	}
	topics := m.ListTopics()
	if len(topics) != 0 {
		t.Fatalf("expected 0 topics, got %d", len(topics))
	}
}

func TestSwitchToTopic(t *testing.T) {
	m := NewManager(t.TempDir())
	originalID := m.list.Current

	t2 := m.CreateTopic("Second")
	ok := m.SwitchToTopic(originalID)
	if !ok {
		t.Fatal("SwitchToTopic returned false for valid topic")
	}
	if m.list.Current != originalID {
		t.Fatalf("expected current %q, got %q", originalID, m.list.Current)
	}
	if m.list.Previos != t2.ID {
		t.Fatalf("expected previous %q, got %q", t2.ID, m.list.Previos)
	}
}

func TestSwitchToTopicNonExistent(t *testing.T) {
	m := NewManager(t.TempDir())
	ok := m.SwitchToTopic("nonexistent")
	if ok {
		t.Fatal("SwitchToTopic should return false for non-existent topic")
	}
}

func TestSwitchToTopicSameTopic(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current
	m.CreateTopic("Second")
	ok := m.SwitchToTopic(id)
	if !ok {
		t.Fatal("SwitchToTopic should return true for valid topic")
	}
	prev := m.list.Previos
	m.SwitchToTopic(id)
	if m.list.Previos != prev {
		t.Fatal("Previos should not change when switching to already-current topic")
	}
}

func TestFormatForPromptNoTurns(t *testing.T) {
	m := NewManager(t.TempDir())
	result := m.FormatForPrompt()
	if result != "No prior conversation." {
		t.Fatalf("expected 'No prior conversation.', got %q", result)
	}
}

func TestFormatForPromptWithTurns(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	turns := []struct {
		input, response string
	}{
		{"Hello", "Hi"},
		{"How are you?", "I'm good"},
		{"What's up?", "Not much"},
		{"Tell me a joke", "Why did the chicken cross the road?"},
		{"Another one", "To get to the other side"},
	}

	for _, turn := range turns {
		_ = m.AppendTurn(id, turn.input, turn.response)
	}

	result := m.FormatForPrompt()
	for _, turn := range turns {
		if !strings.Contains(result, turn.input) {
			t.Fatalf("expected result to contain %q", turn.input)
		}
		if !strings.Contains(result, turn.response) {
			t.Fatalf("expected result to contain %q", turn.response)
		}
	}
}

func TestFormatForPromptOnlyLastFive(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	for i := 1; i <= 10; i++ {
		_ = m.AppendTurn(id, fmt.Sprintf("Input %d", i), fmt.Sprintf("Response %d", i))
	}

	result := m.FormatForPrompt()
	if strings.Contains(result, "[USER] Input 1\n") {
		t.Fatal("expected first 5 turns to be excluded from last-5 truncation")
	}
	if !strings.Contains(result, "[USER] Input 6\n") {
		t.Fatal("expected 6th turn to be included")
	}
	if !strings.Contains(result, "[USER] Input 10\n") {
		t.Fatal("expected 10th turn to be included")
	}
}

func TestFormatForPromptSkipsOlderTurns(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	_ = m.AppendTurn(id, "first", "resp1")
	_ = m.AppendTurn(id, "second", "resp2")
	_ = m.AppendTurn(id, "third", "resp3")

	result := m.FormatForPrompt()
	if !strings.Contains(result, "first") {
		t.Fatal("with <= 5 turns, all should be included")
	}
}

func TestExtractSearchTerms(t *testing.T) {
	tests := []struct {
		name   string
		params json.RawMessage
		want   []string
	}{
		{
			name:   "nil params",
			params: nil,
			want:   nil,
		},
		{
			name:   "empty object",
			params: json.RawMessage(`{}`),
			want:   nil,
		},
		{
			name:   "single term",
			params: json.RawMessage(`{"search_terms": ["hello"]}`),
			want:   []string{"hello"},
		},
		{
			name:   "multiple terms",
			params: json.RawMessage(`{"search_terms": ["hello", "world", "test"]}`),
			want:   []string{"hello", "world", "test"},
		},
		{
			name:   "invalid JSON",
			params: json.RawMessage(`{invalid}`),
			want:   nil,
		},
		{
			name:   "search_terms not array",
			params: json.RawMessage(`{"search_terms": "not array"}`),
			want:   nil,
		},
		{
			name:   "additional fields ignored",
			params: json.RawMessage(`{"search_terms": ["term"], "other": "value"}`),
			want:   []string{"term"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSearchTerms(tt.params)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("expected %v, got %v", tt.want, got)
				}
			}
		})
	}
}

func TestUpdateTitle(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	m.UpdateTitle(id, "Updated Title")
	if m.list.Topics[0].Title != "Updated Title" {
		t.Fatalf("expected title 'Updated Title', got %q", m.list.Topics[0].Title)
	}

	td := m.loadTopicData(id)
	if td.Title != "Updated Title" {
		t.Fatalf("expected topic data title 'Updated Title', got %q", td.Title)
	}
}

func TestUpdateTitleNonExistent(t *testing.T) {
	m := NewManager(t.TempDir())
	m.UpdateTitle("nonexistent", "Title")
}

func TestUpdateTitleUpdatesTimestamp(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current
	original, _ := time.Parse(time.RFC3339, m.list.Topics[0].UpdatedAt)

	m.UpdateTitle(id, "New Title")
	updated, err := time.Parse(time.RFC3339, m.list.Topics[0].UpdatedAt)
	if err != nil {
		t.Fatalf("failed to parse updated timestamp: %v", err)
	}
	if updated.Before(original) {
		t.Fatal("expected UpdatedAt to not be before the original timestamp")
	}
}

func TestForkTopic(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	_ = m.AppendTurn(id, "input1", "resp1")
	_ = m.AppendTurn(id, "input2", "resp2")
	_ = m.AppendTurn(id, "input3", "resp3")

	originalCurrent := m.list.Current
	forked := m.ForkTopic("Forked Topic")
	if forked == nil {
		t.Fatal("ForkTopic returned nil")
	}

	if m.list.Current != forked.ID {
		t.Fatalf("expected current to be forked ID %q, got %q", forked.ID, m.list.Current)
	}
	if m.list.Previos != originalCurrent {
		t.Fatalf("expected previous to be %q, got %q", originalCurrent, m.list.Previos)
	}

	forkTurns := m.loadTopicData(forked.ID).Turns
	if len(forkTurns) != 3 {
		t.Fatalf("expected forked topic to have 3 turns, got %d", len(forkTurns))
	}
	if forkTurns[0].Input != "input1" {
		t.Fatalf("expected first turn input 'input1', got %q", forkTurns[0].Input)
	}
	if forkTurns[2].Input != "input3" {
		t.Fatalf("expected third turn input 'input3', got %q", forkTurns[2].Input)
	}
}

func TestForkTopicNoTurns(t *testing.T) {
	m := NewManager(t.TempDir())
	forked := m.ForkTopic("Empty Fork")
	if forked == nil {
		t.Fatal("ForkTopic returned nil")
	}
	forkTurns := m.loadTopicData(forked.ID).Turns
	if len(forkTurns) != 0 {
		t.Fatalf("expected 0 turns in forked topic, got %d", len(forkTurns))
	}
}

func TestResumeTopic(t *testing.T) {
	m := NewManager(t.TempDir())

	m.CreateTopic("Second")
	m.CreateTopic("Third")

	prev := m.list.Previos
	resumed := m.ResumeTopic()
	if resumed == nil {
		t.Fatal("ResumeTopic returned nil")
	}
	if resumed.ID != prev {
		t.Fatalf("expected resumed topic ID %q, got %q", prev, resumed.ID)
	}
	if m.list.Current != prev {
		t.Fatalf("expected current to be %q, got %q", prev, m.list.Current)
	}
	if m.list.Previos != m.list.Topics[2].ID {
		t.Fatalf("expected previous to swap to %q, got %q", m.list.Topics[2].ID, m.list.Previos)
	}
}

func TestResumeTopicNoPrevious(t *testing.T) {
	m := NewManager(t.TempDir())
	resumed := m.ResumeTopic()
	if resumed != nil {
		t.Fatal("ResumeTopic should return nil when there is no previous topic")
	}
}

func TestResumeTopicSwapsBackAndForth(t *testing.T) {
	m := NewManager(t.TempDir())
	firstID := m.list.Current

	m.CreateTopic("Second")
	secondID := m.list.Current

	r1 := m.ResumeTopic()
	if r1 == nil || r1.ID != firstID {
		t.Fatalf("expected resume to first, got %v", r1)
	}

	r2 := m.ResumeTopic()
	if r2 == nil || r2.ID != secondID {
		t.Fatalf("expected resume back to second, got %v", r2)
	}
}

func TestLoadTopicTurns(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.list.Current

	_ = m.AppendTurn(id, "a", "1")
	_ = m.AppendTurn(id, "b", "2")

	turns := m.LoadTopicTurns(id)
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].Input != "a" {
		t.Fatalf("expected first input 'a', got %q", turns[0].Input)
	}
}

func TestLoadTopicTurnsNonExistent(t *testing.T) {
	m := NewManager(t.TempDir())
	turns := m.LoadTopicTurns("nonexistent")
	if turns != nil {
		t.Fatal("expected nil turns for non-existent topic")
	}
}

func TestSearchPastDataNoTerms(t *testing.T) {
	m := NewManager(t.TempDir())
	result, err := m.SearchPastData(nil)
	if err != nil {
		t.Fatalf("SearchPastData error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty result for no terms, got %q", result)
	}

	result, err = m.SearchPastData([]string{})
	if err != nil {
		t.Fatalf("SearchPastData error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty result for empty terms, got %q", result)
	}
}

func TestSearchPastDataWithTerms(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not found on PATH")
	}

	m := NewManager(t.TempDir())
	m.CreateTopic("Test")
	id := m.list.Current
	_ = m.AppendTurn(id, "hello world", "response about AI")
	_ = m.AppendTurn(id, "goodbye", "response about ML")

	result, err := m.SearchPastData([]string{"hello"})
	if err != nil {
		t.Fatalf("SearchPastData error: %v", err)
	}
	_ = result
}

func TestSearchPastDataNoMatches(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not found on PATH")
	}

	m := NewManager(t.TempDir())
	_, err := m.SearchPastData([]string{"zzzznotfound"})
	if err != nil {
		t.Fatalf("SearchPastData error: %v", err)
	}
}

func TestSearchPastDataRgNotAvailable(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()
	_ = os.Setenv("PATH", "/dev/null")

	m := NewManager(t.TempDir())
	result, err := m.SearchPastData([]string{"hello"})
	if err != nil {
		t.Fatalf("expected nil error even when rg is missing, got %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty result when rg not available, got %q", result)
	}
}

func TestManagerConcurrency(t *testing.T) {
	m := NewManager(t.TempDir())

	var wg sync.WaitGroup
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			topic := m.CreateTopic("Topic")
			_ = m.AppendTurn(topic.ID, "input", "response")
			m.SwitchToTopic(topic.ID)
			m.CurrentTopic()
			m.ListTopics()
			m.FormatForPrompt()
		}(i)
	}
	wg.Wait()

	if len(m.list.Topics) != n+1 {
		t.Fatalf("expected %d topics, got %d", n+1, len(m.list.Topics))
	}
}

func TestTopicPersistenceAcrossManagers(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	m1.CreateTopic("Persistent")
	t1 := m1.CreateTopic("Third")
	_ = m1.AppendTurn(t1.ID, "persist me", "saved")

	m2 := NewManager(dir)
	if len(m2.list.Topics) != 3 {
		t.Fatalf("expected 3 topics after reload, got %d", len(m2.list.Topics))
	}

	turns := m2.LoadTopicTurns(t1.ID)
	if len(turns) != 1 || turns[0].Input != "persist me" {
		t.Fatalf("expected turn 'persist me' after reload, got %+v", turns)
	}
}

func TestCurrentTopicAfterSwitch(t *testing.T) {
	m := NewManager(t.TempDir())
	t1 := m.CreateTopic("First")

	ct := m.CurrentTopic()
	if ct.ID != t1.ID {
		t.Fatalf("expected current %q, got %q", t1.ID, ct.ID)
	}
}

func TestManagerPersistenceOfTopicsJSON(t *testing.T) {
	dir := t.TempDir()
	_ = NewManager(dir)

	path := filepath.Join(dir, "topics", "topics.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("topics.json was not created")
	}
}
