package components

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type Log struct {
	Entries   []string
	PopupOpen bool
	View      viewport.Model
}

func (l *Log) RenderPopup(w int) string {
	content := strings.Join(l.Entries, "\n")
	if content == "" {
		content = "No logs yet..."
	}
	l.View.SetContent(content)

	help := lipgloss.NewStyle().Foreground(DimColor).Render(" ESC to close | scroll: ↑↓ PgUp PgDown ")
	return FocusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(" LOGS ") + "\n" + l.View.View() + "\n" + help,
	)
}

func (l *Log) Append(entry string) {
	l.Entries = append(l.Entries, entry)
	if len(l.Entries) > 1000 {
		l.Entries = l.Entries[len(l.Entries)-500:]
	}
}
