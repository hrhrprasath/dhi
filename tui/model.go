package tui

import (
	"fmt"
	"strings"

	"dhi_harness/internal/settings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type modelPanel struct {
	filter    string
	filtered  []settings.ModelInfo
	selIdx    int
	recentIdx int
	popupOpen bool
	view      viewport.Model
}

func (mp *modelPanel) renderRecent(cur *settings.ModelInfo, focused bool, height int) string {
	var sb strings.Builder

	limit := 5
	all := mp.filtered
	if len(all) > limit {
		all = all[:limit]
	}

	for i, mdl := range all {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if focused && i == mp.recentIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		} else if cur != nil && mdl.Name == cur.Name {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(accentColor)
		}
		label := mdl.Name
		if len(label) > 20 {
			label = label[:17] + "..."
		}
		var ctxStr string
		if mdl.ContextLen > 0 {
			ctxStr = fmt.Sprintf("   %dK ctx", mdl.ContextLen/1024)
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, label)) + "\n")
		if ctxStr != "" {
			sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(ctxStr) + "\n")
		}
	}

	if len(all) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(redColor).Render(" NO MODEL") + "\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" /model browse all"))

	contentLines := strings.Count(sb.String(), "\n") + 1
	used := 7 + 9 + 9 + 4 + 6
	available := height - used - 2
	if fill := available - contentLines; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}

	style := borderStyle
	if focused {
		style = focusedBorderStyle
	}

	return style.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" MODELS ") + "\n" + sb.String(),
	)
}

func (mp *modelPanel) renderPopup(w int, cur *settings.ModelInfo) string {
	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ↑↓ navigate · Enter/Tab select · type to search · ESC close ")

	var list strings.Builder
	for i, mdl := range mp.filtered {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if i == mp.selIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		} else if cur != nil && mdl.Name == cur.Name {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(accentColor)
		}
		sizeStr := formatSize(mdl.Size)
		ctxStr := "?"
		if mdl.ContextLen > 0 {
			ctxStr = fmt.Sprintf("%dK", mdl.ContextLen/1024)
		}
		line := fmt.Sprintf("%s%d. %-24s %6s  %s ctx",
			mark, i+1, mdl.Name, sizeStr, ctxStr)
		list.WriteString(style.Render(line) + "\n")
	}

	content := list.String()
	if mp.filter != "" {
		content = fmt.Sprintf("Search: %s\n\n%s", mp.filter, list.String())
	}
	if len(mp.filtered) == 0 {
		content = "No matching models."
	}

	if len(mp.filtered) > 0 {
		visLines := mp.view.Height()
		yoff := mp.view.YOffset()
		if mp.selIdx > yoff+visLines-2 {
			mp.view.ScrollDown(mp.selIdx - (yoff + visLines) + 2)
		} else if mp.selIdx < yoff {
			mp.view.ScrollUp(yoff - mp.selIdx)
		}
	}

	mp.view.SetContent(content)

	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" ALL MODELS ") + "\n" + mp.view.View() + "\n" + help,
	)
}

func (mp *modelPanel) filterModels(all []settings.ModelInfo) {
	if mp.filter == "" {
		mp.filtered = all
		return
	}
	filter := strings.ToLower(mp.filter)
	mp.filtered = nil
	for _, mdl := range all {
		if strings.Contains(strings.ToLower(mdl.Name), filter) {
			mp.filtered = append(mp.filtered, mdl)
		}
	}
}

func formatSize(bytes int64) string {
	if bytes >= 1_000_000_000 {
		return fmt.Sprintf("%.1fGB", float64(bytes)/1_000_000_000)
	}
	return fmt.Sprintf("%.0fMB", float64(bytes)/1_000_000)
}
