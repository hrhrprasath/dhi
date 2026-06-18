package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type conversationEntry struct {
	input    string
	response string
	thinking bool
}

type commandCenter struct {
	conversations []conversationEntry
	thinking      int
	spinner       spinner.Model
	textInput     textinput.Model
	slashMatches  []string
	slashIndex    int
	lastSubmitted string
	convView      viewport.Model
}

func (cc *commandCenter) renderConv(w int, focused bool, topicTitle string) string {
	style := borderStyle
	if focused {
		style = focusedBorderStyle
	}
	title := " COMMAND CENTER "
	if topicTitle != "" {
		if len(topicTitle) > 32 {
			topicTitle = topicTitle[:29] + "..."
		}
		title = fmt.Sprintf(" TOPIC: %s ", topicTitle)
	}
	if len(cc.conversations) == 0 {
		cc.convView.SetContent("Start a conversation...")
	} else {
		wrap := lipgloss.NewStyle().Width(w - 2)
		var sb strings.Builder
		for _, c := range cc.conversations {
			sb.WriteString("You: ")
			sb.WriteString(wrap.Render(c.input))
			sb.WriteString("\n\nDHI: ")
			response := c.response
			if !c.thinking {
				response = linkify(response)
			}
			sb.WriteString(wrap.Render(response))
			sb.WriteString("\n\n")
			sb.WriteString(strings.Repeat("─", w-2) + "\n\n")
		}
		cc.convView.SetContent(sb.String())
	}
	return style.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(title) + "\n" + cc.convView.View(),
	)
}

func (cc *commandCenter) renderInput(w int, routerOnline bool, focused bool, modelName string) string {
	style := borderStyle
	if focused {
		style = focusedBorderStyle
	}
	var statusStr string
	var statusColor = redColor
	if !routerOnline {
		statusStr = "● OFFLINE"
	} else if modelName == "" {
		statusStr = "● NO MODEL"
	} else {
		statusStr = "● " + modelName
		statusColor = greenColor
	}
	status := lipgloss.NewStyle().Foreground(statusColor).Render(statusStr)

	var content string
	if len(cc.slashMatches) > 0 {
		suggestions := make([]string, len(cc.slashMatches))
		for i, s := range cc.slashMatches {
			if i == cc.slashIndex {
				suggestions[i] = lipgloss.NewStyle().Foreground(accentColor).Underline(true).Render(s)
			} else {
				suggestions[i] = lipgloss.NewStyle().Foreground(dimColor).Render(s)
			}
		}
		help := lipgloss.NewStyle().Foreground(dimColor).Render(" TAB cycle · ENTER select")
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Bottom,
				cc.textInput.View(),
				"  ",
				status,
			),
			"  "+strings.Join(suggestions, "  ")+"  "+help,
		)
	} else {
		content = lipgloss.JoinHorizontal(lipgloss.Bottom,
			cc.textInput.View(),
			"  ",
			status,
		)
	}
	return style.Width(w).Render(content)
}

func (cc *commandCenter) setThinking(input string) {
	cc.conversations = append(cc.conversations, conversationEntry{
		input:    input,
		response: cc.spinner.View(),
		thinking: true,
	})
	cc.thinking++
}

func (cc *commandCenter) setResponse(input, response string) bool {
	for i := len(cc.conversations) - 1; i >= 0; i-- {
		if cc.conversations[i].input == input && cc.conversations[i].thinking {
			cc.conversations[i].response = response
			cc.conversations[i].thinking = false
			cc.thinking--
			return true
		}
	}
	return false
}

func (cc *commandCenter) updateSpinnerResponse() bool {
	if cc.thinking == 0 {
		return false
	}
	for i := len(cc.conversations) - 1; i >= 0; i-- {
		if cc.conversations[i].thinking {
			cc.conversations[i].response = cc.spinner.View()
			return true
		}
	}
	return false
}

func (cc *commandCenter) setConversationsFromTurns(turns []conversationEntry) {
	cc.conversations = turns
	cc.thinking = 0
}
