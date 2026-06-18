package tui

import (
	"strings"
	"testing"
	"time"
)

func TestWelcomePanel_RenderNonEmpty(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "READY",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if out == "" {
		t.Error("render() returned empty string")
	}
}

func TestWelcomePanel_RenderContainsASCIIArt(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "READY",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if !strings.Contains(out, "██") {
		t.Errorf("render() should contain ASCII art blocks, got: %s", out)
	}
}

func TestWelcomePanel_RenderStatusReady(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "READY",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if !strings.Contains(out, "READY") {
		t.Errorf("render() should contain 'READY', got: %s", out)
	}
}

func TestWelcomePanel_RenderStatusOffline(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "OFFLINE",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if !strings.Contains(out, "OFFLINE") {
		t.Errorf("render() should contain 'OFFLINE', got: %s", out)
	}
}

func TestWelcomePanel_RenderStatusUnknown(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "STARTING",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if !strings.Contains(out, "STARTING") {
		t.Errorf("render() should contain 'STARTING', got: %s", out)
	}
}

func TestWelcomePanel_RenderContainsVersion(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "READY",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if !strings.Contains(out, "v0.1.0") {
		t.Errorf("render() should contain 'v0.1.0', got: %s", out)
	}
}

func TestWelcomePanel_RenderContainsPressAnyKey(t *testing.T) {
	w := &welcomePanel{
		show:      true,
		status:    "READY",
		startTime: time.Now(),
	}
	out := w.render(80, 24)
	if !strings.Contains(out, "Press any key") {
		t.Errorf("render() should contain 'Press any key', got: %s", out)
	}
}
