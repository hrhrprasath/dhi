package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM LlmConfig `yaml:"llm"`
}

type LlmConfig struct {
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
}

func (l *LlmConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*l = LlmConfig{
		Endpoint: "http://localhost:11434",
		Model:    "qwen2.5:1.5b",
	}
	type raw LlmConfig
	return unmarshal((*raw)(l))
}

func Load() *Config {
	paths := []string{"config/config.yaml", "config.yaml"}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err == nil {
			return &cfg
		}
	}
	return &Config{
		LLM: LlmConfig{
			Endpoint: "http://localhost:11434",
			Model:    "qwen2.5:1.5b",
		},
	}
}
