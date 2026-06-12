package components

import "charm.land/lipgloss/v2"

var (
	AccentColor = lipgloss.Color("#FFA500")
	DimColor    = lipgloss.Color("#888888")
	GreenColor  = lipgloss.Color("#00FF00")
	YellowColor = lipgloss.Color("#FFFF00")
	RedColor    = lipgloss.Color("#FF0000")

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444"))
	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(AccentColor)
)

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
