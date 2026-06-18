package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardrailPrefix(t *testing.T) {
	if !strings.Contains(GuardrailPrefix, "SECURITY GUARDRAILS") {
		t.Errorf("GuardrailPrefix should contain 'SECURITY GUARDRAILS', got: %s", GuardrailPrefix)
	}
	if !strings.Contains(GuardrailPrefix, "NEVER IGNORE") {
		t.Errorf("GuardrailPrefix should contain 'NEVER IGNORE'")
	}
}

func TestPersona(t *testing.T) {
	if !strings.Contains(Persona, "DHI Harness") {
		t.Errorf("Persona should contain 'DHI Harness', got: %s", Persona)
	}
	if !strings.Contains(Persona, "PERSONA:") {
		t.Errorf("Persona should contain 'PERSONA:'")
	}
}

func TestDefaultPipelines(t *testing.T) {
	pips := defaultPipelines()
	if len(pips) != 3 {
		t.Fatalf("expected 3 default pipelines, got %d", len(pips))
	}

	expected := []struct {
		name string
		desc string
	}{
		{"TODO", "Create and manage todos/reminders"},
		{"SYSTEM", "System metrics and observability"},
		{"SCHEDULE", "Schedule tasks for later execution"},
	}

	for i, exp := range expected {
		t.Run(exp.name, func(t *testing.T) {
			if pips[i].Name != exp.name {
				t.Errorf("pipeline[%d].Name = %q, want %q", i, pips[i].Name, exp.name)
			}
			if !strings.Contains(pips[i].Description, exp.desc) {
				t.Errorf("pipeline[%d].Description = %q, want it to contain %q", i, pips[i].Description, exp.desc)
			}
		})
	}
}

func TestLoadPipelineInfos_FileNotExist(t *testing.T) {
	pips := LoadPipelineInfos("/nonexistent/path/pipelines.yaml")
	if len(pips) != 3 {
		t.Fatalf("expected 3 default pipelines when file doesn't exist, got %d", len(pips))
	}
	names := []string{pips[0].Name, pips[1].Name, pips[2].Name}
	expected := "TODO SYSTEM SCHEDULE"
	got := strings.Join(names, " ")
	if got != expected {
		t.Errorf("expected pipelines %q, got %q", expected, got)
	}
}

func TestLoadPipelineInfos_RealYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "pipelines.yaml")

	yamlContent := `pipelines:
  - name: stocks
    description: Look up stock prices and financial data
    weight: 0.7
    keywords: [stock, price, market, share, nifty, sensex]
    params:
      - name: symbol
        required: true
        extract_hint: Stock ticker symbol (e.g., AAPL, RELIANCE)
  - name: web_search
    description: Search the web for information
    weight: 0.6
    params: []
`

	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	pips := LoadPipelineInfos(yamlPath)
	// Should have 2 from file + 3 defaults = 5
	if len(pips) != 5 {
		t.Fatalf("expected 5 pipelines (2 from file + 3 defaults), got %d", len(pips))
	}

	t.Run("stocks_from_yaml", func(t *testing.T) {
		if pips[0].Name != "STOCKS" {
			t.Errorf("expected STOCKS (uppercased), got %q", pips[0].Name)
		}
		if pips[0].Weight != 0.7 {
			t.Errorf("expected weight 0.7, got %f", pips[0].Weight)
		}
		if len(pips[0].Keywords) != 6 {
			t.Errorf("expected 5 keywords, got %d", len(pips[0].Keywords))
		}
		if len(pips[0].Params) != 1 {
			t.Fatalf("expected 1 param, got %d", len(pips[0].Params))
		}
		if pips[0].Params[0].Name != "symbol" {
			t.Errorf("expected param name 'symbol', got %q", pips[0].Params[0].Name)
		}
		if !pips[0].Params[0].Required {
			t.Error("expected symbol param to be required")
		}
	})

	t.Run("web_search_from_yaml", func(t *testing.T) {
		if pips[1].Name != "WEB_SEARCH" {
			t.Errorf("expected WEB_SEARCH (uppercased), got %q", pips[1].Name)
		}
		if len(pips[1].Params) != 0 {
			t.Errorf("expected 0 params, got %d", len(pips[1].Params))
		}
	})

	t.Run("defaults_appended", func(t *testing.T) {
		if pips[2].Name != "TODO" {
			t.Errorf("expected TODO as first default, got %q", pips[2].Name)
		}
		if pips[3].Name != "SYSTEM" {
			t.Errorf("expected SYSTEM as second default, got %q", pips[3].Name)
		}
		if pips[4].Name != "SCHEDULE" {
			t.Errorf("expected SCHEDULE as third default, got %q", pips[4].Name)
		}
	})
}

