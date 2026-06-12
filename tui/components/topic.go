package components

import (
	"fmt"
	"strings"

	"dhi_harness/internal/topic"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type Topic struct {
	Filter    string
	Filtered  []topic.Topic
	SelIdx    int
	RecentIdx int
	PopupOpen bool
	View      viewport.Model
}

func (t *Topic) RenderRecent(all []topic.Topic, cur *topic.Topic, focused bool, height int) string {
	var sb strings.Builder

	limit := 5
	if len(all) > limit {
		all = all[:limit]
	}

	for i, tp := range all {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(DimColor)
		if focused && i == t.RecentIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
		} else if cur != nil && tp.ID == cur.ID {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(AccentColor)
		}
		label := tp.Title
		if len(label) > 20 {
			label = label[:17] + "..."
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, label)) + "\n")
	}

	if len(all) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(DimColor).Render(" No topics") + "\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(DimColor).Render(" /topic browse all"))

	// Fill remaining height
	contentLines := strings.Count(sb.String(), "\n") + 1
	used := 7 + 9 + 9 // dhi + metrics + pipelines
	available := height - used - 2
	if fill := available - contentLines; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}

	style := BorderStyle
	if focused {
		style = FocusedBorderStyle
	}

	return style.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(" RECENT TOPICS ") + "\n" + sb.String(),
	)
}

func (t *Topic) RenderPopup(w int, cur *topic.Topic) string {
	help := lipgloss.NewStyle().Foreground(DimColor).Render(" ↑↓ navigate · Enter/Tab select · type to search · ESC close ")

	var list strings.Builder
	for i, tp := range t.Filtered {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(DimColor)
		if i == t.SelIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
		} else if cur != nil && tp.ID == cur.ID {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(AccentColor)
		}
		list.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, tp.Title)) + "\n")
	}

	content := list.String()
	if t.Filter != "" {
		content = fmt.Sprintf("Search: %s\n\n%s", t.Filter, list.String())
	}
	if len(t.Filtered) == 0 {
		content = "No matching topics."
	}

	// Ensure selection visible
	if len(t.Filtered) > 0 {
		visLines := t.View.Height()
		yoff := t.View.YOffset()
		if t.SelIdx > yoff+visLines-2 {
			t.View.ScrollDown(t.SelIdx - (yoff + visLines) + 2)
		} else if t.SelIdx < yoff {
			t.View.ScrollUp(yoff - t.SelIdx)
		}
	}

	t.View.SetContent(content)

	return FocusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(" ALL TOPICS ") + "\n" + t.View.View() + "\n" + help,
	)
}

func (t *Topic) FilterTopics(all []topic.Topic) {
	if t.Filter == "" {
		t.Filtered = all
		return
	}
	filter := strings.ToLower(t.Filter)
	t.Filtered = nil
	for _, tp := range all {
		if strings.Contains(strings.ToLower(tp.Title), filter) ||
			strings.Contains(strings.ToLower(tp.ID), filter) {
			t.Filtered = append(t.Filtered, tp)
		}
	}
}
