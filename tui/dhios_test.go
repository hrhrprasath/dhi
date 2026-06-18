package tui

import (
	"strings"
	"testing"
)

func TestDhiOS_RenderContainsDHI(t *testing.T) {
	d := &dhiOS{
		hostname:  "testhost",
		kernel:    "24.0.0",
		uptimeSec: 3600,
	}
	out := d.render()
	if !strings.Contains(out, "DHI OS") {
		t.Errorf("render() should contain 'DHI OS', got: %s", out)
	}
}

func TestDhiOS_RenderContainsHost(t *testing.T) {
	d := &dhiOS{
		hostname:  "my-machine",
		kernel:    "23.0.0",
		uptimeSec: 7200,
	}
	out := d.render()
	if !strings.Contains(out, "my-machine") {
		t.Errorf("render() should contain hostname, got: %s", out)
	}
}

func TestDhiOS_RenderContainsKernel(t *testing.T) {
	d := &dhiOS{
		hostname:  "h",
		kernel:    "25.1.0",
		uptimeSec: 0,
	}
	out := d.render()
	if !strings.Contains(out, "25.1.0") {
		t.Errorf("render() should contain kernel, got: %s", out)
	}
}

func TestDhiOS_RenderUptimeMinutes(t *testing.T) {
	d := &dhiOS{
		hostname:  "h",
		kernel:    "k",
		uptimeSec: 3660,
	}
	out := d.render()
	if !strings.Contains(out, "61m") {
		t.Errorf("render() should show 61m for 3660s, got: %s", out)
	}
}

func TestDhiOS_RenderContainsVersion(t *testing.T) {
	d := &dhiOS{
		hostname:  "h",
		kernel:    "k",
		uptimeSec: 0,
	}
	out := d.render()
	if !strings.Contains(out, "v0.1.0") {
		t.Errorf("render() should contain v0.1.0, got: %s", out)
	}
}