func TestBuildSchema_NotEmpty(t *testing.T) {
	result := BuildSchema(nil, "/nonexistent/path/pipelines.yaml")
	if result == "" {
		t.Fatal("BuildSchema returned empty string")
	}
	if !strings.Contains(result, "TODO") {
		t.Error("BuildSchema should contain TODO pipeline name")
	}
	if !strings.Contains(result, "SYSTEM") {
		t.Error("BuildSchema should contain SYSTEM pipeline name")
	}
	if !strings.Contains(result, "SCHEDULE") {
		t.Error("BuildSchema should contain SCHEDULE pipeline name")
	}
	if !strings.Contains(result, "EXECUTE_") {
		t.Error("BuildSchema should contain EXECUTE_ event prefix")
	}
}

func TestBuildSchema_ContainsPipelineNames(t *testing.T) {
	result := BuildSchema(nil, "/nonexistent/path/pipelines.yaml")

	for _, name := range []string{"TODO", "SYSTEM", "SCHEDULE"} {
		if !strings.Contains(result, name) {
			t.Errorf("BuildSchema output should contain pipeline name %q", name)
		}
		if !strings.Contains(result, "EXECUTE_"+name) {
			t.Errorf("BuildSchema output should contain EXECUTE_%s", name)
		}
	}
}

func TestBuildSchema_WithWeightLines(t *testing.T) {
	weightLines := map[string]string{
		"todo":     "priority: high",
		"schedule": "priority: medium",
	}
	result := BuildSchema(weightLines, "/nonexistent/path/pipelines.yaml")

	if !strings.Contains(result, "priority: high") {
		t.Error("BuildSchema should include weight line for 'todo'")
	}
	if !strings.Contains(result, "priority: medium") {
		t.Error("BuildSchema should include weight line for 'schedule'")
	}
	// SYSTEM doesn't have a custom weight line, so it should use the default weight format
	if !strings.Contains(result, "weight: 0.6") {
		t.Error("BuildSchema should include default weight for SYSTEM (0.6)")
	}
}

func TestClassifyPrompt_ContainsClassifierAndConversation(t *testing.T) {
	result := ClassifyPrompt("hello", "", "", "", "", nil, "/nonexistent/path/pipelines.yaml")
	if result == "" {
		t.Fatal("ClassifyPrompt returned empty string")
	}
	if !strings.Contains(result, "CLASSIFIER") {
		t.Error("ClassifyPrompt should contain 'CLASSIFIER'")
	}
	if !strings.Contains(result, "CONVERSATION") {
		t.Error("ClassifyPrompt should contain 'CONVERSATION'")
	}
}

func TestClassifyPrompt_ContainsOutputOnly(t *testing.T) {
	result := ClassifyPrompt("hello", "", "", "", "", nil, "/nonexistent/path/pipelines.yaml")
	if !strings.Contains(result, "Output ONLY") {
		t.Error("ClassifyPrompt should contain 'Output ONLY'")
	}
	if !strings.Contains(result, "SECURITY GUARDRAILS") {
		t.Error("ClassifyPrompt should contain guardrails")
	}
	if !strings.Contains(result, "DHI Harness") {
		t.Error("ClassifyPrompt should contain persona")
	}
}

func TestClassifyPrompt_IncludesUserInput(t *testing.T) {
	input := "show me system stats"
	result := ClassifyPrompt(input, "", "", "", "", nil, "/nonexistent/path/pipelines.yaml")
	if !strings.Contains(result, input) {
		t.Errorf("ClassifyPrompt should include the user input %q", input)
	}
}

