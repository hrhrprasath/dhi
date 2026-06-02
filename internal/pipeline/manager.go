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
	Weight      float64             `yaml:"weight"`
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
	Summarize     bool `yaml:"summarize"`
	IgnoreContext bool `yaml:"summarize_ignore_context"`
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
	var pipelines []Pipeline
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg Config
		if yaml.Unmarshal(data, &cfg) == nil {
			pipelines = cfg.Pipelines
		}
	}
	pipelines = append(pipelines, defaultPipelines()...)
	return &Manager{pipelines: pipelines}
}

func defaultPipelines() []Pipeline {
	return []Pipeline{
		{Name: "todo", Description: "Create and manage todos/reminders (track, remind, todo, task, remember, follow up)", Weight: 0.9, Params: []ParamDefinition{
			{Name: "action", Required: false, Default: "list"},
			{Name: "text", Required: false, ExtractHint: "Extract the task description from the user input (e.g., 'buy milk', 'call john', 'pay bill at 5pm')"},
		}, Action: PipelineAction{Type: "internal", Handler: "todo"}},
		{Name: "system", Description: "System metrics and observability (system info, metrics, status, cpu, ram, uptime, kernel)", Weight: 0.6, Params: []ParamDefinition{
			{Name: "metric", Required: false, Default: "all"},
		}, Action: PipelineAction{Type: "internal", Handler: "system_metrics"}},
		{Name: "schedule", Description: "Schedule tasks for later execution (schedule, remind, at, in, every, daily)", Weight: 0.8, Params: []ParamDefinition{
			{Name: "action", Required: false, Default: "add", ExtractHint: "add, list, delete, update"},
			{Name: "when", Required: false, ExtractHint: "When to run (e.g., 'in 5 minutes', 'daily at 9am', 'tomorrow at 5pm')"},
			{Name: "command", Required: false, ExtractHint: "What to do at the scheduled time"},
			{Name: "schedule_id", Required: false},
			{Name: "schedules", Required: false, ExtractHint: "JSON array of {when, command} objects for multi-schedule"},
		}, Action: PipelineAction{Type: "internal", Handler: "schedule"}},
	}
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
