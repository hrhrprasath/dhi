package pipeline

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Params      []ParamDefinition   `yaml:"params"`
	Action      PipelineAction      `yaml:"action"`
}

type ParamDefinition struct {
	Name        string `yaml:"name"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
	ExtractHint string `yaml:"extract_hint,omitempty"`
}

type PipelineAction struct {
	Type      string            `yaml:"type"`
	URL       string            `yaml:"url,omitempty"`
	Method    string            `yaml:"method,omitempty"`
	Command   string            `yaml:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty"`
	Script    string            `yaml:"script,omitempty"`
	Target    string            `yaml:"target,omitempty"`
	Handler   string            `yaml:"handler,omitempty"`
	Auth      *HTTPAuth         `yaml:"auth,omitempty"`
	Summarize bool              `yaml:"summarize"`
}

type HTTPAuth struct {
	Header string `yaml:"header"`
	Value  string `yaml:"value"`
}

type Manager struct {
	pipelines []Pipeline
}

type Config struct {
	Pipelines []Pipeline `yaml:"pipelines"`
}

func NewManager(path string) *Manager {
	if path == "" {
		path = "config/pipelines.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &Manager{pipelines: defaultPipelines()}
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil || len(cfg.Pipelines) == 0 {
		return &Manager{pipelines: defaultPipelines()}
	}
	return &Manager{pipelines: cfg.Pipelines}
}

func (m *Manager) GetPipeline(name string) *Pipeline {
	for i := range m.pipelines {
		if strings.EqualFold(m.pipelines[i].Name, name) {
			return &m.pipelines[i]
		}
	}
	return nil
}

func (m *Manager) ListPipelines() []Pipeline {
	out := make([]Pipeline, len(m.pipelines))
	copy(out, m.pipelines)
	return out
}

func (m *Manager) ListDescriptions() []string {
	var descs []string
	for _, p := range m.pipelines {
		descs = append(descs, fmt.Sprintf("%s: %s", p.Name, p.Description))
	}
	return descs
}

func (m *Manager) PipelineCount() int {
	return len(m.pipelines)
}

func defaultPipelines() []Pipeline {
	return []Pipeline{
		{Name: "stocks", Description: "Stock queries with news", Params: []ParamDefinition{
			{Name: "company", Required: true},
		}, Action: PipelineAction{Type: "bash", Script: "scripts/stock {company}"}},
		{Name: "search_inline", Description: "Search web inline", Params: []ParamDefinition{
			{Name: "query", Required: true},
		}, Action: PipelineAction{Type: "bash", Summarize: true, Script: "scripts/search_inline {query}"}},
		{Name: "search_browser", Description: "Open a website or search in default browser — only when explicitly asked to open/open in browser", Params: []ParamDefinition{
			{Name: "query", Required: true},
			{Name: "provider", Required: false},
		}, Action: PipelineAction{Type: "bash", Script: "scripts/search_browser {provider} {query}"}},
		{Name: "browse", Description: "Read webpage", Params: []ParamDefinition{
			{Name: "url", Required: true},
		}, Action: PipelineAction{Type: "bash", Summarize: true, Script: "scripts/browse {url}"}},
		{Name: "system", Description: "System metrics and observability", Params: []ParamDefinition{
			{Name: "metric", Required: false},
		}, Action: PipelineAction{Type: "internal", Handler: "system_metrics"}},
		{Name: "todo", Description: "Create and manage todos/reminders", Params: []ParamDefinition{
			{Name: "action", Required: false, Default: "list"},
			{Name: "text", Required: false, ExtractHint: "Extract the task description from the user input (e.g., 'buy milk', 'call john', 'pay bill at 5pm')"},
		}, Action: PipelineAction{Type: "internal", Handler: "todo"}},
	}
}
