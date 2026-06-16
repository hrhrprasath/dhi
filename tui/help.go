package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type helpPanel struct {
	popupOpen bool
	view      viewport.Model
	content   string
}

func newHelpPanel() helpPanel {
	return helpPanel{
		view: viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
}

func (hp *helpPanel) renderPopup(w int) string {
	hp.view.SetContent(hp.content)

	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ESC to close | scroll: ↑↓ PgUp PgDown ")
	return focusedBorderStyle.Width(w).Render(
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(" HELP ") + "\n" + hp.view.View() + "\n" + help,
	)
}

func buildHelpContent() string {
	var sb strings.Builder

	sb.WriteString(header("Commands"))
	sb.WriteString(cmd("/help", "/?", ":h", "Show this help screen"))
	sb.WriteString(cmd("/quit", "/q", ":q", "Quit the application"))
	sb.WriteString(cmd("/topic", "/topics", "", "Browse and switch conversation topics"))
	sb.WriteString(cmd("/model", "/models", "", "Browse and switch AI models"))
	sb.WriteString(cmd("/log", "/logs", "/l", "View application logs"))
	sb.WriteString(cmd("/new", "", "", "Create a new conversation topic"))
	sb.WriteString(cmd("/fork", "", "", "Fork the current topic"))
	sb.WriteString(cmd("/resume", "", "", "Switch to the previous topic"))
	sb.WriteString(cmd("/good", "", "", "Rate last response positively"))
	sb.WriteString(cmd("/bad", "", "", "Rate last response negatively"))
	sb.WriteString("\n")

	sb.WriteString(header("Navigation"))
	sb.WriteString("Tab        Cycle focus between panels\n")
	sb.WriteString("Enter      Submit input / select item\n")
	sb.WriteString("↑/↓        Navigate lists / scroll\n")
	sb.WriteString("PgUp/PgDn  Scroll faster\n")
	sb.WriteString("ESC        Close popup / cancel\n")
	sb.WriteString("\n")

	sb.WriteString(header("Configuration"))
	sb.WriteString("See config/config.yaml for AI provider settings\n")
	sb.WriteString("  provider, base_url, api_key, model, max_tokens, temperature\n")
	sb.WriteString("  Supports: ollama, groq, openrouter, openai, vllm, nim, custom\n")

	return sb.String()
}

func header(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(s) + "\n"
}

func cmd(main, alt1, alt2, desc string) string {
	aliases := main
	if alt1 != "" {
		aliases += ", " + alt1
	}
	if alt2 != "" {
		aliases += ", " + alt2
	}
	return fmt.Sprintf("%-20s %s\n", aliases, desc)
}
