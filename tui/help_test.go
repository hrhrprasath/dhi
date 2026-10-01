package tui

import (
	"strings"
	"testing"
)

func TestHelpPanel_DefaultFields(t *testing.T) {
	hp := newHelpPanel()
	if hp.popupOpen {
		t.Error("popupOpen should be false by default")
	}
}

func TestHelpPanel_RenderPopupNonEmpty(t *testing.T) {
	hp := newHelpPanel()
	out := hp.renderPopup(40)
	if out == "" {
		t.Error("renderPopup() returned empty string")
	}
}

func TestHelpPanel_RenderPopupContainsHelp(t *testing.T) {
	hp := newHelpPanel()
	out := hp.renderPopup(40)
	if !strings.Contains(out, "HELP") {
		t.Errorf("renderPopup() should contain 'HELP', got: %s", out)
	}
}

func TestBuildHelpContent_ContainsCommands(t *testing.T) {
	content := buildHelpContent()
	if !strings.Contains(content, "/help") {
		t.Errorf("buildHelpContent() should contain '/help', got: %s", content)
	}
	if !strings.Contains(content, "/quit") {
		t.Errorf("buildHelpContent() should contain '/quit'")
	}
	if !strings.Contains(content, "/topic") {
		t.Errorf("buildHelpContent() should contain '/topic'")
	}
	if !strings.Contains(content, "/model") {
		t.Errorf("buildHelpContent() should contain '/model'")
	}
	if !strings.Contains(content, "/log") {
		t.Errorf("buildHelpContent() should contain '/log'")
	}
	if !strings.Contains(content, "/todo") {
		t.Errorf("buildHelpContent() should contain '/todo'")
	}
	if !strings.Contains(content, "/schedule") {
		t.Errorf("buildHelpContent() should contain '/schedule'")
	}
	if !strings.Contains(content, "/rss") {
		t.Errorf("buildHelpContent() should contain '/rss'")
	}
}

func TestBuildHelpContent_ContainsNavigation(t *testing.T) {
	content := buildHelpContent()
	if !strings.Contains(content, "Navigation") {
		t.Errorf("buildHelpContent() should contain 'Navigation'")
	}
	if !strings.Contains(content, "Tab") {
		t.Errorf("buildHelpContent() should contain 'Tab'")
	}
	if !strings.Contains(content, "Enter") {
		t.Errorf("buildHelpContent() should contain 'Enter'")
	}
}

func TestBuildHelpContent_ContainsConfiguration(t *testing.T) {
	content := buildHelpContent()
	if !strings.Contains(content, "Configuration") {
		t.Errorf("buildHelpContent() should contain 'Configuration'")
	}
	if !strings.Contains(content, "config.yaml") {
		t.Errorf("buildHelpContent() should contain 'config.yaml'")
	}
}

func TestHeader(t *testing.T) {
	h := header("Test Header")
	if !strings.Contains(h, "Test Header") {
		t.Errorf("header() should contain text, got: %s", h)
	}
	if !strings.HasSuffix(h, "\n") {
		t.Errorf("header() should end with newline")
	}
}

func TestCmd(t *testing.T) {
	tests := []struct {
		name string
		main string
		alt1 string
		alt2 string
		desc string
	}{
		{"with two aliases", "/help", "/?", ":h", "Show help"},
		{"with one alias", "/quit", "/q", "", "Quit"},
		{"no aliases", "/new", "", "", "New topic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmd(tt.main, tt.alt1, tt.alt2, tt.desc)
			if !strings.Contains(got, tt.desc) {
				t.Errorf("cmd() should contain description, got: %s", got)
			}
			if !strings.Contains(got, tt.main) {
				t.Errorf("cmd() should contain main cmd %q, got: %s", tt.main, got)
			}
		})
	}
}
