package tui

import (
	"fmt"
	"strings"
	"time"

	"dhi_harness/internal/rss"
	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// ─── Tracker Panel (left column) ───

type trackerPanel struct{}

func (t *trackerPanel) render(todoMgr *todo.Manager, taskMgr *task.Manager, rssMgr *rss.Manager) string {
	var sb strings.Builder

	activeTodos := todoMgr.ActiveCount()
	activeScheds := taskMgr.CountByStatus(task.StatusWaiting)
	rssCount := rssMgr.EntryCount()

	fmt.Fprintf(&sb, "   Todos:  %d pending\n", activeTodos)
	fmt.Fprintf(&sb, "   Sched:  %d upcoming\n", activeScheds)
	fmt.Fprintf(&sb, "   RSS:    %d entries\n", rssCount)
	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" /todo  /schedule  /rss"))

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" TRACKER ") + "\n" + sb.String(),
	)
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

// ─── RSS Panel (popup) ───

type rawItem struct {
	isSource bool
	source   rss.FeedSource
	entry    rss.FeedEntry
}

type rssPanel struct {
	feedSources   []rss.FeedSource
	feedEntries   []rss.FeedEntry
	rawItems      []rawItem
	bookmarkItems []rss.FeedEntry
	aiSummary     string
	selIdx        int
	tabIdx        int
	popupOpen     bool
	view          viewport.Model
	loading       bool
	loaded        bool
	lastFetch     time.Time
}

func (rp *rssPanel) loadData(mgr *rss.Manager) {
	rp.feedSources = mgr.ListSources()
	rp.feedEntries = mgr.ListEntries("")

	rp.rawItems = nil
	for _, src := range rp.feedSources {
		entries := mgr.ListEntries(src.ID)
		rp.rawItems = append(rp.rawItems, rawItem{isSource: true, source: src})
		for _, entry := range entries {
			rp.rawItems = append(rp.rawItems, rawItem{isSource: false, entry: entry})
		}
	}

	rp.bookmarkItems = mgr.GetBookmarks()

	maxIdx := 0
	switch rp.tabIdx {
	case 1:
		maxIdx = len(rp.rawItems) - 1
	case 2:
		maxIdx = len(rp.bookmarkItems) - 1
	}
	if rp.selIdx > maxIdx && maxIdx >= 0 {
		rp.selIdx = maxIdx
	} else if maxIdx < 0 {
		rp.selIdx = 0
	}
}

func (rp *rssPanel) renderPopup(w int) string {
	tabLabels := []string{"AI CURATED", "RAW", "BOOKMARKS"}
	var tabs strings.Builder
	for i, label := range tabLabels {
		tabStyle := lipgloss.NewStyle().Foreground(dimColor)
		if i == rp.tabIdx {
			tabStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
			tabs.WriteString(tabStyle.Render(fmt.Sprintf(" [ %s ] ", label)))
		} else {
			tabs.WriteString(tabStyle.Render(fmt.Sprintf(" %s ", label)))
		}
	}

	var helpTxt string
	switch rp.tabIdx {
	case 0:
		helpTxt = " ↑↓ nav · ←→ tab · Enter open · s/S refresh summary · ESC close "
	case 1:
		helpTxt = " ↑↓ nav · ←→ tab · Enter open · B bookmark · R remove source · s/S fetch topic · ESC close "
	case 2:
		helpTxt = " ↑↓ nav · ←→ tab · Enter open · B unbookmark · s/S fetch topic · ESC close "
	}
	help := lipgloss.NewStyle().Foreground(dimColor).Render(helpTxt)

	var content string
	switch rp.tabIdx {
	case 0:
		content = rp.renderAITab()
	case 1:
		content = rp.renderRawTab()
	case 2:
		content = rp.renderBookmarkTab()
	}

	if len(rp.rawItems) > 0 && rp.tabIdx > 0 {
		var itemLen int
		if rp.tabIdx == 1 {
			itemLen = len(rp.rawItems)
		} else {
			itemLen = len(rp.bookmarkItems)
		}
		if itemLen > 0 {
			visLines := rp.view.Height()
			yoff := rp.view.YOffset()
			selLine := rp.selIdx * 2
			if selLine > yoff+visLines-3 {
				rp.view.ScrollDown(selLine - (yoff + visLines) + 3)
			} else if selLine < yoff {
				rp.view.ScrollUp(yoff - selLine)
			}
		}
	}

	rp.view.SetContent(content)

	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" RSS FEEDS ") + "\n" +
			tabs.String() + "\n" +
			rp.view.View() + "\n" + help,
	)
}

