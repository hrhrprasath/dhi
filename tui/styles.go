package tui

import "charm.land/lipgloss/v2"

var (
	accentColor = lipgloss.Color("#FFA500")
	dimColor    = lipgloss.Color("#888888")
	greenColor  = lipgloss.Color("#00FF00")
	yellowColor = lipgloss.Color("#FFFF00")
	redColor    = lipgloss.Color("#FF0000")

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444"))
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)
)

func SetAccentColor(hex string) {
	if hex == "" {
		hex = "#FFA500"
	}
	accentColor = lipgloss.Color(hex)
	focusedBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
