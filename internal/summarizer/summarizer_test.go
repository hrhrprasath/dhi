package summarizer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hrhrprasath/dhi/internal/ai"
)

func TestSummarize_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "This is a test summary"}},
			},
		})
	}))
	defer server.Close()

	s := New(ai.Config{
		BaseURL:     server.URL,
		Model:       "test-model",
		MaxTokens:   200,
		Temperature: 0.5,
	})

	result := s.Summarize("raw data content", "test_pipeline", "user query", "previous context")
	if result != "This is a test summary" {
		t.Errorf("Summarize() = %q, want %q", result, "This is a test summary")
	}
}

func TestSummarize_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream failure"))
	}))
	defer server.Close()

	s := New(ai.Config{
		BaseURL:     server.URL,
		Model:       "test-model",
		MaxTokens:   200,
		Temperature: 0.5,
	})

	result := s.Summarize("data", "pipeline", "input", "")
	if !strings.HasPrefix(result, "[Summary unavailable") {
		t.Errorf("Summarize() should return error message, got %q", result)
	}
	if !strings.Contains(result, "upstream failure") {
		t.Errorf("Summarize() error should contain upstream message, got %q", result)
	}
}

func TestSummarize_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	s := New(ai.Config{
		BaseURL:     server.URL,
		Model:       "test-model",
		MaxTokens:   200,
		Temperature: 0.5,
	})

	result := s.Summarize("data", "pipeline", "input", "")
	if !strings.HasPrefix(result, "[Summary unavailable") {
		t.Errorf("Summarize() should return error message for empty response, got %q", result)
	}
	if !strings.Contains(result, "empty response") {
		t.Errorf("Summarize() error should mention empty response, got %q", result)
	}
}

func TestSummarize_WithSetModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatal(err)
		}
		if model, ok := reqBody["model"].(string); !ok || model != "custom-model" {
			t.Errorf("expected model 'custom-model', got %v", reqBody["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "response with custom model"}},
			},
		})
	}))
	defer server.Close()

	s := New(ai.Config{
		BaseURL:     server.URL,
		Model:       "default-model",
		MaxTokens:   200,
		Temperature: 0.5,
	})
	s.SetModel("custom-model")

	result := s.Summarize("data", "pipeline", "input", "")
	if result != "response with custom model" {
		t.Errorf("Summarize() = %q, want %q", result, "response with custom model")
	}
}