func TestClassifyPrompt_WithCalibrationAndKeywords(t *testing.T) {
	calBlock := "CALIBRATION: test calibration"
	kwBlock := "KEYWORDS: test, keywords"
	result := ClassifyPrompt("hello", "", "", calBlock, kwBlock, nil, "/nonexistent/path/pipelines.yaml")
	if !strings.Contains(result, calBlock) {
		t.Error("ClassifyPrompt should include calibration block")
	}
	if !strings.Contains(result, kwBlock) {
		t.Error("ClassifyPrompt should include keyword block")
	}
}

func TestConversationPrompt(t *testing.T) {
	result := ConversationPrompt("how are you?", "previous conversation", "", "greeting")
	if result == "" {
		t.Fatal("ConversationPrompt returned empty string")
	}
	if !strings.Contains(result, "SECURITY GUARDRAILS") {
		t.Error("ConversationPrompt should contain guardrails")
	}
	if !strings.Contains(result, "DHI Harness") {
		t.Error("ConversationPrompt should contain persona")
	}
	if !strings.Contains(result, "how are you?") {
		t.Error("ConversationPrompt should include user input")
	}
	if !strings.Contains(result, "previous conversation") {
		t.Error("ConversationPrompt should include summary text")
	}
}

func TestConversationPrompt_WithSearchResults(t *testing.T) {
	searchResults := "SEARCH_RESULTS: found relevant data"
	result := ConversationPrompt("what is ai?", "summary", searchResults, "ai topic")
	if !strings.Contains(result, searchResults) {
		t.Error("ConversationPrompt should include search results when provided")
	}
}

func TestSynthesisPrompt(t *testing.T) {
	result := SynthesisPrompt("{data: 42}", "stocks", "what is aapl price?", "previous chat")
	if result == "" {
		t.Fatal("SynthesisPrompt returned empty string")
	}
	if !strings.Contains(result, "SECURITY GUARDRAILS") {
		t.Error("SynthesisPrompt should contain guardrails")
	}
	if !strings.Contains(result, "DHI Harness") {
		t.Error("SynthesisPrompt should contain persona")
	}
	if !strings.Contains(result, "stocks") {
		t.Error("SynthesisPrompt should contain pipeline name")
	}
	if !strings.Contains(result, "{data: 42}") {
		t.Error("SynthesisPrompt should contain raw data")
	}
	if !strings.Contains(result, "what is aapl price?") {
		t.Error("SynthesisPrompt should contain user input")
	}
}

func TestSynthesisPrompt_AllPipelines(t *testing.T) {
	pipelines := []string{"TODO", "SYSTEM", "SCHEDULE", "STOCKS", "WEB_SEARCH"}
	for _, p := range pipelines {
		t.Run(p, func(t *testing.T) {
			result := SynthesisPrompt("data", p, "input", "")
			if !strings.Contains(result, p) {
				t.Errorf("SynthesisPrompt should reference pipeline %q, got:\n%s", p, result)
			}
		})
	}
}

func TestBuildSchema_WithRealYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "pipelines.yaml")

	yamlContent := `pipelines:
  - name: music
    description: Play music and manage playlists
    weight: 0.5
    keywords: [music, song, playlist, play, spotify]
    params:
      - name: action
        required: true
        default: play
        extract_hint: play, pause, next, previous
      - name: query
        required: false
        extract_hint: song or artist name
`

	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	result := BuildSchema(nil, yamlPath)
	if result == "" {
		t.Fatal("BuildSchema returned empty string")
	}
	if !strings.Contains(result, "MUSIC") {
		t.Error("BuildSchema should contain MUSIC pipeline from YAML")
	}
	if !strings.Contains(result, "EXECUTE_MUSIC") {
		t.Error("BuildSchema should contain EXECUTE_MUSIC event")
	}
	if !strings.Contains(result, "Play music and manage playlists") {
		t.Error("BuildSchema should contain the music pipeline description")
	}
	if !strings.Contains(result, "TODO") {
		t.Error("BuildSchema should still contain TODO default pipeline")
	}
}
