package tui

import (
	"strings"
	"testing"

	"github.com/hrhrprasath/dhi/internal/services"

	"charm.land/bubbles/v2/progress"
)

func TestSystemPanel_DefaultFields(t *testing.T) {
	s := &systemPanel{
		metrics:      services.SystemMetrics{},
		routerOnline: false,
		routerStatus: "",
		cpuProgress:  progress.New(progress.WithWidth(21)),
		ramProgress:  progress.New(progress.WithWidth(21)),
		gpuProgress:  progress.New(progress.WithWidth(21)),
	}
	if s.routerOnline {
		t.Error("routerOnline should be false by default")
	}
}

func TestSystemPanel_RenderContainsSystemMetrics(t *testing.T) {
	s := &systemPanel{
		metrics: services.SystemMetrics{
			CPU:        50.0,
			RAMUsedMB:  4096,
			RAMTotalMB: 16384,
			GPUPercent: 30.0,
		},
		routerOnline: true,
		routerStatus: "RUNNING",
		cpuProgress:  progress.New(progress.WithWidth(21)),
		ramProgress:  progress.New(progress.WithWidth(21)),
		gpuProgress:  progress.New(progress.WithWidth(21)),
	}
	out := s.render()
	if !strings.Contains(out, "SYSTEM METRICS") {
		t.Errorf("render() should contain 'SYSTEM METRICS', got: %s", out)
	}
}

func TestSystemPanel_RenderContainsCPU(t *testing.T) {
	s := &systemPanel{
		metrics: services.SystemMetrics{
			CPU:        75.0,
			RAMUsedMB:  2048,
			RAMTotalMB: 8192,
			GPUPercent: 60.0,
		},
		routerOnline: true,
		routerStatus: "IDLE",
		cpuProgress:  progress.New(progress.WithWidth(21)),
		ramProgress:  progress.New(progress.WithWidth(21)),
		gpuProgress:  progress.New(progress.WithWidth(21)),
	}
	out := s.render()
	if !strings.Contains(out, "CPU") {
		t.Errorf("render() should contain 'CPU', got: %s", out)
	}
	if !strings.Contains(out, "RAM") {
		t.Errorf("render() should contain 'RAM', got: %s", out)
	}
	if !strings.Contains(out, "GPU") {
		t.Errorf("render() should contain 'GPU', got: %s", out)
	}
}

func TestSystemPanel_RenderRouterStatus(t *testing.T) {
	tests := []struct {
		name   string
		online bool
		status string
	}{
		{"online running", true, "RUNNING"},
		{"online idle", true, "IDLE"},
		{"offline", false, "OFFLINE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &systemPanel{
				metrics:      services.SystemMetrics{CPU: 0, RAMUsedMB: 0, RAMTotalMB: 1, GPUPercent: 0},
				routerOnline: tt.online,
				routerStatus: tt.status,
				cpuProgress:  progress.New(progress.WithWidth(21)),
				ramProgress:  progress.New(progress.WithWidth(21)),
				gpuProgress:  progress.New(progress.WithWidth(21)),
			}
			out := s.render()
			if !strings.Contains(out, "AI "+tt.status) {
				t.Errorf("render() should contain status %q, got: %s", tt.status, out)
			}
		})
	}
}
