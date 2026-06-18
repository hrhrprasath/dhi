package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_CreatesClientWithConfig(t *testing.T) {
	cfg := Config{
		Provider:    "ollama",
		BaseURL:     "http://localhost:11434/v1",
		APIKey:      "sk-test",
		Model:       "qwen2.5:7b",
		MaxTokens:   4096,
		Temperature: 0.7,
	}
	c := NewClient(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.config.Provider != "ollama" {
		t.Errorf("expected Provider 'ollama', got %q", c.config.Provider)
	}
	if c.config.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("expected BaseURL 'http://localhost:11434/v1', got %q", c.config.BaseURL)
	}
	if c.config.APIKey != "sk-test" {
		t.Errorf("expected APIKey 'sk-test', got %q", c.config.APIKey)
	}
	if c.config.Model != "qwen2.5:7b" {
		t.Errorf("expected Model 'qwen2.5:7b', got %q", c.config.Model)
	}
	if c.config.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens 4096, got %d", c.config.MaxTokens)
	}
	if c.config.Temperature != 0.7 {
		t.Errorf("expected Temperature 0.7, got %f", c.config.Temperature)
	}
	if c.http == nil {
		t.Error("expected non-nil http client")
	}
}

func TestHealthCheck_ReturnsTrueOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/models" {
			t.Errorf("expected /models, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	if !c.HealthCheck(context.Background()) {
		t.Error("expected HealthCheck to return true")
	}
}

func TestHealthCheck_ReturnsFalseOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	if c.HealthCheck(context.Background()) {
		t.Error("expected HealthCheck to return false")
	}
}

func TestChat_ReturnsContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "Hello! How can I help you?"}},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	resp, err := c.Chat(context.Background(), ChatRequest{
		Model: "test",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "Hello! How can I help you?" {
		t.Errorf("expected 'Hello! How can I help you?', got %q", resp)
	}
}

func TestChat_ReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`invalid request`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	_, err := c.Chat(context.Background(), ChatRequest{
		Model: "test",
	})
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestListModels_ReturnsModelNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/models" {
			t.Errorf("expected /models, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ModelList{
			Data: []struct {
				ID string `json:"id"`
			}{
				{ID: "qwen2.5:7b"},
				{ID: "llama3:8b"},
				{ID: "mistral:7b"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0] != "qwen2.5:7b" {
		t.Errorf("expected 'qwen2.5:7b', got %q", models[0])
	}
	if models[1] != "llama3:8b" {
		t.Errorf("expected 'llama3:8b', got %q", models[1])
	}
	if models[2] != "mistral:7b" {
		t.Errorf("expected 'mistral:7b', got %q", models[2])
	}
}

func TestChatStream_ReturnsFullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ChatCompletion{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "streamed response content"}},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL})
	tokenCh, errCh := c.ChatStream(context.Background(), ChatRequest{
		Model: "test",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	})

	select {
	case token := <-tokenCh:
		if token != "streamed response content" {
			t.Errorf("expected 'streamed response content', got %q", token)
		}
	case err := <-errCh:
		t.Fatalf("unexpected error from stream: %v", err)
	}

	if _, ok := <-tokenCh; ok {
		t.Error("expected token channel to be closed")
	}
	if _, ok := <-errCh; ok {
		t.Error("expected error channel to be closed")
	}
}
