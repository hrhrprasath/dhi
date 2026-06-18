package intent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dhi_harness/internal/ai"
	"dhi_harness/internal/services"
)

func TestParseJSON_ValidJSONReturnsParsedIntent(t *testing.T) {
	raw := `{"event": "EXECUTE_TODO", "description": "test intent", "confidence": 0.9}`
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Event != "EXECUTE_TODO" {
		t.Fatalf("expected event 'EXECUTE_TODO', got %q", parsed.Event)
	}
	if parsed.Description != "test intent" {
		t.Fatalf("expected description 'test intent', got %q", parsed.Description)
	}
}

func TestParseJSON_JSONInCodeFences(t *testing.T) {
	raw := "```json\n{\"event\": \"CONVERSATION\", \"confidence\": 0.7}\n```"
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Event != "CONVERSATION" {
		t.Fatalf("expected event 'CONVERSATION', got %q", parsed.Event)
	}
}

func TestParseJSON_JSONInMarkdownBlocks(t *testing.T) {
	raw := "Here is the result:\n\n```\n{\"event\": \"EXECUTE_WEB\", \"confidence\": 0.85}\n```\n"
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Event != "EXECUTE_WEB" {
		t.Fatalf("expected event 'EXECUTE_WEB', got %q", parsed.Event)
	}
}

