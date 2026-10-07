package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigProvider(t *testing.T) {
	cfg := defaultConfig()
	if cfg.AI.Provider != "ollama" {
		t.Errorf("expected provider ollama, got %s", cfg.AI.Provider)
	}
}

func TestLoadNonexistentPath(t *testing.T) {
	dir := t.TempDir()
	_, err := ForceReload(dir)
	if err == nil {
		t.Error("expected error when loading a directory path, got nil")
	}
}

func TestForceReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte("ai:\n  provider: ollama\n  base_url: http://localhost:11434/v1\n  model: qwen2.5:7b\n  max_tokens: 4096\n  temperature: 0.7\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ForceReload(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AI.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", cfg.AI.Provider)
	}

	content2 := []byte("ai:\n  provider: openai\n  base_url: https://api.openai.com/v1\n  model: gpt-4\n  max_tokens: 2048\n  temperature: 0.5\n")
	if err := os.WriteFile(path, content2, 0644); err != nil {
		t.Fatal(err)
	}

	cfg2, err := ForceReload(path)
	if err != nil {
		t.Fatalf("ForceReload failed: %v", err)
	}
	if cfg2.AI.Provider != "openai" {
		t.Errorf("expected openai after reload, got %s", cfg2.AI.Provider)
	}
}

func TestResolveEnvVars(t *testing.T) {
	_ = os.Setenv("TEST_API_KEY", "sk-test-key")
	defer func() { _ = os.Unsetenv("TEST_API_KEY") }()

	cfg := defaultConfig()
	cfg.AI.APIKey = "${TEST_API_KEY}"

	resolveEnvVars(&cfg)

	if cfg.AI.APIKey != "sk-test-key" {
		t.Errorf("expected sk-test-key, got %s", cfg.AI.APIKey)
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte("ai:\n  provider: openai\n  base_url: https://api.openai.com/v1\n  api_key: sk-123\n  model: gpt-4\n  max_tokens: 2048\n  temperature: 0.5\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ForceReload(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AI.Provider != "openai" {
		t.Errorf("expected openai, got %s", cfg.AI.Provider)
	}
	if cfg.AI.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected https://api.openai.com/v1, got %s", cfg.AI.BaseURL)
	}
	if cfg.AI.APIKey != "sk-123" {
		t.Errorf("expected sk-123, got %s", cfg.AI.APIKey)
	}
	if cfg.AI.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", cfg.AI.Model)
	}
	if cfg.AI.MaxTokens != 2048 {
		t.Errorf("expected 2048, got %d", cfg.AI.MaxTokens)
	}
	if cfg.AI.Temperature != 0.5 {
		t.Errorf("expected 0.5, got %f", cfg.AI.Temperature)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte("ai:\n  provider: openai\n  invalid_yaml: [\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ForceReload(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestDefaultConfigSystemOneModelEmpty(t *testing.T) {
	cfg := defaultConfig()
	if cfg.AI.SystemOneModel != "" {
		t.Errorf("expected empty systemone_model, got %q", cfg.AI.SystemOneModel)
	}
}

func TestLoadSystemOneModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte("ai:\n  provider: ollama\n  base_url: http://localhost:11434/v1\n  model: qwen2.5:1.5b\n  systemone_model: tev1:0.8b-mxfp8\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ForceReload(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AI.SystemOneModel != "tev1:0.8b-mxfp8" {
		t.Errorf("expected tev1:0.8b-mxfp8, got %q", cfg.AI.SystemOneModel)
	}
}

func TestLoadWithoutSystemOneModelKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte("ai:\n  provider: ollama\n  model: qwen2.5:1.5b\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ForceReload(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AI.SystemOneModel != "" {
		t.Errorf("expected empty systemone_model for missing key, got %q", cfg.AI.SystemOneModel)
	}
}
