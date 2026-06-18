package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dhi_harness/internal/ai"
)

type Settings struct {
	SelectedModel string      `json:"selected_model"`
	Models        []ModelInfo `json:"models"`
}

type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
	ContextLen int    `json:"context_length"`
}

type Manager struct {
	dataDir  string
	client   *ai.Client
	settings *Settings
	mu       sync.Mutex
	config   ai.Config
}

var knownContextLengths = map[string]int{
	"qwen2.5:1.5b":      32768,
	"qwen2.5:3b":        32768,
	"qwen2.5:7b":        32768,
	"qwen2.5:14b":       32768,
	"llama3.2:1b":       131072,
	"llama3.2:3b":       131072,
	"llama3.1:8b":       131072,
	"llama3.1:70b":      131072,
	"mistral:7b":        32768,
	"mistral-nemo:12b":  128000,
	"mixtral:8x7b":      32768,
	"deepseek:7b":       4096,
	"deepseek-r1:7b":    16384,
	"codellama:7b":      16384,
	"codellama:13b":     16384,
	"nomic-embed-text":  8192,
	"mxbai-embed-large": 512,
	"phi3:3.8b":         4096,
	"phi3:14b":          128000,
	"gemma2:2b":         8192,
	"gemma2:7b":         8192,
	"gemma2:27b":        8192,
}

func NewManager(dataDir string, client *ai.Client, cfg ai.Config) *Manager {
	dir := filepath.Join(dataDir, "settings")
	_ = os.MkdirAll(dir, 0755)

	m := &Manager{
		dataDir:  dataDir,
		client:   client,
		config:   cfg,
		settings: &Settings{},
	}

	m.load()
	return m
}

func (m *Manager) FetchModels() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	models, err := m.client.ListModels(context.Background())
	if err != nil {
		return fmt.Errorf("fetch models: %w", err)
	}

	infos := make([]ModelInfo, 0, len(models))
	for _, name := range models {
		infos = append(infos, ModelInfo{
			Name:       name,
			ContextLen: m.lookupContextLength(name),
		})
	}
	m.settings.Models = infos

	m.resolveSelection()

	_ = m.save()
	return nil
}

func (m *Manager) resolveSelection() {
	if m.settings.SelectedModel != "" {
		for _, mdl := range m.settings.Models {
			if mdl.Name == m.settings.SelectedModel {
				return
			}
		}
		m.settings.SelectedModel = ""
	}

	if m.config.Model != "" {
		for _, mdl := range m.settings.Models {
			if mdl.Name == m.config.Model {
				m.settings.SelectedModel = mdl.Name
				return
			}
		}
	}

	if len(m.settings.Models) > 0 {
		m.settings.SelectedModel = m.settings.Models[0].Name
		return
	}

	if m.config.Model != "" {
		m.settings.Models = append(m.settings.Models, ModelInfo{
			Name:       m.config.Model,
			ContextLen: m.lookupContextLength(m.config.Model),
		})
		m.settings.SelectedModel = m.config.Model
	}
}

func (m *Manager) lookupContextLength(name string) int {
	name = strings.ToLower(name)

	if cl, ok := knownContextLengths[name]; ok {
		return cl
	}

	for key, cl := range knownContextLengths {
		if strings.HasPrefix(name, key) {
			return cl
		}
	}

	return 4096
}

func (m *Manager) SelectModel(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	for _, mdl := range m.settings.Models {
		if mdl.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("model %q not found in available models", name)
	}

	m.settings.SelectedModel = name
	_ = m.save()
	return nil
}

func (m *Manager) CurrentModel() *ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.settings.SelectedModel == "" {
		return nil
	}
	for i := range m.settings.Models {
		if m.settings.Models[i].Name == m.settings.SelectedModel {
			return &m.settings.Models[i]
		}
	}
	return nil
}

func (m *Manager) ListModels() []ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]ModelInfo, len(m.settings.Models))
	copy(result, m.settings.Models)
	return result
}

func (m *Manager) ContextLength() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.settings.SelectedModel == "" {
		return 4096
	}
	for _, mdl := range m.settings.Models {
		if mdl.Name == m.settings.SelectedModel && mdl.ContextLen > 0 {
			return mdl.ContextLen
		}
	}
	return 4096
}

func (m *Manager) settingsPath() string {
	return filepath.Join(m.dataDir, "settings", "settings.json")
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.settingsPath())
	if err != nil {
		return
	}
	var s Settings
	if json.Unmarshal(data, &s) != nil {
		return
	}
	m.settings = &s
}

func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.settingsPath(), data, 0644)
}
