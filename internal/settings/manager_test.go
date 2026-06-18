package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dhi_harness/internal/ai"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestCurrentModelReturnsNilWhenNoneSelected(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	if model := m.CurrentModel(); model != nil {
		t.Fatalf("expected nil, got %+v", model)
	}
}

func TestListModelsReturnsAvailableModels(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
			{Name: "llama3.2:1b", ContextLen: 131072},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	models := m.ListModels()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].Name != "qwen2.5:1.5b" {
		t.Fatalf("expected 'qwen2.5:1.5b', got %q", models[0].Name)
	}
	if models[1].Name != "llama3.2:1b" {
		t.Fatalf("expected 'llama3.2:1b', got %q", models[1].Name)
	}
}

func TestListModelsReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	models := m.ListModels()
	models[0].Name = "modified"

	original := m.ListModels()
	if original[0].Name != "qwen2.5:1.5b" {
		t.Fatal("ListModels should return a copy")
	}
}

func TestListModelsReturnsEmptyWhenNoneConfigured(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	models := m.ListModels()
	if len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}
}

func TestSelectModel(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
			{Name: "llama3.2:1b", ContextLen: 131072},
		},
	})

	m := NewManager(dir, nil, ai.Config{})

	err := m.SelectModel("llama3.2:1b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	model := m.CurrentModel()
	if model == nil {
		t.Fatal("expected non-nil current model")
	}
	if model.Name != "llama3.2:1b" {
		t.Fatalf("expected 'llama3.2:1b', got %q", model.Name)
	}
	if model.ContextLen != 131072 {
		t.Fatalf("expected context length 131072, got %d", model.ContextLen)
	}
}

func TestSelectModelErrorsOnUnknown(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	err := m.SelectModel("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestContextLengthReturnsDefaultWhenNoModel(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	if cl := m.ContextLength(); cl != 4096 {
		t.Fatalf("expected 4096, got %d", cl)
	}
}

func TestContextLengthReturnsDefaultWhenModelHasNoContextLen(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 0},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	_ = m.SelectModel("qwen2.5:1.5b")
	if cl := m.ContextLength(); cl != 4096 {
		t.Fatalf("expected 4096 for model with zero context length, got %d", cl)
	}
}

func TestContextLengthReturnsCorrectLengthForKnownModels(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	_ = m.SelectModel("qwen2.5:1.5b")
	if cl := m.ContextLength(); cl != 32768 {
		t.Fatalf("expected 32768, got %d", cl)
	}
}

func TestContextLengthMultipleKnownModels(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "llama3.2:1b", ContextLen: 131072},
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
			{Name: "mistral:7b", ContextLen: 32768},
			{Name: "deepseek-r1:7b", ContextLen: 16384},
			{Name: "gemma2:27b", ContextLen: 8192},
			{Name: "nomic-embed-text", ContextLen: 8192},
		},
	})

	m := NewManager(dir, nil, ai.Config{})

	tests := []struct {
		model string
		want  int
	}{
		{"llama3.2:1b", 131072},
		{"qwen2.5:1.5b", 32768},
		{"mistral:7b", 32768},
		{"deepseek-r1:7b", 16384},
		{"gemma2:27b", 8192},
		{"nomic-embed-text", 8192},
	}

	for _, tt := range tests {
		_ = m.SelectModel(tt.model)
		if got := m.ContextLength(); got != tt.want {
			t.Errorf("%s: expected context length %d, got %d", tt.model, tt.want, got)
		}
	}
}

func TestManagerRejectsModelNotInList(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	err := m.SelectModel("llama3.2:1b")
	if err == nil {
		t.Fatal("expected error when selecting model not in list")
	}
}

func TestManagerHasDefaultContextLengthForUnknownModels(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "custom-model", ContextLen: 0},
		},
	})

	m := NewManager(dir, nil, ai.Config{})
	_ = m.SelectModel("custom-model")
	if cl := m.ContextLength(); cl != 4096 {
		t.Fatalf("expected default 4096 for unknown model, got %d", cl)
	}
}

func TestSettingsPersistence(t *testing.T) {
	dir := t.TempDir()

	NewManager(dir, nil, ai.Config{})

	writeSettings(t, dir, Settings{
		SelectedModel: "qwen2.5:1.5b",
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
			{Name: "llama3.2:1b", ContextLen: 131072},
		},
	})

	// Reload
	m2 := NewManager(dir, nil, ai.Config{})

	model := m2.CurrentModel()
	if model == nil {
		t.Fatal("expected non-nil model after reload")
	}
	if model.Name != "qwen2.5:1.5b" {
		t.Fatalf("expected 'qwen2.5:1.5b', got %q", model.Name)
	}
	if model.ContextLen != 32768 {
		t.Fatalf("expected context length 32768, got %d", model.ContextLen)
	}

	models := m2.ListModels()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}

