package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type PipelineEntry struct {
	Name   string
	Status string
}

type Pipeline struct {
	Entries []PipelineEntry
	View    viewport.Model
}

func (p *Pipeline) Render() string {
	var sb strings.Builder
	for _, pe := range p.Entries {
		statusColor := GreenColor
		if pe.Status == "RUNNING" {
			statusColor = YellowColor
		} else if pe.Status == "FAILED" {
			statusColor = RedColor
		}
		line := lipgloss.NewStyle().Foreground(statusColor).Render(pe.Status)
		sb.WriteString(fmt.Sprintf("%-15s %s\n", pe.Name, line))
	}
	p.View.SetContent(sb.String())

	return BorderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(" PIPELINES ") + "\n" + p.View.View(),
	)
}
