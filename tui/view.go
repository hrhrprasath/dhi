package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *Model) View() tea.View {
	return tea.View{
		Content:   m.render(),
		AltScreen: true,
	}
}

func (m *Model) render() string {
	rw := m.rightWidth()

	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		m.renderMetrics(),
		"",
		m.renderPipelinesBlock(),
		"",
		m.renderDHI(),
		"",
		m.renderTopicListBlock(),
	)

	var mainBlock string
	switch {
	case m.logPopupVisible:
		mainBlock = m.renderLogPopup(rw)
	case m.topicPopupVisible:
		mainBlock = m.renderTopicPopup(rw)
	default:
		mainBlock = m.renderConvBlock(rw)
	}

	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		mainBlock,
		m.renderInputBlock(rw),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftPanel,
		"  ",
		rightPanel,
	)
}

func (m *Model) rightWidth() int {
	return max(m.width-30, 40)
}

// ─── Left Panel ───

func (m *Model) renderMetrics() string {
	statusColor := greenColor
	statusDot := "●"
	if !m.routerOnline {
		statusColor = redColor
		statusDot = "●"
	} else if m.routerStatus == "RUNNING" || m.routerStatus == "PROCESSING" {
		statusColor = yellowColor
	}

	cpuPct := m.metrics.CPU / 100.0
	ramPct := float64(m.metrics.RAMUsedMB) / float64(max(int(m.metrics.RAMTotalMB), 1))

	gpuLine := fmt.Sprintf("GPU: %s", m.gpuProgress.ViewAs(m.metrics.GPUPercent/100.0))
	content := fmt.Sprintf(
		"CPU: %s\nRAM: %s\n%s\n%s %s",
		m.cpuProgress.ViewAs(cpuPct),
		m.ramProgress.ViewAs(ramPct),
		gpuLine,
		lipgloss.NewStyle().Foreground(statusColor).Render(statusDot),
		lipgloss.NewStyle().Foreground(statusColor).Render("Ollama "+m.routerStatus),
	)

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" SYSTEM METRICS ") + "\n" + content,
	)
}

func (m *Model) renderTopicListBlock() string {
	var sb strings.Builder

	all := m.topicMgr.ListTopics()
	limit := 5
	if len(all) > limit {
		all = all[:limit]
	}

	cur := m.topicMgr.CurrentTopic()
	for i, t := range all {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if m.focused == focusTopics && i == m.recentSelIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		} else if cur != nil && t.ID == cur.ID {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(accentColor)
		}
		label := t.Title
		if len(label) > 20 {
			label = label[:17] + "..."
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, label)) + "\n")
	}

	if len(all) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" No topics") + "\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(" /topic browse all"))

	// Fill remaining height
	contentLines := strings.Count(sb.String(), "\n") + 1
	used := 9 + 1 + 9 + 1 + 7 + 1 // metrics + spacer + pipelines + spacer + dhi + spacer
	available := m.height - used - 2
	if fill := available - contentLines; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}

	style := borderStyle
	if m.focused == focusTopics && !m.topicPopupVisible && !m.logPopupVisible {
		style = focusedBorderStyle
	}

	return style.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" RECENT TOPICS ") + "\n" + sb.String(),
	)
}

func (m *Model) renderPipelinesBlock() string {
	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" PIPELINES ") + "\n" + m.pipelineView.View(),
	)
}

func (m *Model) renderDHI() string {
	info := fmt.Sprintf("v%s\nHost: %s\nKernel: %s\nUp: %dm",
		"0.1.0",
		m.hostname,
		m.kernel,
		m.uptimeSec/60,
	)
	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" DHI OS ") + "\n" +
			lipgloss.NewStyle().Foreground(dimColor).Render(info),
	)
}

// ─── Right Panel ───

func (m *Model) renderLogPopup(logW int) string {
	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ESC to close | scroll: ↑↓ PgUp PgDown ")
	return focusedBorderStyle.Width(logW).Render(
		lipgloss.NewStyle().Bold(true).Render(" LOGS ") + "\n" + m.logView.View() + "\n" + help,
	)
}

