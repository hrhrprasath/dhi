package components

import (
	"fmt"

	"dhi_harness/internal/services"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

type System struct {
	Metrics      services.SystemMetrics
	RouterOnline bool
	RouterStatus string
	CPUProgress  progress.Model
	RAMProgress  progress.Model
	GPUProgress  progress.Model
}

func (s *System) Render() string {
	statusColor := GreenColor
	statusDot := "●"
	if !s.RouterOnline {
		statusColor = RedColor
	} else if s.RouterStatus == "RUNNING" || s.RouterStatus == "PROCESSING" {
		statusColor = YellowColor
	}

	cpuPct := s.Metrics.CPU / 100.0
	ramPct := float64(s.Metrics.RAMUsedMB) / float64(Max(int(s.Metrics.RAMTotalMB), 1))

	gpuLine := fmt.Sprintf("GPU: %s", s.GPUProgress.ViewAs(s.Metrics.GPUPercent/100.0))
	content := fmt.Sprintf(
		"CPU: %s\nRAM: %s\n%s\n%s %s",
		s.CPUProgress.ViewAs(cpuPct),
		s.RAMProgress.ViewAs(ramPct),
		gpuLine,
		lipgloss.NewStyle().Foreground(statusColor).Render(statusDot),
		lipgloss.NewStyle().Foreground(statusColor).Render("Ollama "+s.RouterStatus),
	)

	return BorderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(" SYSTEM METRICS ") + "\n" + content,
	)
}
