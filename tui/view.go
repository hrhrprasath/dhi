package tui

import (
	"fmt"
	"image/color"
	"math/rand"
	"strings"
	"time"

	"dhi_harness/internal/task"

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

	var mainBlock string
	if m.logPopupVisible {
		mainBlock = m.renderLogPopup(rw)
	} else {
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
	if m.metrics.GPUPercent == 0 {
		gpuLine = "GPU: ○ NVIDIA Coming Soon"
	}

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

func (m *Model) renderTasks() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pending: %d    Waiting: %d", m.pendingTasks, m.waitingTasks))

	waiting := m.taskMgr.ListByStatus(task.StatusWaiting)
	if len(waiting) > 0 {
		now := time.Now()
		// Show up to 3 waiting tasks
		limit := 3
		if len(waiting) < limit {
			limit = len(waiting)
		}
		for i := 0; i < limit; i++ {
			t := waiting[i]
			desc := t.RawInput
			if len(desc) > 18 {
				desc = desc[:15] + "..."
			}
			var countdown string
			if t.ScheduledFor != nil {
				rem := t.ScheduledFor.Sub(now)
				if rem <= 0 {
					countdown = "now"
				} else if rem < time.Minute {
					countdown = fmt.Sprintf("%ds", int(rem.Seconds()))
				} else if rem < time.Hour {
					countdown = fmt.Sprintf("%dm", int(rem.Minutes()))
				} else {
					h := int(rem.Hours())
					mn := int(rem.Minutes()) % 60
					countdown = fmt.Sprintf("%dh%dm", h, mn)
				}
			}
			sb.WriteString(fmt.Sprintf("\n  • %s in %s", desc, countdown))
		}
	}

	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" TASKS ") + "\n" + sb.String(),
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
	return max(m.height-29, 3)
}

func (m *Model) renderOrbBlock() string {
	return m.orbView.View()
}

// ─── Right Panel ───

func (m *Model) renderLogPopup(logW int) string {
	help := lipgloss.NewStyle().Foreground(dimColor).Render(" ESC to close | scroll: ↑↓ PgUp PgDown ")
	return focusedBorderStyle.Width(logW).Render(
		lipgloss.NewStyle().Bold(true).Render(" LOGS ") + "\n" + m.logView.View() + "\n" + help,
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

// ─── Fire Animation (port of lamp.py) ───

var firePalette = []color.RGBA{
	{0, 0, 0, 255},
	{6, 0, 0, 255}, {12, 0, 0, 255}, {18, 0, 0, 255}, {24, 0, 0, 255},
	{30, 0, 0, 255}, {36, 0, 0, 255}, {42, 0, 0, 255}, {48, 0, 0, 255},
	{54, 0, 0, 255}, {64, 0, 0, 255}, {74, 0, 0, 255}, {84, 0, 0, 255},
	{94, 0, 0, 255}, {108, 0, 0, 255}, {122, 0, 0, 255}, {136, 0, 0, 255},
	{150, 8, 0, 255}, {164, 20, 0, 255}, {178, 36, 0, 255},
	{192, 52, 0, 255}, {206, 68, 0, 255}, {220, 84, 0, 255},
	{232, 104, 0, 255}, {240, 124, 8, 255}, {248, 144, 16, 255},
	{255, 164, 28, 255}, {255, 184, 44, 255}, {255, 204, 64, 255},
	{255, 224, 88, 255}, {255, 244, 116, 255}, {255, 255, 148, 255},
	{255, 255, 180, 255}, {255, 255, 212, 255}, {255, 255, 240, 255},
	{255, 255, 255, 255},
}

func (m *Model) advanceFire() {
	w := m.orbView.Width()
	h := m.orbView.Height()
	if w < 3 || h < 3 {
		return
	}

	size := w * h
	if len(m.firePixels) != size {
		m.firePixels = make([]int, size)
	}

	centerX := w / 2
	sourceW := 5
	sourceY := h - 2
	for x := centerX - sourceW/2; x <= centerX+sourceW/2; x++ {
		if x >= 0 && x < w {
			idx := sourceY*w + x
			m.firePixels[idx] = 33 - rand.Intn(4)
		}
	}

	newPixels := make([]int, size)

	for y := 1; y < h; y++ {
		for x := 0; x < w; x++ {
			srcIdx := y*w + x
			pixel := m.firePixels[srcIdx]
			if pixel <= 0 {
				continue
			}

			r := rand.Intn(4)
			drift := 0
			if r == 1 {
				drift = -1
			} else if r == 2 {
				drift = 1
			}

			dstX := x + drift
			if dstX < 0 || dstX >= w {
				continue
			}
			dstIdx := (y-1)*w + dstX

			distFromCenter := abs(x - centerX)
			decay := r & 1
			if distFromCenter > sourceW/2 {
				decay++
			}

			newIntensity := pixel - decay
			if newIntensity < 0 {
				newIntensity = 0
			}
			if newIntensity > newPixels[dstIdx] {
				newPixels[dstIdx] = newIntensity
			}
		}
	}

	m.firePixels = newPixels
}

func (m *Model) refreshOrbContent() {
	h := m.orbView.Height()
	w := m.orbView.Width()
	if h < 3 || w < 3 {
		m.orbView.SetContent("")
		return
	}

	size := w * h
	if len(m.firePixels) != size {
		m.firePixels = make([]int, size)
		m.orbView.SetContent("")
		return
	}

	fg := lipgloss.NewStyle().Foreground
	var sb strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			intensity := m.firePixels[y*w+x]
			if intensity == 0 {
				sb.WriteString(" ")
			} else {
				idx := intensity
				if idx >= len(firePalette) {
					idx = len(firePalette) - 1
				}
				sb.WriteString(fg(firePalette[idx]).Render("█"))
			}
		}
		if y < h-1 {
			sb.WriteString("\n")
		}
	}
	m.orbView.SetContent(sb.String())
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
