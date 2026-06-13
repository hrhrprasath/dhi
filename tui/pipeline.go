package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type pipelineEntry struct {
	name   string
	status string
}

type pipelinePanel struct {
	entries []pipelineEntry
	view    viewport.Model
}

func (p *pipelinePanel) render() string {
	var sb strings.Builder
	for _, pe := range p.entries {
		statusColor := greenColor
		if pe.status == "RUNNING" {
			statusColor = yellowColor
		} else if pe.status == "FAILED" {
			statusColor = redColor
		}
		line := lipgloss.NewStyle().Foreground(statusColor).Render(pe.status)
		sb.WriteString(fmt.Sprintf("%-15s %s\n", pe.name, line))
	}
	p.view.SetContent(sb.String())

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" PIPELINES ") + "\n" + p.view.View(),
	)
}
