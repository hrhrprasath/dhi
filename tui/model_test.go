package tui

import (
	"strings"
	"testing"

	"dhi_harness/internal/settings"

	"charm.land/bubbles/v2/viewport"
)

func TestModelPanel_DefaultFields(t *testing.T) {
	mp := &modelPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	if mp.popupOpen {
		t.Error("popupOpen should be false by default")
	}
	if mp.selIdx != 0 {
		t.Errorf("selIdx should be 0, got %d", mp.selIdx)
	}
}

func TestModelPanel_RenderPopupEmpty(t *testing.T) {
	mp := &modelPanel{
		filtered: nil,
		selIdx:   0,
		view:     viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	out := mp.renderPopup(40, nil)
	if !strings.Contains(out, "No matching models") {
		t.Errorf("renderPopup() should contain 'No matching models', got: %s", out)
	}
}

func TestModelPanel_RenderPopupWithModels(t *testing.T) {
	mp := &modelPanel{
		filtered: []settings.ModelInfo{
			{Name: "llama3:8b", Size: 4_700_000_000, ContextLen: 8192},
			{Name: "qwen2:1.5b", Size: 1_500_000_000, ContextLen: 32768},
		},
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(60), viewport.WithHeight(10)),
	}
	out := mp.renderPopup(60, nil)
	if !strings.Contains(out, "ALL MODELS") {
		t.Errorf("renderPopup() should contain 'ALL MODELS', got: %s", out)
	}
	if !strings.Contains(out, "llama3") {
		t.Errorf("renderPopup() should contain 'llama3', got: %s", out)
	}
	if !strings.Contains(out, "qwen2") {
		t.Errorf("renderPopup() should contain 'qwen2', got: %s", out)
	}
}

func TestModelPanel_RenderPopupSelectionArrow(t *testing.T) {
	mp := &modelPanel{
		filtered: []settings.ModelInfo{
			{Name: "model-a", Size: 1_000_000_000, ContextLen: 4096},
			{Name: "model-b", Size: 2_000_000_000, ContextLen: 8192},
		},
		selIdx: 1,
		view:   viewport.New(viewport.WithWidth(60), viewport.WithHeight(10)),
	}
	cur := &settings.ModelInfo{Name: "model-b", Size: 2_000_000_000, ContextLen: 8192}
	out := mp.renderPopup(60, cur)
	if !strings.Contains(out, "▸") {
		t.Errorf("renderPopup() should contain selection arrow, got: %s", out)
	}
}

func TestModelPanel_RenderPopupWithFilter(t *testing.T) {
	mp := &modelPanel{
		filter: "llama",
		filtered: []settings.ModelInfo{
			{Name: "llama3:8b", Size: 4_700_000_000, ContextLen: 8192},
		},
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(60), viewport.WithHeight(10)),
	}
	out := mp.renderPopup(60, nil)
	if !strings.Contains(out, "Search: llama") {
		t.Errorf("renderPopup() should show filter, got: %s", out)
	}
}

func TestModelPanel_FilterModels(t *testing.T) {
	mp := &modelPanel{}
	all := []settings.ModelInfo{
		{Name: "llama3:8b"},
		{Name: "qwen2:1.5b"},
		{Name: "mistral:7b"},
	}
	mp.filter = "llama"
	mp.filterModels(all)
	if len(mp.filtered) != 1 {
		t.Errorf("expected 1 filtered model, got %d", len(mp.filtered))
	}
	if mp.filtered[0].Name != "llama3:8b" {
		t.Errorf("expected 'llama3:8b', got %q", mp.filtered[0].Name)
	}
}

func TestModelPanel_FilterModelsEmpty(t *testing.T) {
	mp := &modelPanel{filter: ""}
	all := []settings.ModelInfo{
		{Name: "a"},
		{Name: "b"},
	}
	mp.filterModels(all)
	if len(mp.filtered) != 2 {
		t.Errorf("expected 2 models with empty filter, got %d", len(mp.filtered))
	}
}

func TestModelPanel_FilterModelsNoMatch(t *testing.T) {
	mp := &modelPanel{filter: "zzz"}
	all := []settings.ModelInfo{
		{Name: "hello"},
	}
	mp.filterModels(all)
	if len(mp.filtered) != 0 {
		t.Errorf("expected 0 matched models, got %d", len(mp.filtered))
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0MB"},
		{"1MB", 1_000_000, "1MB"},
		{"500MB", 500_000_000, "500MB"},
		{"1GB", 1_000_000_000, "1.0GB"},
		{"4.7GB", 4_700_000_000, "4.7GB"},
		{"15GB", 15_000_000_000, "15.0GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
