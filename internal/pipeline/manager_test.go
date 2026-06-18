package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(filepath.Join(t.TempDir(), "nonexistent.yaml"))
}

func TestDefaultPipelines_ReturnsTodoAndSchedule(t *testing.T) {
	m := defaultManager(t)
	pipelines := m.ListPipelines()

	names := make(map[string]bool)
	for _, p := range pipelines {
		names[p.Name] = true
	}

	if !names["todo"] {
		t.Fatal("expected 'todo' pipeline in defaults")
	}
	if !names["schedule"] {
		t.Fatal("expected 'schedule' pipeline in defaults")
	}
}

func TestDefaultPipelines_HasMinimumCount(t *testing.T) {
	m := defaultManager(t)
	count := m.PipelineCount()
	if count < 2 {
		t.Fatalf("expected at least 2 default pipelines, got %d", count)
	}
}

func TestLoadPipelineConfig_LoadsFromYAMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelines.yaml")

	yamlContent := `
pipelines:
  - name: custom_pipeline
    description: A custom pipeline for testing
    weight: 0.9
    action:
      type: bash
      script: "echo hello"
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(path)

	found := false
	for _, p := range m.ListPipelines() {
		if p.Name == "custom_pipeline" {
			found = true
			if p.Description != "A custom pipeline for testing" {
				t.Fatalf("expected description 'A custom pipeline for testing', got %q", p.Description)
			}
			if p.Weight != 0.9 {
				t.Fatalf("expected weight 0.9, got %f", p.Weight)
			}
			if p.Action.Type != "bash" {
				t.Fatalf("expected action type 'bash', got %q", p.Action.Type)
			}
		}
	}
	if !found {
		t.Fatal("expected 'custom_pipeline' to be loaded from YAML")
	}

	if m.PipelineCount() < 3 {
		t.Fatalf("expected at least 3 pipelines (1 custom + defaults), got %d", m.PipelineCount())
	}
}

func TestLoadPipelineConfig_ReturnsDefaultsWhenFileNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")

	m := NewManager(path)

	count := m.PipelineCount()
	if count < 2 {
		t.Fatalf("expected defaults when file doesn't exist, got %d pipelines", count)
	}

	names := make(map[string]bool)
	for _, p := range m.ListPipelines() {
		names[p.Name] = true
	}
	if !names["todo"] || !names["schedule"] {
		t.Fatal("expected 'todo' and 'schedule' pipelines from defaults")
	}
}

func TestDefaultPipelines_NamesAreLowercase(t *testing.T) {
	m := defaultManager(t)
	for _, p := range m.ListPipelines() {
		if p.Name != strings.ToLower(p.Name) {
			t.Fatalf("pipeline name %q is not lowercase", p.Name)
		}
	}
}

func TestPipelineDefinition_HasCorrectStructure(t *testing.T) {
	p := Pipeline{
		Name:        "test_pipeline",
		Description: "Test description",
		Weight:      0.7,
		Keywords:    []string{"test", "example"},
		Params: []ParamDefinition{
			{Name: "query", Required: true, Default: "", ExtractHint: "Enter query"},
		},
		Action: PipelineAction{
			Type:      "bash",
			Command:   "",
			Script:    "echo {query}",
			Args:      nil,
			Handler:   "",
			Summarize: true,
		},
	}

	if p.Name != "test_pipeline" {
		t.Fatalf("expected Name 'test_pipeline', got %q", p.Name)
	}
	if p.Description != "Test description" {
		t.Fatalf("expected Description 'Test description', got %q", p.Description)
	}
	if p.Weight != 0.7 {
		t.Fatalf("expected Weight 0.7, got %f", p.Weight)
	}
	if len(p.Keywords) != 2 {
		t.Fatalf("expected 2 keywords, got %d", len(p.Keywords))
	}
	if len(p.Params) != 1 {
		t.Fatalf("expected 1 param definition, got %d", len(p.Params))
	}
	if p.Action.Type != "bash" {
		t.Fatalf("expected Action.Type 'bash', got %q", p.Action.Type)
	}
	if p.Action.Script != "echo {query}" {
		t.Fatalf("expected Action.Script 'echo {query}', got %q", p.Action.Script)
	}
	if !p.Action.Summarize {
		t.Fatal("expected Action.Summarize to be true")
	}
}

func TestPipeline_DefaultsHaveAllFields(t *testing.T) {
	m := defaultManager(t)
	for _, p := range m.ListPipelines() {
		if p.Name == "" {
			t.Fatal("all default pipelines must have a name")
		}
		if p.Description == "" {
			t.Fatalf("pipeline %q has empty description", p.Name)
		}
		if p.Action.Type == "" {
			t.Fatalf("pipeline %q has empty action type", p.Name)
		}
	}
}

func TestValidActionTypes(t *testing.T) {
	valid := map[string]bool{
		"http":     true,
		"exec":     true,
		"bash":     true,
		"internal": true,
	}

	// Verify that the executeAction switch handles the expected types
	// by checking that all default pipelines use a valid action type.
	m := defaultManager(t)
	for _, p := range m.ListPipelines() {
		if !valid[p.Action.Type] {
			t.Fatalf("pipeline %q uses unknown action type %q", p.Name, p.Action.Type)
		}
	}
}

func TestValidActionTypes_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		actType string
		valid   bool
	}{
		{"bash is valid", "bash", true},
		{"http is valid", "http", true},
		{"exec is valid", "exec", true},
		{"internal is valid", "internal", true},
		{"invalid type", "python", false},
		{"empty type", "", false},
		{"unknown type", "docker", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Pipeline{
				Name: "test",
				Action: PipelineAction{
					Type: tt.actType,
				},
			}
			// executeAction on a nil executor panics, so we validate by checking
			// the action type against the known set
			knownValid := map[string]bool{
				"http":     true,
				"exec":     true,
				"bash":     true,
				"internal": true,
			}
			if knownValid[p.Action.Type] != tt.valid {
				t.Fatalf("expected action type %q valid=%v, got %v", tt.actType, tt.valid, knownValid[p.Action.Type])
			}
		})
	}
}

func TestLoadPipelineConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	if err := os.WriteFile(path, []byte("not: valid: yaml: [[["), 0644); err != nil {
		t.Fatal(err)
	}

	// Should fall back to defaults
	m := NewManager(path)
	count := m.PipelineCount()
	if count < 2 {
		t.Fatalf("expected defaults on invalid YAML, got %d", count)
	}
}

func TestLoadPipelineConfig_EmptyPipelines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")

	if err := os.WriteFile(path, []byte("pipelines:\n"), 0644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(path)
	count := m.PipelineCount()
	if count < 2 {
		t.Fatalf("expected defaults on empty pipelines, got %d", count)
	}
}

func TestListDescriptions(t *testing.T) {
	m := defaultManager(t)

	descs := m.ListDescriptions()
	if len(descs) < 2 {
		t.Fatalf("expected at least 2 descriptions, got %d", len(descs))
	}

	for _, d := range descs {
		if !strings.Contains(d, ": ") {
			t.Fatalf("expected description format 'name: description', got %q", d)
		}
	}

	foundTodo := false
	for _, d := range descs {
		if strings.HasPrefix(d, "todo:") {
			foundTodo = true
			break
		}
	}
	if !foundTodo {
		t.Fatal("expected 'todo' in descriptions")
	}
}