func TestSelectModelPersists(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, Settings{
		Models: []ModelInfo{
			{Name: "qwen2.5:1.5b", ContextLen: 32768},
			{Name: "llama3.2:1b", ContextLen: 131072},
		},
	})

	m1 := NewManager(dir, nil, ai.Config{})
	_ = m1.SelectModel("llama3.2:1b")

	// Reload
	m2 := NewManager(dir, nil, ai.Config{})

	model := m2.CurrentModel()
	if model == nil {
		t.Fatal("expected non-nil current model after persistence")
	}
	if model.Name != "llama3.2:1b" {
		t.Fatalf("expected 'llama3.2:1b' after persistence, got %q", model.Name)
	}
}

func TestNewManagerCreatesSettingsDir(t *testing.T) {
	dir := t.TempDir()
	NewManager(dir, nil, ai.Config{})

	settingsDir := filepath.Join(dir, "settings")
	info, err := os.Stat(settingsDir)
	if err != nil {
		t.Fatalf("expected settings dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected settings path to be a directory")
	}
}

func writeSettings(t *testing.T, dir string, s Settings) {
	t.Helper()
	settingsDir := filepath.Join(dir, "settings")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("failed to create settings dir: %v", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644); err != nil {
		t.Fatalf("failed to write settings: %v", err)
	}
}

func TestFetchModels_ReturnsErrorWhenClientNotConnected(t *testing.T) {
	client := ai.NewClient(ai.Config{
		BaseURL: "http://127.0.0.1:1",
	})
	m := NewManager(t.TempDir(), client, ai.Config{})
	err := m.FetchModels()
	if err == nil {
		t.Fatal("expected error when client is not connected")
	}
}

func TestLookupContextLengthKnown(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})

	tests := []struct {
		model string
		want  int
	}{
		{"qwen2.5:7b", 32768},
		{"llama3.2:1b", 131072},
		{"deepseek:7b", 4096},
		{"nomic-embed-text", 8192},
		{"mxbai-embed-large", 512},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := m.lookupContextLength(tt.model)
			if got != tt.want {
				t.Errorf("lookupContextLength(%q) = %d, want %d", tt.model, got, tt.want)
			}
		})
	}
}

func TestLookupContextLengthUnknown(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	got := m.lookupContextLength("completely-unknown-model:99b")
	if got != 4096 {
		t.Errorf("expected 4096 for unknown model, got %d", got)
	}
}

func TestLookupContextLengthPrefixMatch(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})

	got := m.lookupContextLength("qwen2.5:7b-q4_K_M")
	if got != 32768 {
		t.Errorf("expected 32768 (prefix match qwen2.5:7b), got %d", got)
	}

	got = m.lookupContextLength("llama3.2:1b-Q4_K_M")
	if got != 131072 {
		t.Errorf("expected 131072 (prefix match llama3.2:1b), got %d", got)
	}
}

func TestResolveSelection_PicksFirstModelWhenNoneSelected(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	m.settings.Models = []ModelInfo{
		{Name: "qwen2.5:1.5b", ContextLen: 32768},
		{Name: "llama3.2:1b", ContextLen: 131072},
	}
	m.resolveSelection()

	if m.settings.SelectedModel != "qwen2.5:1.5b" {
		t.Fatalf("expected first model to be selected, got %q", m.settings.SelectedModel)
	}
}

func TestResolveSelection_UsesConfigModel(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{Model: "llama3.2:1b"})
	m.settings.Models = []ModelInfo{
		{Name: "qwen2.5:1.5b", ContextLen: 32768},
		{Name: "llama3.2:1b", ContextLen: 131072},
	}
	m.resolveSelection()

	if m.settings.SelectedModel != "llama3.2:1b" {
		t.Fatalf("expected config model to be selected, got %q", m.settings.SelectedModel)
	}
}

func TestResolveSelection_KeepsExistingSelection(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{})
	m.settings.Models = []ModelInfo{
		{Name: "qwen2.5:1.5b", ContextLen: 32768},
		{Name: "llama3.2:1b", ContextLen: 131072},
	}
	m.settings.SelectedModel = "llama3.2:1b"
	m.resolveSelection()

	if m.settings.SelectedModel != "llama3.2:1b" {
		t.Fatalf("expected existing selection to be kept, got %q", m.settings.SelectedModel)
	}
}

func TestResolveSelection_ClearsInvalidSelection(t *testing.T) {
	m := NewManager(t.TempDir(), nil, ai.Config{Model: "qwen2.5:1.5b"})
	m.settings.Models = []ModelInfo{
		{Name: "qwen2.5:1.5b", ContextLen: 32768},
	}
	m.settings.SelectedModel = "nonexistent"
	m.resolveSelection()

	if m.settings.SelectedModel != "qwen2.5:1.5b" {
		t.Fatalf("expected config model after clearing invalid, got %q", m.settings.SelectedModel)
	}
}
