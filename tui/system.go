package tui

import (
	"fmt"

	"dhi_harness/internal/services"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

type systemPanel struct {
	metrics      services.SystemMetrics
	routerOnline bool
	routerStatus string
	cpuProgress  progress.Model
	ramProgress  progress.Model
	gpuProgress  progress.Model
}

func (s *systemPanel) render() string {
	statusColor := greenColor
	statusDot := "●"
	if !s.routerOnline {
		statusColor = redColor
	} else if s.routerStatus == "RUNNING" || s.routerStatus == "PROCESSING" {
		statusColor = yellowColor
	}

	cpuPct := s.metrics.CPU / 100.0
	ramPct := float64(s.metrics.RAMUsedMB) / float64(max(int(s.metrics.RAMTotalMB), 1))

	gpuLine := fmt.Sprintf("GPU: %s", s.gpuProgress.ViewAs(s.metrics.GPUPercent/100.0))
	content := fmt.Sprintf(
		"CPU: %s\nRAM: %s\n%s\n%s %s",
		s.cpuProgress.ViewAs(cpuPct),
		s.ramProgress.ViewAs(ramPct),
		gpuLine,
		lipgloss.NewStyle().Foreground(statusColor).Render(statusDot),
		lipgloss.NewStyle().Foreground(statusColor).Render("AI "+s.routerStatus),
	)

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" SYSTEM METRICS ") + "\n" + content,
	)
}