func TestParseJSON_InvalidJSONReturnsError(t *testing.T) {
	raw := `{invalid json here}`
	_, err := parseJSON(raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseJSON_MissingEventFieldReturnsError(t *testing.T) {
	raw := `{"description": "no event field", "confidence": 0.5}`
	_, err := parseJSON(raw)
	if err == nil {
		t.Fatal("expected error for missing 'event' field")
	}
}

func TestParseJSON_EmptyEventFieldReturnsError(t *testing.T) {
	raw := `{"event": "", "confidence": 0.5}`
	_, err := parseJSON(raw)
	if err == nil {
		t.Fatal("expected error for empty 'event' field")
	}
}

func TestParseJSON_ZeroConfidenceSetsDefault(t *testing.T) {
	raw := `{"event": "EXECUTE_TODO", "confidence": 0}`
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Confidence != 0.5 {
		t.Fatalf("expected default confidence 0.5, got %f", parsed.Confidence)
	}
}

func TestParseJSON_ConfidenceZeroEightPreserved(t *testing.T) {
	raw := `{"event": "EXECUTE_TODO", "confidence": 0.8}`
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Confidence != 0.8 {
		t.Fatalf("expected confidence 0.8, got %f", parsed.Confidence)
	}
}

func TestParseJSON_PartialObjectInText(t *testing.T) {
	raw := `Some text before {"event": "EXECUTE_TODO", "confidence": 0.6} and text after`
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Event != "EXECUTE_TODO" {
		t.Fatalf("expected event 'EXECUTE_TODO', got %q", parsed.Event)
	}
	if parsed.Confidence != 0.6 {
		t.Fatalf("expected confidence 0.6, got %f", parsed.Confidence)
	}
}

func TestParseJSON_WithParamsField(t *testing.T) {
	raw := `{"event": "EXECUTE_TODO", "params": {"action": "add", "text": "buy milk"}, "confidence": 0.9}`
	parsed, err := parseJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Event != "EXECUTE_TODO" {
		t.Fatalf("expected event 'EXECUTE_TODO', got %q", parsed.Event)
	}

	var params map[string]string
	if err := json.Unmarshal(parsed.Params, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	if params["action"] != "add" {
		t.Fatalf("expected params.action 'add', got %q", params["action"])
	}
	if params["text"] != "buy milk" {
		t.Fatalf("expected params.text 'buy milk', got %q", params["text"])
	}
}

func TestParseJSON_EmptyStringReturnsError(t *testing.T) {
	_, err := parseJSON("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestParseJSON_WhitespaceOnlyReturnsError(t *testing.T) {
	_, err := parseJSON("   \n\t  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only string")
	}
}

func TestParseJSON_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantEvent     string
		wantConf      float64
		wantErr       bool
		checkParamsFn func(t *testing.T, parsed *ParsedIntent)
	}{
		{
			name:      "valid minimal JSON",
			input:     `{"event": "EXECUTE_TODO"}`,
			wantEvent: "EXECUTE_TODO",
			wantConf:  0.5,
		},
		{
			name:      "valid with confidence",
			input:     `{"event": "CONVERSATION", "confidence": 0.95}`,
			wantEvent: "CONVERSATION",
			wantConf:  0.95,
		},
		{
			name:    "no JSON object",
			input:   "just text without braces",
			wantErr: true,
		},
		{
			name:    "missing closing brace",
			input:   `{"event": "TEST"`,
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			input:   `{"event": "TEST", "confidence": abc}`,
			wantErr: true,
		},
		{
			name:    "multiple JSON objects returns error",
			input:   `{"event": "FIRST"} {"event": "SECOND"}`,
			wantErr: true,
		},
		{
			name:      "params preserved as raw JSON",
			input:     `{"event": "EXECUTE_TODO", "params": {"action": "list"}, "confidence": 1.0}`,
			wantEvent: "EXECUTE_TODO",
			wantConf:  1.0,
			checkParamsFn: func(t *testing.T, parsed *ParsedIntent) {
				var p map[string]string
				if err := json.Unmarshal(parsed.Params, &p); err != nil {
					t.Fatalf("params unmarshal error: %v", err)
				}
				if p["action"] != "list" {
					t.Fatalf("expected action 'list', got %q", p["action"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseJSON(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.Event != tt.wantEvent {
				t.Fatalf("expected event %q, got %q", tt.wantEvent, parsed.Event)
			}
			if parsed.Confidence != tt.wantConf {
				t.Fatalf("expected confidence %f, got %f", tt.wantConf, parsed.Confidence)
			}
			if tt.checkParamsFn != nil {
				tt.checkParamsFn(t, parsed)
			}
		})
	}
}

func TestParseJSON_NoBracesReturnsError(t *testing.T) {
	_, err := parseJSON("plain text without any braces")
	if err == nil {
		t.Fatal("expected error for text without braces")
	}
}

func TestParseJSON_OpenBraceOnlyReturnsError(t *testing.T) {
	_, err := parseJSON("{")
	if err == nil {
		t.Fatal("expected error for single open brace")
	}
}

func TestParseJSON_CloseBraceOnlyReturnsError(t *testing.T) {
	_, err := parseJSON("}")
	if err == nil {
		t.Fatal("expected error for single close brace")
	}
}

func TestParseJSON_ReversedBracesReturnsError(t *testing.T) {
	_, err := parseJSON("}{")
	if err == nil {
		t.Fatal("expected error for reversed braces")
	}
}

type testMessage struct {
	Content string `json:"content"`
}

type testChoice struct {
	Message testMessage `json:"message"`
}

type testChatResponse struct {
	Choices []testChoice `json:"choices"`
}

func TestNewRouter_CreatesNonNilRouter(t *testing.T) {
	cfg := ai.Config{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434/v1",
		Model:    "qwen2.5:7b",
	}
	logger := services.NewLogger()
	r := NewRouter(cfg, logger, t.TempDir(), t.TempDir())
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.model != "qwen2.5:7b" {
		t.Errorf("expected model 'qwen2.5:7b', got %q", r.model)
	}
	if r.client == nil {
		t.Error("expected non-nil ai client")
	}
}

func TestRoute_WithValidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: `{"event": "EXECUTE_TODO", "description": "buy milk", "confidence": 0.92}`}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{
		BaseURL: srv.URL,
		Model:   "test-model",
	}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")

	result, err := r.Route("buy milk", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "EXECUTE_TODO" {
		t.Errorf("expected event 'EXECUTE_TODO', got %q", result.Event)
	}
	if result.Description != "buy milk" {
		t.Errorf("expected description 'buy milk', got %q", result.Description)
	}
	if result.Confidence != 0.92 {
		t.Errorf("expected confidence 0.92, got %f", result.Confidence)
	}
}

func TestRoute_WithInvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: `{"event": "EXECUTE_TODO", "confidence": 0.85, `}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{
		BaseURL: srv.URL,
		Model:   "test-model",
	}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")

	_, err := r.Route("do something", "", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestRoute_WithNonJSONText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: "This is just plain text without any JSON"}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{
		BaseURL: srv.URL,
		Model:   "test-model",
	}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")

	_, err := r.Route("do something", "", "")
	if err == nil {
		t.Fatal("expected error for non-JSON text response")
	}
}

func TestSetModel_UpdatesModel(t *testing.T) {
	r := &Router{model: "old-model"}
	r.SetModel("new-model")
	if r.model != "new-model" {
		t.Errorf("expected 'new-model', got %q", r.model)
	}
}

type mockWeightProvider struct{}

func (m *mockWeightProvider) BuildWeightLine(pipelineName, userInput string, baseWeight float64) string {
	return ""
}

func (m *mockWeightProvider) BuildCalibrationBlock() string {
	return ""
}

func (m *mockWeightProvider) BuildKeywordBlock(userInput string, pipelineKeywords map[string][]string) string {
	return ""
}

func (m *mockWeightProvider) CalibrateWithEvent(rawConfidence float64, event string) float64 {
	return rawConfidence
}

func TestSetWeightProvider_SetsWeightProvider(t *testing.T) {
	r := &Router{}
	mock := &mockWeightProvider{}
	r.SetWeightProvider(mock)
	if r.weights == nil {
		t.Fatal("expected weights to be set")
	}
	if r.weights != mock {
		t.Error("expected weights to reference the mock provider")
	}
}

func TestHealthCheck_ReturnsFalseWhenServerUnreachable(t *testing.T) {
	cfg := ai.Config{
		BaseURL: "http://127.0.0.1:1",
	}
	logger := services.NewLogger()
	r := NewRouter(cfg, logger, t.TempDir(), t.TempDir())
	if r.HealthCheck() {
		t.Error("expected HealthCheck to return false")
	}
}

func TestGenerateTitle_ReturnsTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: "buy milk groceries"}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	title, err := r.GenerateTitle("buy milk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "buy milk groceries" {
		t.Errorf("expected 'buy milk groceries', got %q", title)
	}
}

