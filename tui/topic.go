package tui

import (
	"fmt"
	"strings"

	"github.com/hrhrprasath/dhi/internal/topic"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type topicPanel struct {
	filter    string
	filtered  []topic.Topic
	selIdx    int
	recentIdx int
	popupOpen bool
	view      viewport.Model
}

func (t *topicPanel) renderRecent(all []topic.Topic, cur *topic.Topic, focused bool, height int) string {
	var sb strings.Builder

	limit := 17
	if len(all) > limit {
		all = all[:limit]
	}

	for i, tp := range all {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if focused && i == t.recentIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		} else if cur != nil && tp.ID == cur.ID {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(accentColor)
		}
		label := tp.Title
		if len(label) > 20 {
			label = label[:17] + "..."
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, label)) + "\n")
	}

	if len(all) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" No topics") + "\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" /topic browse all"))

	contentLines := strings.Count(sb.String(), "\n") + 1
	used := 7 + 9 + 9 + 7
	available := height - used - 2
	if fill := available - contentLines; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}

	style := borderStyle
	if focused {
		style = focusedBorderStyle
	}

	return style.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" RECENT TOPICS ") + "\n" + sb.String(),
	)
}

func (t *topicPanel) renderPopup(w int, cur *topic.Topic) string {
	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ↑↓ navigate · Enter/Tab select · type to search · ESC close ")

	var list strings.Builder
	for i, tp := range t.filtered {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if i == t.selIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		} else if cur != nil && tp.ID == cur.ID {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(accentColor)
		}
		list.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, tp.Title)) + "\n")
	}

	content := list.String()
	if t.filter != "" {
		content = fmt.Sprintf("Search: %s\n\n%s", t.filter, list.String())
	}
	if len(t.filtered) == 0 {
		content = "No matching topics."
	}

	if len(t.filtered) > 0 {
		visLines := t.view.Height()
		yoff := t.view.YOffset()
		if t.selIdx > yoff+visLines-2 {
			t.view.ScrollDown(t.selIdx - (yoff + visLines) + 2)
		} else if t.selIdx < yoff {
			t.view.ScrollUp(yoff - t.selIdx)
		}
	}

	t.view.SetContent(content)

	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" ALL TOPICS ") + "\n" + t.view.View() + "\n" + help,
	)
}

func (t *topicPanel) filterTopics(all []topic.Topic) {
	if t.filter == "" {
		t.filtered = all
		return
	}
	filter := strings.ToLower(t.filter)
	t.filtered = nil
	for _, tp := range all {
		if strings.Contains(strings.ToLower(tp.Title), filter) ||
			strings.Contains(strings.ToLower(tp.ID), filter) {
			t.filtered = append(t.filtered, tp)
		}
	}
}
