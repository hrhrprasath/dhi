package settings

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Settings struct {
	SelectedModel string      `json:"selected_model"`
	Endpoint      string      `json:"endpoint"`
	Models        []ModelInfo `json:"models"`
}

type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	ContextLen int    `json:"context_length"`
}

type Manager struct {
	dataDir  string
	endpoint string
	settings *Settings
	mu       sync.Mutex
	client   *http.Client
}

type tagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Size       int64  `json:"size"`
		ModifiedAt string `json:"modified_at"`
	} `json:"models"`
}

type showResponse struct {
	ModelInfo map[string]any `json:"model_info"`
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

func NewManager(dataDir, endpoint string) *Manager {
	dir := filepath.Join(dataDir, "settings")
	os.MkdirAll(dir, 0755)

	m := &Manager{
		dataDir:  dataDir,
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
		settings: &Settings{Endpoint: endpoint},
	}

	m.load()
	return m
}

func (m *Manager) FetchModels() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp, err := m.client.Get(fmt.Sprintf("%s/api/tags", m.endpoint))
	if err != nil {
		return fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read tags response: %w", err)
	}

	var tr tagsResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("parse tags: %w", err)
	}

	models := make([]ModelInfo, 0, len(tr.Models))
	for _, m := range tr.Models {
		models = append(models, ModelInfo{
			Name:       m.Name,
			Size:       m.Size,
			ModifiedAt: m.ModifiedAt,
		})
	}
	m.settings.Models = models

	m.resolveSelection()
	m.resolveContextLengths()

	m.save()
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
	if len(m.settings.Models) > 0 {
		m.settings.SelectedModel = m.settings.Models[0].Name
	}
}

func (m *Manager) resolveContextLengths() {
	for i, mdl := range m.settings.Models {
		if mdl.ContextLen > 0 {
			continue
		}
		cl := m.lookupContextLength(mdl.Name)
		m.settings.Models[i].ContextLen = cl
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

	cl, err := m.fetchContextLength(name)
	if err == nil && cl > 0 {
		return cl
	}

	return 4096
}

func (m *Manager) fetchContextLength(name string) (int, error) {
	reqBody := map[string]string{"model": name}
	data, _ := json.Marshal(reqBody)
	resp, err := m.client.Post(
		fmt.Sprintf("%s/api/show", m.endpoint),
		"application/json",
		strings.NewReader(string(data)),
	)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var sr showResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return 0, err
	}

	arch, _ := sr.ModelInfo["general.architecture"].(string)
	if arch == "" {
		return 0, fmt.Errorf("no architecture found")
	}
	clKey := arch + ".context_length"
	clRaw, ok := sr.ModelInfo[clKey]
	if !ok {
		return 0, fmt.Errorf("no context_length for arch %s", arch)
	}
	clFloat, ok := clRaw.(float64)
	if !ok {
		return 0, fmt.Errorf("context_length not a number")
	}
	return int(clFloat), nil
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
	m.resolveContextLengths()
	m.save()
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

func (m *Manager) Endpoint() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings.Endpoint
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
	if s.Endpoint == "" {
		s.Endpoint = m.endpoint
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
