package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

type dhiOS struct {
	hostname  string
	kernel    string
	uptimeSec uint64
}

func (d *dhiOS) render() string {
	info := fmt.Sprintf("v%s\nHost: %s\nKernel: %s\nUp: %dm",
		"0.1.0",
		d.hostname,
		d.kernel,
		d.uptimeSec/60,
	)
	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" DHI OS ") + "\n" +
			lipgloss.NewStyle().Foreground(dimColor).Render(info),
	)
}