func TestChat_ReturnsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: "Here is some help with that."}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	resp, err := r.Chat("help me", "summary text", "search results", "topic title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "Here is some help with that." {
		t.Errorf("unexpected response: %q", resp)
	}
}

func TestChatStream_ReturnsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: "streamed response"}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	tokenCh, errCh := r.ChatStream("hello", "summary", "search results", "topic")

	select {
	case token := <-tokenCh:
		if token != "streamed response" {
			t.Errorf("expected 'streamed response', got %q", token)
		}
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChat_WithAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	_, err := r.Chat("hello", "", "", "")
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !strings.Contains(err.Error(), "chat request failed") {
		t.Errorf("expected 'chat request failed' error, got %v", err)
	}
}

func TestChat_EmptyInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: "response to empty input"}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	resp, err := r.Chat("", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response to empty input" {
		t.Errorf("expected 'response to empty input', got %q", resp)
	}
}

func TestChatStream_WithAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("stream error"))
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	tokenCh, errCh := r.ChatStream("hello", "", "", "")

	select {
	case <-tokenCh:
		t.Fatal("expected error, not token")
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if !strings.Contains(err.Error(), "API returned 500") {
			t.Errorf("expected 'API returned 500' error, got %v", err)
		}
	}
}

func TestHealthCheck_WithReachableServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ai.ModelList{
			Data: []struct {
				ID string `json:"id"`
			}{
				{ID: "test-model"},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	if !r.HealthCheck() {
		t.Error("expected HealthCheck to return true for reachable server")
	}
}

func TestGenerateTitle_WithAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), t.TempDir())

	_, err := r.GenerateTitle("test query")
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !strings.Contains(err.Error(), "title generation request failed") {
		t.Errorf("expected 'title generation request failed', got %v", err)
	}
}

func TestRoute_WithWeightProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testChatResponse{
			Choices: []testChoice{
				{Message: testMessage{Content: `{"event": "EXECUTE_TODO", "confidence": 0.9}`}},
			},
		})
	}))
	defer srv.Close()

	cfg := ai.Config{BaseURL: srv.URL, Model: "test"}
	r := NewRouter(cfg, services.NewLogger(), t.TempDir(), "/nonexistent/pipelines.yaml")
	r.SetWeightProvider(&mockWeightProvider{})

	result, err := r.Route("buy milk", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Event != "EXECUTE_TODO" {
		t.Errorf("expected event 'EXECUTE_TODO', got %q", result.Event)
	}
	if result.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", result.Confidence)
	}
}
