package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

type welcomePanel struct {
	show      bool
	status    string
	checked   bool
	startTime time.Time
	minShow   time.Duration
}

func (w *welcomePanel) render(width, height int) string {
	art := []string{
		"██████╗  ██╗  ██╗ ██╗",
		"██╔══██╗ ██║  ██║ ╚═╝",
		"██║  ██║ ███████║ ██╗",
		"██║  ██║ ██╔══██║ ██║",
		"██████╔╝ ██║  ██║ ██║",
		"╚═════╝  ╚═╝  ╚═╝ ╚═╝",
	}

	artStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	dim := lipgloss.NewStyle().Foreground(dimColor)

	var statusDot string
	var statusStyle lipgloss.Style
	switch w.status {
	case "READY":
		statusDot = "●"
		statusStyle = lipgloss.NewStyle().Foreground(greenColor)
	case "OFFLINE":
		statusDot = "●"
		statusStyle = lipgloss.NewStyle().Foreground(redColor)
	default:
		statusDot = "●"
		statusStyle = lipgloss.NewStyle().Foreground(yellowColor)
	}

	statusText := fmt.Sprintf("%s AI %s", statusDot, w.status)

	var lines []string
	for _, l := range art {
		lines = append(lines, artStyle.Render(l))
	}
	lines = append(lines, "")
	lines = append(lines, dim.Render("Terminal AI OS"))
	lines = append(lines, "")
	lines = append(lines, statusStyle.Render(statusText))
	lines = append(lines, "")
	lines = append(lines, dim.Render("v0.1.0"))
	lines = append(lines, "")
	lines = append(lines, dim.Render("Press any key to continue"))

	content := strings.Join(lines, "\n")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}