func (m *Model) renderTopicPopup(logW int) string {
	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ↑↓ navigate · Enter/Tab select · type to search · ESC close ")

	var list strings.Builder
	cur := m.topicMgr.CurrentTopic()
	for i, t := range m.topicFiltered {
		mark := "  "
		style := lipgloss.NewStyle().Foreground(dimColor)
		if i == m.topicSelIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		} else if cur != nil && t.ID == cur.ID {
			mark = "→ "
			style = lipgloss.NewStyle().Foreground(accentColor)
		}
		list.WriteString(style.Render(fmt.Sprintf("%s%d. %s", mark, i+1, t.Title)) + "\n")
	}

	content := list.String()
	if m.topicFilter != "" {
		content = fmt.Sprintf("Search: %s\n\n%s", m.topicFilter, list.String())
	}
	if len(m.topicFiltered) == 0 {
		content = "No matching topics."
	}

	// Scroll topicView to keep selection visible
	if len(m.topicFiltered) > 0 {
		visLines := m.topicView.Height()
		yoff := m.topicView.YOffset()
		scrollTo := m.topicSelIdx
		if scrollTo > yoff+visLines-2 {
			m.topicView.ScrollDown(scrollTo - (yoff + visLines) + 2)
		} else if scrollTo < yoff {
			m.topicView.ScrollUp(yoff - scrollTo)
		}
	}

	m.topicView.SetContent(content)

	return focusedBorderStyle.Width(logW).Render(
		lipgloss.NewStyle().Bold(true).Render(" ALL TOPICS ") + "\n" + m.topicView.View() + "\n" + help,
	)
}

func (m *Model) renderConvBlock(logW int) string {
	style := borderStyle
	if m.focused == focusConv {
		style = focusedBorderStyle
	}
	title := " CONVERSATION "
	if cur := m.topicMgr.CurrentTopic(); cur != nil {
		t := cur.Title
		if len(t) > 40 {
			t = t[:37] + "..."
		}
		title = fmt.Sprintf(" TOPIC: %s ", t)
	}
	return style.Width(logW).Render(
		lipgloss.NewStyle().Bold(true).Render(title) + "\n" + m.convView.View(),
	)
}

func (m *Model) renderInputBlock(logW int) string {
	style := borderStyle
	if m.focused == focusInput {
		style = focusedBorderStyle
	}
	onlineStr := "● OFFLINE"
	onlineColor := redColor
	if m.routerOnline {
		onlineStr = "● ONLINE"
		onlineColor = greenColor
	}
	status := lipgloss.NewStyle().Foreground(onlineColor).Render(onlineStr)

	var content string
	if len(m.slashMatches) > 0 {
		suggestions := make([]string, len(m.slashMatches))
		for i, s := range m.slashMatches {
			if i == m.slashIndex {
				suggestions[i] = lipgloss.NewStyle().Foreground(accentColor).Underline(true).Render(s)
			} else {
				suggestions[i] = lipgloss.NewStyle().Foreground(dimColor).Render(s)
			}
		}
		help := lipgloss.NewStyle().Foreground(dimColor).Render(" TAB cycle · ENTER select")
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Bottom,
				m.textInput.View(),
				"  ",
				status,
			),
			"  "+strings.Join(suggestions, "  ")+"  "+help,
		)
	} else {
		content = lipgloss.JoinHorizontal(lipgloss.Bottom,
			m.textInput.View(),
			"  ",
			status,
		)
	}
	return style.Width(logW).Render(content)
}

// ─── Content Refresh ───

func (m *Model) refreshLogContent() {
	content := strings.Join(m.logEntries, "\n")
	if content == "" {
		content = "No logs yet..."
	}
	m.logView.SetContent(content)
}

func (m *Model) refreshConvContent() {
	w := m.convView.Width()
	if w < 10 {
		w = 40
	}
	wrap := lipgloss.NewStyle().Width(w)
	var sb strings.Builder
	for _, c := range m.conversations {
		sb.WriteString("You: ")
		sb.WriteString(wrap.Render(c.Input))
		sb.WriteString("\n\nDHI: ")
		sb.WriteString(wrap.Render(c.Response))
		sb.WriteString("\n\n")
		sb.WriteString(strings.Repeat("─", w) + "\n\n")
	}
	if sb.Len() == 0 {
		sb.WriteString("Start a conversation...")
	}
	m.convView.SetContent(sb.String())
}

func (m *Model) refreshPipelineContent() {
	var sb strings.Builder
	for _, p := range m.pipelines {
		statusColor := greenColor
		if p.Status == "RUNNING" {
			statusColor = yellowColor
		} else if p.Status == "FAILED" {
			statusColor = redColor
		}
		line := lipgloss.NewStyle().Foreground(statusColor).Render(p.Status)
		sb.WriteString(fmt.Sprintf("%-15s %s\n", p.Name, line))
	}
	m.pipelineView.SetContent(sb.String())
}

func (m *Model) refreshTopicListContent() {
	// topic view is refreshed lazily on render
}

func (m *Model) refreshTopicFiltered() {
	all := m.topicMgr.ListTopics()
	if m.topicFilter == "" {
		m.topicFiltered = all
		return
	}
	filter := strings.ToLower(m.topicFilter)
	m.topicFiltered = nil
	for _, t := range all {
		if strings.Contains(strings.ToLower(t.Title), filter) ||
			strings.Contains(strings.ToLower(t.ID), filter) {
			m.topicFiltered = append(m.topicFiltered, t)
		}
	}
}
