package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type ConversationEntry struct {
	Input    string
	Response string
	Thinking bool
}

type CommandCenter struct {
	Conversations []ConversationEntry
	Thinking      int
	Spinner       spinner.Model
	TextInput     textinput.Model
	SlashMatches  []string
	SlashIndex    int
	LastSubmitted string
	LastInput     string
	LastIntent    interface{}
	ConvView      viewport.Model
}

func NewCommandCenter(spinnerModel spinner.Model, ti textinput.Model, cv viewport.Model) CommandCenter {
	return CommandCenter{
		Spinner:   spinnerModel,
		TextInput: ti,
		ConvView:  cv,
	}
}

func (cc *CommandCenter) RenderConv(w int, focused bool, topicTitle string) string {
	style := BorderStyle
	if focused {
		style = FocusedBorderStyle
	}
	title := " COMMAND CENTER "
	if topicTitle != "" {
		if len(topicTitle) > 32 {
			topicTitle = topicTitle[:29] + "..."
		}
		title = fmt.Sprintf(" TOPIC: %s ", topicTitle)
	}
	if len(cc.Conversations) == 0 {
		cc.ConvView.SetContent("Start a conversation...")
	} else {
		wrap := lipgloss.NewStyle().Width(w - 2)
		var sb strings.Builder
		for _, c := range cc.Conversations {
			sb.WriteString("You: ")
			sb.WriteString(wrap.Render(c.Input))
			sb.WriteString("\n\nDHI: ")
			sb.WriteString(wrap.Render(c.Response))
			sb.WriteString("\n\n")
			sb.WriteString(strings.Repeat("─", w-2) + "\n\n")
		}
		cc.ConvView.SetContent(sb.String())
	}
	return style.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(title) + "\n" + cc.ConvView.View(),
	)
}

func (cc *CommandCenter) RenderInput(w int, routerOnline bool, focused bool) string {
	style := BorderStyle
	if focused {
		style = FocusedBorderStyle
	}
	onlineStr := "● OFFLINE"
	onlineColor := RedColor
	if routerOnline {
		onlineStr = "● ONLINE"
		onlineColor = GreenColor
	}
	status := lipgloss.NewStyle().Foreground(onlineColor).Render(onlineStr)

	var content string
	if len(cc.SlashMatches) > 0 {
		suggestions := make([]string, len(cc.SlashMatches))
		for i, s := range cc.SlashMatches {
			if i == cc.SlashIndex {
				suggestions[i] = lipgloss.NewStyle().Foreground(AccentColor).Underline(true).Render(s)
			} else {
				suggestions[i] = lipgloss.NewStyle().Foreground(DimColor).Render(s)
			}
		}
		help := lipgloss.NewStyle().Foreground(DimColor).Render(" TAB cycle · ENTER select")
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Bottom,
				cc.TextInput.View(),
				"  ",
				status,
			),
			"  "+strings.Join(suggestions, "  ")+"  "+help,
		)
	} else {
		content = lipgloss.JoinHorizontal(lipgloss.Bottom,
			cc.TextInput.View(),
			"  ",
			status,
		)
	}
	return style.Width(w).Render(content)
}

func (cc *CommandCenter) SetThinking(input string) {
	cc.Conversations = append(cc.Conversations, ConversationEntry{
		Input:    input,
		Response: cc.Spinner.View(),
		Thinking: true,
	})
	cc.Thinking++
}

func (cc *CommandCenter) SetResponse(input, response string) bool {
	for i := len(cc.Conversations) - 1; i >= 0; i-- {
		if cc.Conversations[i].Input == input && cc.Conversations[i].Thinking {
			cc.Conversations[i].Response = response
			cc.Conversations[i].Thinking = false
			cc.Thinking--
			return true
		}
	}
	return false
}

func (cc *CommandCenter) UpdateSpinnerResponse() bool {
	if cc.Thinking == 0 {
		return false
	}
	for i := len(cc.Conversations) - 1; i >= 0; i-- {
		if cc.Conversations[i].Thinking {
			cc.Conversations[i].Response = cc.Spinner.View()
			return true
		}
	}
	return false
}

func (cc *CommandCenter) SetConversationsFromTurns(turns []ConversationEntry) {
	cc.Conversations = turns
	cc.Thinking = 0
}

func (cc *CommandCenter) ConvHeight(totalHeight int) int {
	return Max(totalHeight-7, 3)
}

func (cc *CommandCenter) LogHeight(convH int) int {
	return Max(convH-1, 6)
}
