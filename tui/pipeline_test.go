package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
)

func TestPipelineEntry_Struct(t *testing.T) {
	e := pipelineEntry{name: "test", status: "RUNNING"}
	if e.name != "test" {
		t.Errorf("expected name 'test', got %q", e.name)
	}
	if e.status != "RUNNING" {
		t.Errorf("expected status 'RUNNING', got %q", e.status)
	}
}

func TestPipelinePanel_RenderEmpty(t *testing.T) {
	p := &pipelinePanel{
		entries: nil,
		view:    viewport.New(viewport.WithWidth(28), viewport.WithHeight(5)),
	}
	out := p.render()
	if !strings.Contains(out, "PIPELINES") {
		t.Errorf("render() should contain 'PIPELINES', got: %s", out)
	}
}

func TestPipelinePanel_RenderWithEntries(t *testing.T) {
	p := &pipelinePanel{
		entries: []pipelineEntry{
			{name: "web_search", status: "IDLE"},
			{name: "stocks", status: "RUNNING"},
			{name: "music", status: "FAILED"},
		},
		view: viewport.New(viewport.WithWidth(28), viewport.WithHeight(10)),
	}
	out := p.render()
	if !strings.Contains(out, "web_search") {
		t.Errorf("render() should contain 'web_search'")
	}
	if !strings.Contains(out, "stocks") {
		t.Errorf("render() should contain 'stocks'")
	}
	if !strings.Contains(out, "music") {
		t.Errorf("render() should contain 'music'")
	}
}

func TestPipelinePanel_RenderStatusColors(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"idle", "IDLE"},
		{"running", "RUNNING"},
		{"failed", "FAILED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &pipelinePanel{
				entries: []pipelineEntry{{name: "p", status: tt.status}},
				view:    viewport.New(viewport.WithWidth(28), viewport.WithHeight(5)),
			}
			out := p.render()
			if !strings.Contains(out, tt.status) {
				t.Errorf("render() should contain status %q, got: %s", tt.status, out)
			}
		})
	}
}
