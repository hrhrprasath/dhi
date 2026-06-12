package components

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

type DHIOS struct {
	Hostname  string
	Kernel    string
	UptimeSec uint64
}

func (d *DHIOS) Render() string {
	info := fmt.Sprintf("v%s\nHost: %s\nKernel: %s\nUp: %dm",
		"0.1.0",
		d.Hostname,
		d.Kernel,
		d.UptimeSec/60,
	)
	return BorderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(" DHI OS ") + "\n" +
			lipgloss.NewStyle().Foreground(DimColor).Render(info),
	)
}
