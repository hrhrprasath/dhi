package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type logPanel struct {
	entries   []string
	popupOpen bool
	view      viewport.Model
}

func (l *logPanel) renderPopup(w int) string {
	content := strings.Join(l.entries, "\n")
	if content == "" {
		content = "No logs yet..."
	}
	l.view.SetContent(content)

	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ESC to close | scroll: ↑↓ PgUp PgDown ")
	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" LOGS ") + "\n" + l.view.View() + "\n" + help,
	)
}

func (l *logPanel) append(entry string) {
	l.entries = append(l.entries, entry)
	if len(l.entries) > 1000 {
		l.entries = l.entries[len(l.entries)-500:]
	}
}
