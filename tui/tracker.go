package tui

import (
	"fmt"
	"strings"

	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// ─── Tracker Panel (left column) ───

type trackerPanel struct{}

func (t *trackerPanel) render(todoMgr *todo.Manager, taskMgr *task.Manager) string {
	var sb strings.Builder

	activeTodos := todoMgr.ActiveCount()
	activeScheds := taskMgr.CountByStatus(task.StatusWaiting)

	sb.WriteString(fmt.Sprintf(" ☐ Todos:  %d pending\n", activeTodos))
	sb.WriteString(fmt.Sprintf(" ☐ Sched:  %d upcoming\n", activeScheds))
	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" /todo  /schedule"))

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" TRACKER ") + "\n" + sb.String(),
	)
}

func (t *trackerPanel) height() int {
	return 5
}

// ─── Todo Panel (popup) ───

type todoPanel struct {
	items     []todo.Item
	selIdx    int
	tabIdx    int // 0 = active, 1 = completed
	popupOpen bool
	view      viewport.Model
}

func (tp *todoPanel) loadItems(mgr *todo.Manager) {
	all := mgr.ListItems()
	tp.items = nil
	for _, it := range all {
		if (tp.tabIdx == 0 && !it.Done) || (tp.tabIdx == 1 && it.Done) {
			tp.items = append(tp.items, it)
		}
	}
	if tp.selIdx >= len(tp.items) {
		tp.selIdx = 0
	}
}

func (tp *todoPanel) renderPopup(w int) string {
	tabs := "[ ACTIVE ]"
	if tp.tabIdx == 1 {
		tabs = "[ COMPLETED ]"
	}

	var helpTxt string
	if tp.tabIdx == 0 {
		helpTxt = " ↑↓ nav · ←→ tab · R remove · D done · ESC close "
	} else {
		helpTxt = " ↑↓ nav · ←→ tab · R remove · D undo · ESC close "
	}
	help := lipgloss.NewStyle().Foreground(dimColor).Render(helpTxt)

	var list strings.Builder
	for i, it := range tp.items {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if i == tp.selIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		}
		status := "☐"
		if it.Done {
			status = "☑"
		}
		label := it.Text
		if len(label) > 50 {
			label = label[:47] + "..."
		}
		list.WriteString(style.Render(fmt.Sprintf("%s%s %s", mark, status, label)) + "\n")
	}

	content := list.String()
	if len(tp.items) == 0 {
		if tp.tabIdx == 0 {
			content = lipgloss.NewStyle().Foreground(dimColor).Render("No active todos.") + "\n"
		} else {
			content = lipgloss.NewStyle().Foreground(dimColor).Render("No completed todos.") + "\n"
		}
	}

	if len(tp.items) > 0 {
		visLines := tp.view.Height()
		yoff := tp.view.YOffset()
		if tp.selIdx > yoff+visLines-2 {
			tp.view.ScrollDown(tp.selIdx - (yoff + visLines) + 2)
		} else if tp.selIdx < yoff {
			tp.view.ScrollUp(yoff - tp.selIdx)
		}
	}

	tp.view.SetContent(content)

	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" TODOS ") + "\n" +
			lipgloss.NewStyle().Foreground(accentColor).Render(tabs) + "\n" +
			tp.view.View() + "\n" + help,
	)
}

// ─── Schedule Panel (popup) ───

type schedulePanel struct {
	items     []task.Task
	selIdx    int
	tabIdx    int // 0 = scheduled, 1 = completed
	popupOpen bool
	view      viewport.Model
}

func (sp *schedulePanel) loadItems(mgr *task.Manager) {
	if sp.tabIdx == 0 {
		sp.items = mgr.ListByStatus(task.StatusWaiting)
	} else {
		sp.items = mgr.ListByStatus(task.StatusCompleted, task.StatusFailed)
	}
	if sp.selIdx >= len(sp.items) {
		sp.selIdx = 0
	}
}

func (sp *schedulePanel) renderPopup(w int) string {
	tabs := "[ SCHEDULED ]"
	if sp.tabIdx == 1 {
		tabs = "[ COMPLETED ]"
	}

	var helpTxt string
	if sp.tabIdx == 0 {
		helpTxt = " ↑↓ nav · ←→ tab · R remove · ESC close "
	} else {
		helpTxt = " ↑↓ nav · ←→ tab · R remove · D reset · ESC close "
	}
	help := lipgloss.NewStyle().Foreground(dimColor).Render(helpTxt)

	var list strings.Builder
	for i, t := range sp.items {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if i == sp.selIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		}
		statusColor := dimColor
		if t.Status == task.StatusFailed {
			statusColor = redColor
		}
		when := ""
		if t.ScheduledFor != nil {
			when = t.ScheduledFor.Format("15:04")
		}
		label := t.RawInput
		if len(label) > 35 {
			label = label[:32] + "..."
		}
		line := fmt.Sprintf("%s%s", mark, label)
		if when != "" {
			line += fmt.Sprintf("  (%s)", when)
		}
		if t.Status == task.StatusFailed {
			line += " FAILED"
		}
		list.WriteString(style.Foreground(statusColor).Render(line) + "\n")
	}

	content := list.String()
	if len(sp.items) == 0 {
		if sp.tabIdx == 0 {
			content = lipgloss.NewStyle().Foreground(dimColor).Render("No upcoming schedules.") + "\n"
		} else {
			content = lipgloss.NewStyle().Foreground(dimColor).Render("No completed schedules.") + "\n"
		}
	}

	if len(sp.items) > 0 {
		visLines := sp.view.Height()
		yoff := sp.view.YOffset()
		if sp.selIdx > yoff+visLines-2 {
			sp.view.ScrollDown(sp.selIdx - (yoff + visLines) + 2)
		} else if sp.selIdx < yoff {
			sp.view.ScrollUp(yoff - sp.selIdx)
		}
	}

	sp.view.SetContent(content)

	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" SCHEDULES ") + "\n" +
			lipgloss.NewStyle().Foreground(accentColor).Render(tabs) + "\n" +
			sp.view.View() + "\n" + help,
	)
}
