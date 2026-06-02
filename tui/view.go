package tui

import (
	"fmt"
	"image/color"
	"math"
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
		m.renderTasks(),
		"",
		m.renderPipelinesBlock(),
		"",
		m.renderDHI(),
		"",
		m.renderOrbBlock(),
	)

	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		m.renderLogBlock(rw),
		m.renderConvBlock(rw),
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
	gpuPct := m.metrics.GPUPercent / 100.0

	content := fmt.Sprintf(
		"CPU: %s\nRAM: %s\nGPU: %s\n%s %s",
		m.cpuProgress.ViewAs(cpuPct),
		m.ramProgress.ViewAs(ramPct),
		m.gpuProgress.ViewAs(gpuPct),
		lipgloss.NewStyle().Foreground(statusColor).Render(statusDot),
		lipgloss.NewStyle().Foreground(statusColor).Render("Ollama "+m.routerStatus),
	)

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" SYSTEM METRICS ") + "\n" + content,
	)
}

func (m *Model) renderTasks() string {
	content := fmt.Sprintf("Pending: %d\nWaiting: %d", m.pendingTasks, m.waitingTasks)
	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" TASKS ") + "\n" + content,
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

func (m *Model) orbHeight() int {
	return max(int(float64(m.height-31)*0.8), 3)
}

func (m *Model) renderOrbBlock() string {
	return m.orbView.View()
}

// ─── Right Panel ───

func (m *Model) renderLogBlock(logW int) string {
	style := borderStyle
	if m.focused == focusLogs {
		style = focusedBorderStyle
	}
	return style.Width(logW).Render(
		lipgloss.NewStyle().Bold(true).Render(" LOGS ") + "\n" + m.logView.View(),
	)
}

func (m *Model) renderConvBlock(logW int) string {
	style := borderStyle
	if m.focused == focusConv {
		style = focusedBorderStyle
	}
	return style.Width(logW).Render(
		lipgloss.NewStyle().Bold(true).Render(" CONVERSATION ") + "\n" + m.convView.View(),
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
	return style.Width(logW).Render(
		lipgloss.JoinHorizontal(lipgloss.Bottom,
			m.textInput.View(),
			"  ",
			status,
		),
	)
}

// ─── Orb Animation ───

func (m *Model) refreshOrbContent() {
	h := m.orbView.Height()
	if h < 3 {
		m.orbView.SetContent("")
		return
	}
	w := m.orbView.Width()

	intensity := (m.metrics.CPU + float64(m.metrics.RAMUsedMB)/float64(max(int(m.metrics.RAMTotalMB), 1))*100 + m.metrics.GPUPercent) / 3.0
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 100 {
		intensity = 100
	}
	intensity /= 100.0

	t := float64(m.frame) * 0.06

	baseline := lipgloss.Color("#B86520")
	fg := lipgloss.NewStyle().Foreground
	var sb strings.Builder
	for row := 0; row < h; row++ {
		isBase := row == h-1
		for col := 0; col < w; col++ {
			if isBase {
				sb.WriteString(fg(baseline).Render("▁"))
				continue
			}

			x := float64(col) * 0.4
			y := float64(row) * 0.3
			v1 := math.Sin(x*1.7 + t*1.2 + y*0.5)
			v2 := math.Sin(x*3.1 - t*0.8 + y*0.7)
			v3 := math.Sin(x*0.9 + t*2.3 - y*1.1)
			val := (v1 + v2 + v3) / 3.0
			val = val*0.7 + 0.5

			rowFactor := 0.2 + 0.8*(float64(row+1)/float64(h))
			spike := val * intensity * rowFactor * 1.4
			if spike < 0 {
				spike = 0
			}
			if spike > 1 {
				spike = 1
			}

			var ch string
			switch {
			case spike > 0.5:
				ch = "█"
			case spike > 0.3:
				ch = "▓"
			case spike > 0.15:
				ch = "▒"
			case spike > 0.05:
				ch = "░"
			default:
				ch = " "
			}

			if ch == " " {
				sb.WriteString(" ")
				continue
			}

			r := int(220.0*spike + 35.0)
			g := int(140.0*spike + 20.0)
			b := int(40.0 * spike)
			if intensity < 0.3 {
				r = int(140.0 * spike)
				g = int(80.0 * spike)
				b = int(30.0 * spike)
			}
			colr := color.RGBA{R: uint8(min(r, 255)), G: uint8(min(g, 255)), B: uint8(min(b, 255)), A: 255}
			sb.WriteString(fg(colr).Render(ch))
		}
		if row < h-1 {
			sb.WriteString("\n")
		}
	}
	m.orbView.SetContent(sb.String())
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