func (rp *rssPanel) renderAITab() string {
	if rp.loading {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := frames[int(time.Now().UnixMilli()/200)%len(frames)]
		return lipgloss.NewStyle().Foreground(dimColor).Render(frame+" Fetching feeds...") + "\n"
	}
	if !rp.loaded {
		return lipgloss.NewStyle().Foreground(dimColor).Render("Open the RSS popup to fetch feeds.") + "\n"
	}
	if rp.aiSummary == "" {
		return lipgloss.NewStyle().Foreground(dimColor).Render("Generating AI summary...") + "\n"
	}
	if len(rp.feedEntries) == 0 {
		return lipgloss.NewStyle().Foreground(dimColor).Render("No entries. Add feeds with /rss add <url>") + "\n"
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("AI Curation\n\n"))

	lines := strings.Split(rp.aiSummary, "\n")
	for _, line := range lines {
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func (rp *rssPanel) renderRawTab() string {
	if len(rp.rawItems) == 0 {
		return lipgloss.NewStyle().Foreground(dimColor).Render("No feeds. Add via /rss add <url> or OPML import.") + "\n"
	}

	// Find which source is currently "open" (contains the selected item)
	openSourceID := ""
	selItem := rp.selIdx
	if selItem >= 0 && selItem < len(rp.rawItems) {
		item := rp.rawItems[selItem]
		if item.isSource {
			openSourceID = item.source.ID
		} else {
			openSourceID = item.entry.SourceID
		}
	}

	var sb strings.Builder
	n := 0
	titleMax := rp.view.Width() - 6
	if titleMax < 20 {
		titleMax = 20
	}
	for _, item := range rp.rawItems {
		selIdx := n
		n++

		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if selIdx == rp.selIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		}

		if item.isSource {
			entryCount := 0
			for _, ri := range rp.rawItems {
				if !ri.isSource && ri.entry.SourceID == item.source.ID {
					entryCount++
				}
			}
			sb.WriteString(style.Render(fmt.Sprintf("%s▶ %s (%d)", mark, item.source.Title, entryCount)) + "\n")
		} else {
			if item.entry.SourceID != openSourceID {
				continue
			}
			entry := item.entry
			bookmark := " "
			if entry.IsBookmark {
				bookmark = "🔖"
			}
			title := entry.Title
			if len(title) > titleMax {
				title = title[:titleMax-3] + "..."
			}
			timeAgo := formatTimeAgo(entry.PublishedAt)
			linkMax := rp.view.Width() - 9 - len(timeAgo)
			if linkMax < 10 {
				linkMax = 10
			}
			link := shortenURL(entry.Link, linkMax)

			sb.WriteString(style.Render(fmt.Sprintf("   %s%s %s", mark, bookmark, title)) + "\n")
			linkStyle := lipgloss.NewStyle().Foreground(dimColor)
			if selIdx == rp.selIdx {
				linkStyle = lipgloss.NewStyle().Foreground(accentColor)
			}
			sb.WriteString(linkStyle.Render(fmt.Sprintf("      %s   %s", hyperlink(entry.Link, link), timeAgo)) + "\n")
		}
	}
	return sb.String()
}

func (rp *rssPanel) renderBookmarkTab() string {
	if len(rp.bookmarkItems) == 0 {
		return lipgloss.NewStyle().Foreground(dimColor).Render("No bookmarks. Press B on an entry to bookmark.") + "\n"
	}
	var sb strings.Builder
	titleMax := rp.view.Width() - 2
	if titleMax < 20 {
		titleMax = 20
	}
	for i, entry := range rp.bookmarkItems {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if i == rp.selIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		}

		title := entry.Title
		if len(title) > titleMax {
			title = title[:titleMax-3] + "..."
		}
		sourceName := ""
		for _, src := range rp.feedSources {
			if src.ID == entry.SourceID {
				sourceName = src.Title
				break
			}
		}
		timeAgo := formatTimeAgo(entry.PublishedAt)
		linkMax := rp.view.Width() - 10 - len(sourceName) - len(timeAgo)
		if linkMax < 10 {
			linkMax = 10
		}
		link := shortenURL(entry.Link, linkMax)

		sb.WriteString(style.Render(fmt.Sprintf("  %s%s", mark, title)) + "\n")
		sourceStyle := lipgloss.NewStyle().Foreground(dimColor)
		if i == rp.selIdx {
			sourceStyle = lipgloss.NewStyle().Foreground(accentColor)
		}
		sb.WriteString(sourceStyle.Render(fmt.Sprintf("     %s  (%s)  %s", hyperlink(entry.Link, link), sourceName, timeAgo)) + "\n")
	}
	return sb.String()
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%dm", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		return fmt.Sprintf("%dh", h)
	}
	if d < 7*24*time.Hour {
		d2 := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", d2)
	}
	return t.Format("Jan 2")
}

func shortenURL(url string, max int) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "www.")
	if len(url) > max {
		url = url[:max-3] + "..."
	}
	return url
}
