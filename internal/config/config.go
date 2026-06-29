package config

import (
	"fmt"
	"os"
	"sync"

	"dhi_harness/internal/ai"

	"gopkg.in/yaml.v3"
)

var (
	loadOnce sync.Once
	cfg      AIConfig
	loadErr  error
)

type AIConfig struct {
	AI          ai.Config            `yaml:"ai"`
	Providers   map[string]ai.Config `yaml:"providers,omitempty"`
	RSS         RSSConfig            `yaml:"rss"`
	AccentColor string               `yaml:"accent_color"`
}

type RSSConfig struct {
	SkipShorts bool `yaml:"skip_shorts"`
}

const defaultConfigPath = ""

func Load(path string) (*AIConfig, error) {
	loadOnce.Do(func() {
		if path == "" {
			path = defaultConfigPath
		}
		cfg, loadErr = loadFile(path)
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return &cfg, nil
}

func loadFile(path string) (AIConfig, error) {
	var ac AIConfig

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return ac, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &ac); err != nil {
		return ac, fmt.Errorf("parse config: %w", err)
	}

	resolveEnvVars(&ac)

	return ac, nil
}

func resolveEnvVars(ac *AIConfig) {
	ac.AI.APIKey = os.ExpandEnv(ac.AI.APIKey)
	for name, p := range ac.Providers {
		p.APIKey = os.ExpandEnv(p.APIKey)
		ac.Providers[name] = p
	}
}

func defaultConfig() AIConfig {
	return AIConfig{
		AI: ai.Config{
			Provider:    "ollama",
			BaseURL:     "http://localhost:11434/v1",
			Model:       "qwen2.5:7b",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		RSS: RSSConfig{
			SkipShorts: true,
		},
		AccentColor: "#FFA500",
	}
}

func ForceReload(path string) (*AIConfig, error) {
	loadOnce = sync.Once{}
	return Load(path)
}
