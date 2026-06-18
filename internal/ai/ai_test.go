package ai

import (
	"context"
	"encoding/json"
	"testing"
)

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Provider:    "ollama",
		BaseURL:     "http://localhost:11434/v1",
		APIKey:      "",
		Model:       "qwen2.5:7b",
		MaxTokens:   4096,
		Temperature: 0.7,
	}
	if cfg.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", cfg.Provider)
	}
	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("expected http://localhost:11434/v1, got %s", cfg.BaseURL)
	}
	if cfg.Model != "qwen2.5:7b" {
		t.Errorf("expected qwen2.5:7b, got %s", cfg.Model)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected 4096, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", cfg.Temperature)
	}
}

func TestChatRequestJSON(t *testing.T) {
	req := ChatRequest{
		Model: "qwen2.5:7b",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   2048,
		Temperature: 0.5,
		Stream:      false,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Model != "qwen2.5:7b" {
		t.Errorf("expected qwen2.5:7b, got %s", decoded.Model)
	}
	if len(decoded.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != "user" {
		t.Errorf("expected user role, got %s", decoded.Messages[0].Role)
	}
	if decoded.Messages[0].Content != "Hello" {
		t.Errorf("expected Hello, got %s", decoded.Messages[0].Content)
	}
	if decoded.MaxTokens != 2048 {
		t.Errorf("expected 2048, got %d", decoded.MaxTokens)
	}
	if decoded.Temperature != 0.5 {
		t.Errorf("expected 0.5, got %f", decoded.Temperature)
	}
	if decoded.Stream != false {
		t.Errorf("expected stream false, got %v", decoded.Stream)
	}
}

func TestChatMessage(t *testing.T) {
	msg := ChatMessage{
		Role:    "assistant",
		Content: "I am an AI.",
	}
	if msg.Role != "assistant" {
		t.Errorf("expected assistant, got %s", msg.Role)
	}
	if msg.Content != "I am an AI." {
		t.Errorf("expected 'I am an AI.', got %s", msg.Content)
	}
}

func TestModelListJSON(t *testing.T) {
	ml := ModelList{
		Data: []struct {
			ID string `json:"id"`
		}{
			{ID: "qwen2.5:7b"},
			{ID: "llama3:8b"},
		},
	}

	data, err := json.Marshal(ml)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ModelList
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.Data) != 2 {
		t.Errorf("expected 2 models, got %d", len(decoded.Data))
	}
	if decoded.Data[0].ID != "qwen2.5:7b" {
		t.Errorf("expected qwen2.5:7b, got %s", decoded.Data[0].ID)
	}
	if decoded.Data[1].ID != "llama3:8b" {
		t.Errorf("expected llama3:8b, got %s", decoded.Data[1].ID)
	}
}

func TestChatCompletionDeserialization(t *testing.T) {
	jsonStr := `{"choices":[{"message":{"content":"Hello! How can I help?"}}]}`

	var completion ChatCompletion
	if err := json.Unmarshal([]byte(jsonStr), &completion); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(completion.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(completion.Choices))
	}
	if completion.Choices[0].Message.Content != "Hello! How can I help?" {
		t.Errorf("expected 'Hello! How can I help?', got %s", completion.Choices[0].Message.Content)
	}
}

func TestHealthCheck_ReturnsFalseOnNewRequestError(t *testing.T) {
	cfg := Config{
		BaseURL: "\x00",
	}
	client := NewClient(cfg)
	ctx := context.Background()
	if client.HealthCheck(ctx) {
		t.Error("expected HealthCheck to return false when NewRequest fails")
	}
}

func TestNewClient(t *testing.T) {
	cfg := Config{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434/v1",
		Model:    "qwen2.5:7b",
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.config.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", client.config.Provider)
	}
	if client.config.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("expected http://localhost:11434/v1, got %s", client.config.BaseURL)
	}
	if client.config.Model != "qwen2.5:7b" {
		t.Errorf("expected qwen2.5:7b, got %s", client.config.Model)
	}
	if client.http == nil {
		t.Error("expected non-nil http client")
	}
}

func TestHealthCheckReturnsFalse(t *testing.T) {
	cfg := Config{
		BaseURL: "http://127.0.0.1:1",
	}
	client := NewClient(cfg)
	ctx := context.Background()
	if client.HealthCheck(ctx) {
		t.Error("expected HealthCheck to return false when server is not running")
	}
}
