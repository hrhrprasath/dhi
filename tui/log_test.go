package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
)

func TestLogPanel_DefaultFields(t *testing.T) {
	l := &logPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	_ = l.entries == nil
	if l.popupOpen {
		t.Error("popupOpen should be false by default")
	}
}

func TestLogPanel_Append(t *testing.T) {
	l := &logPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	l.append("entry 1")
	l.append("entry 2")
	if len(l.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(l.entries))
	}
	if l.entries[0] != "entry 1" {
		t.Errorf("expected 'entry 1', got %q", l.entries[0])
	}
}

func TestLogPanel_AppendTruncates(t *testing.T) {
	l := &logPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	for i := 0; i < 1500; i++ {
		l.append("log line")
	}
	if len(l.entries) > 1000 {
		t.Errorf("entries should be capped at ~500, got %d", len(l.entries))
	}
}

func TestLogPanel_RenderPopupEmpty(t *testing.T) {
	l := &logPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	out := l.renderPopup(40)
	if !strings.Contains(out, "No logs yet") {
		t.Errorf("renderPopup should show 'No logs yet', got: %s", out)
	}
}

func TestLogPanel_RenderPopupWithEntries(t *testing.T) {
	l := &logPanel{
		entries: []string{"started", "completed"},
		view:    viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	out := l.renderPopup(40)
	if !strings.Contains(out, "LOGS") {
		t.Errorf("renderPopup should contain 'LOGS', got: %s", out)
	}
	if !strings.Contains(out, "started") {
		t.Errorf("renderPopup should contain 'started', got: %s", out)
	}
	if !strings.Contains(out, "completed") {
		t.Errorf("renderPopup should contain 'completed', got: %s", out)
	}
}
