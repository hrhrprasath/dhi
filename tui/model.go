package tui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strings"
	"time"

	"dhi_harness/internal/config"
	"dhi_harness/internal/conversation"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/queue"
	"dhi_harness/internal/services"
	"dhi_harness/internal/todo"

	"charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/progress"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type focusPanel int

const (
	focusLogs focusPanel = iota
	focusInput
	focusConv
)

type conversationEntry struct {
	Input    string
	Response string
}

type tickMsg struct{}
type routerStatusMsg struct{ online bool }
type logMsg struct{ message string }
type convMsg struct {
	input    string
	response string
}
type pipelineStatusMsg struct {
	name   string
	status string
}
type refreshMsg struct{}

var (
	accentColor = lipgloss.Color("#FFA500")
	dimColor    = lipgloss.Color("#888888")
	greenColor  = lipgloss.Color("#00FF00")
	yellowColor = lipgloss.Color("#FFFF00")
	redColor    = lipgloss.Color("#FF0000")

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444"))
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)
)

type Model struct {
	cfg       *config.Config
	logger    *services.Logger
	queueMgr  *queue.Manager
	router    *intent.Router
	executor  *pipeline.Executor
	convMgr   *conversation.Manager
	sysSvc    *services.SystemService
	todoMgr   *todo.Manager

	textInput    textinput.Model
	logView      viewport.Model
	convView     viewport.Model
	pipelineView viewport.Model
	orbView      viewport.Model
	cpuProgress  progress.Model
	ramProgress  progress.Model
	gpuProgress  progress.Model

	frame int

	conversations []conversationEntry
	logEntries    []string
	routerOnline  bool
	routerStatus  string
	pipelines     []pipelineEntry
	inputCount    int
	outputCount   int
	focused       focusPanel

	hostname  string
	kernel    string
	uptimeSec uint64

	metrics services.SystemMetrics

	events   chan interface{}
	focusCmd tea.Cmd
	width    int
	height   int
}

type pipelineEntry struct {
	Name   string
	Status string
}

func NewModel(cfg *config.Config, logger *services.Logger, qm *queue.Manager,
	router *intent.Router, pipeExec *pipeline.Executor, convMgr *conversation.Manager, sysSvc *services.SystemService, todoMgr *todo.Manager) *Model {

	ti := textinput.New()
	ti.Placeholder = " Type a command... (Tab to focus)"
	ti.Prompt = "> "
	ti.SetWidth(34)
	focusCmd := ti.Focus()

	initW := 40
	lv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(8))
	lv.SetContent("No logs yet...")
	cv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(8))
	cv.SetContent("Start a conversation...")
	pv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(6))
	ov := viewport.New(viewport.WithWidth(26), viewport.WithHeight(6))
	ov.SetContent("")
	cp := progress.New(progress.WithWidth(21))
	cp.Empty = '░'
	cp.Full = '█'
	rp := progress.New(progress.WithWidth(21))
	rp.Empty = '░'
	rp.Full = '█'
	gp := progress.New(progress.WithWidth(21))
	gp.Empty = '░'
	gp.Full = '█'

	metrics := sysSvc.GetMetrics()

	statusPipelines := make([]pipelineEntry, 0)
	for _, p := range pipeExec.ListPipelines() {
		statusPipelines = append(statusPipelines, pipelineEntry{Name: p, Status: "IDLE"})
	}

	return &Model{
		cfg:        cfg,
		logger:     logger,
		queueMgr:   qm,
		router:     router,
		executor:   pipeExec,
		convMgr:   convMgr,
		sysSvc:    sysSvc,
		todoMgr:   todoMgr,
		textInput:  ti,
		logView:    lv,
		convView:   cv,
		pipelineView: pv,
		orbView:     ov,
		cpuProgress: cp,
		ramProgress: rp,
		gpuProgress: gp,
		focused:      focusInput,
		pipelines:    statusPipelines,
		routerStatus: "OFFLINE",
		hostname:     metrics.Hostname,
		kernel:       metrics.Kernel,
		uptimeSec:    metrics.UptimeSec,
		metrics:      metrics,
		focusCmd:    focusCmd,
		events:      make(chan interface{}, 200),
		width:       80,
		height:      30,
		logEntries:  []string{"[SYSTEM][INFO] DHI Harness initializing..."},
	}
}

func (m *Model) Init() tea.Cmd {
	go m.consumerLoop()
	go m.healthLoop()
	go m.loggerForwardLoop()

	go func() {
		online := m.router.HealthCheck()
		m.events <- routerStatusMsg{online: online}
	}()

	return tea.Batch(
		m.eventPoller(),
		tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
		m.focusCmd,
	)
}

func (m *Model) eventPoller() tea.Cmd {
	return func() tea.Msg {
		return <-m.events
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		cmd := m.handleKeyPress(msg)
		return m, tea.Batch(cmd, m.eventPoller())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		rw := m.rightWidth()
		vw := rw - 2
		lh := max((msg.Height-9)/3, 3)
		ch := max(msg.Height-9-lh, 3)
		m.logView.SetWidth(vw)
		m.logView.SetHeight(lh)
		m.convView.SetWidth(vw)
		m.convView.SetHeight(ch)
		m.textInput.SetWidth(rw - 16)
		m.orbView.SetWidth(26)
		m.orbView.SetHeight(m.orbHeight())
		m.refreshLogContent()
		m.refreshOrbContent()
		return m, m.eventPoller()

	case tickMsg:
		m.frame++
		m.uptimeSec++
		m.inputCount = m.queueMgr.InputLen()
		m.outputCount = m.queueMgr.OutputLen()
		m.metrics = m.sysSvc.GetMetrics()
		m.refreshLogContent()
		m.refreshOrbContent()
		return m, tea.Batch(
			m.eventPoller(),
			tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
		)

	case routerStatusMsg:
		m.routerOnline = msg.online
		if msg.online {
			m.routerStatus = "READY"
		} else {
			m.routerStatus = "OFFLINE"
		}
		return m, m.eventPoller()

	case logMsg:
		m.logEntries = append(m.logEntries, msg.message)
		if len(m.logEntries) > 1000 {
			m.logEntries = m.logEntries[len(m.logEntries)-500:]
		}
		m.refreshLogContent()
		m.logView.GotoBottom()
		return m, m.eventPoller()

	case convMsg:
		m.conversations = append(m.conversations, conversationEntry{Input: msg.input, Response: stripMarkdown(msg.response)})
		m.refreshConvContent()
		m.convView.GotoBottom()
		return m, m.eventPoller()

	case pipelineStatusMsg:
		for i, p := range m.pipelines {
			if p.Name == msg.name {
				m.pipelines[i].Status = msg.status
				m.refreshPipelineContent()
				return m, m.eventPoller()
			}
		}
		m.pipelines = append(m.pipelines, pipelineEntry{Name: msg.name, Status: msg.status})
		m.refreshPipelineContent()
		return m, m.eventPoller()

	case refreshMsg:
		m.refreshLogContent()
		m.refreshConvContent()
		m.refreshPipelineContent()
		return m, m.eventPoller()

	default:
		return m, m.eventPoller()
	}
}

func (m *Model) View() tea.View {
	return tea.View{
		Content:   m.render(),
		AltScreen: true,
	}
}

// ─── Key Handling ───

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		fallthrough
	case msg.Code == 'd' && msg.Mod == tea.ModCtrl:
		return tea.Quit

	case msg.Code == tea.KeyTab:
		switch m.focused {
		case focusLogs:
			m.focused = focusInput
			return m.textInput.Focus()
		case focusInput:
			m.focused = focusConv
			m.textInput.Blur()
			return nil
		case focusConv:
			m.focused = focusLogs
			m.textInput.Blur()
			return nil
		}

	case msg.Code == tea.KeyEnter:
		if m.focused == focusInput {
			input := strings.TrimSpace(m.textInput.Value())
			m.textInput.SetValue("")
			if input == "quit" || input == "exit" || input == ":q" || input == "/quit" || input == "/q" {
				return tea.Quit
			}
			m.logger.Info("INPUT", fmt.Sprintf("User: %s", input))
			if input != "" && m.routerOnline {
				m.queueMgr.EnqueueInput(queue.NewInputItem(input))
				m.inputCount = m.queueMgr.InputLen()
			}
		}
		return nil

	case msg.Code == tea.KeyUp:
		switch m.focused {
		case focusLogs:
			m.logView.ScrollUp(3)
		case focusConv:
			m.convView.ScrollUp(3)
		}
		return nil

	case msg.Code == tea.KeyDown:
		switch m.focused {
		case focusLogs:
			m.logView.ScrollDown(3)
		case focusConv:
			m.convView.ScrollDown(3)
		}
		return nil

	case msg.Code == tea.KeyPgUp:
		switch m.focused {
		case focusLogs:
			m.logView.ScrollUp(m.logView.Height())
		case focusConv:
			m.convView.ScrollUp(m.convView.Height())
		}
		return nil

	case msg.Code == tea.KeyPgDown:
		switch m.focused {
		case focusLogs:
			m.logView.ScrollDown(m.logView.Height())
		case focusConv:
			m.convView.ScrollDown(m.convView.Height())
		}
		return nil
	}

	if m.focused == focusInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return cmd
	}
	return nil
}

// ─── Background Loops ───

func (m *Model) consumerLoop() {
	for {
		time.Sleep(200 * time.Millisecond)
		m.processQueue()
	}
}

func (m *Model) healthLoop() {
	for {
		time.Sleep(30 * time.Second)
		online := m.router.HealthCheck()
		m.events <- routerStatusMsg{online: online}
	}
}

func (m *Model) loggerForwardLoop() {
	sub := m.logger.Subscribe(100)
	for entry := range sub.Ch() {
		var level string
		switch entry.Level {
		case services.DebugLevel:
			level = "DEBUG"
		case services.InfoLevel:
			level = "INFO"
		case services.WarnLevel:
			level = "WARN"
		case services.ErrorLevel:
			level = "ERROR"
		default:
			level = "?"
		}
		m.events <- logMsg{message: fmt.Sprintf("[%s][%s] %s", entry.Component, level, entry.Message)}
	}
}

func (m *Model) processQueue() {
	item := m.queueMgr.DequeueNextPending()
	if item == nil {
		return
	}
	if item.RetryCount >= 3 {
		m.logger.Error("CONSUMER", fmt.Sprintf("Item %s exceeded max retries", item.ID))
		item.Status = queue.StatusFailed
		m.queueMgr.UpdateItem(item)
		return
	}

	summary := m.convMgr.LoadSummary()
	summaryText := m.convMgr.FormatForPrompt(&summary)

	todos := m.todoMgr.FormatForPrompt()
	if !strings.Contains(todos, "No todos") && !strings.Contains(todos, "No pending") {
		summaryText += "\n\nPending todos:\n" + todos
	}

	m.logger.Info("CONSUMER", fmt.Sprintf("Classifying: %s", item.RawInput))

	intentResult, err := m.router.Route(item.RawInput, summaryText)
	if err != nil {
		m.logger.Error("CONSUMER", fmt.Sprintf("Classify failed: %s", err))
		item.RetryCount++
		item.Status = queue.StatusPending
		item.ProcessingStartedAt = nil
		m.queueMgr.UpdateItem(item)
		return
	}

	if intentResult.Confidence < 0.8 || !strings.HasPrefix(intentResult.Event, "EXECUTE_") {
		m.logger.Info("CONSUMER", fmt.Sprintf("Confidence %.2f < 0.8 — falling back to conversation", intentResult.Confidence))
		m.handleConversation(item, summaryText, intentResult)
	} else {
		m.handlePipeline(item, summaryText, intentResult)
	}
}

func stripMarkdown(s string) string {
	s = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(s, "")
	s = regexp.MustCompile("`[^`]*`").ReplaceAllString(s, "")
	s = regexp.MustCompile("\\[([^\\]]*)\\]\\([^)]*\\)").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("!\\[([^\\]]*)\\]\\([^)]*\\)").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("\\*\\*(.*?)\\*\\*").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("__(.*?)__").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("\\*(.*?)\\*").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("_(.*?)_").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("~~(.*?)~~").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("^#{1,6}\\s+").ReplaceAllString(s, "")
	s = regexp.MustCompile("^>\\s?").ReplaceAllString(s, "")
	s = regexp.MustCompile("^[-*+]\\s+").ReplaceAllString(s, "")
	s = regexp.MustCompile("^\\d+\\.\\s+").ReplaceAllString(s, "")
	s = regexp.MustCompile("^[-*_]{3,}$").ReplaceAllString(s, "")
	s = regexp.MustCompile("\\n{3,}").ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func cleanResponse(response, userInput string) string {
	idx := strings.Index(response, "[USER]")
	if idx < 0 {
		idx = strings.Index(response, "---BEGIN INPUT---")
	}
	if idx < 0 {
		idx = strings.Index(response, "User says:")
	}
	if idx < 0 && len(userInput) > 0 {
		idx = strings.Index(response, "User: "+userInput)
	}
	if idx >= 0 {
		response = strings.TrimSpace(response[:idx])
	}
	return response
}

func (m *Model) handleConversation(item *queue.InputItem, summaryText string, intentResult *intent.ParsedIntent) {
	m.logger.Info("CONSUMER", fmt.Sprintf("Intent: %s — chatting", intentResult.Event))

	terms := conversation.ExtractSearchTerms(intentResult.Params)
	searchResults := ""
	if len(terms) > 0 {
		sr, _ := conversation.SearchPastData(terms)
		searchResults = sr
	}

	response, err := m.router.Chat(item.RawInput, summaryText, searchResults)
	if err != nil {
		m.logger.Error("CONSUMER", fmt.Sprintf("Chat failed: %s", err))
		item.Status = queue.StatusFailed
		m.queueMgr.UpdateItem(item)
		return
	}

	response = cleanResponse(response, item.RawInput)

	m.logger.Info("DHI", response)
	m.convMgr.UpdateSummary(item.RawInput, response)
	m.events <- convMsg{input: item.RawInput, response: response}

	item.Status = queue.StatusCompleted
	m.queueMgr.UpdateItem(item)
}

func (m *Model) handlePipeline(item *queue.InputItem, summaryText string, intentResult *intent.ParsedIntent) {
	m.logger.Info("CONSUMER", fmt.Sprintf("Intent: %s — executing", intentResult.Event))

	pipelineName := strings.ToLower(strings.TrimPrefix(intentResult.Event, "EXECUTE_"))
	item.Intent = &queue.Intent{
		Pipeline: pipelineName,
		Action:   intentResult.Event,
	}
	item.Status = queue.StatusCompleted
	m.queueMgr.UpdateItem(item)

	m.events <- pipelineStatusMsg{name: pipelineName, status: "RUNNING"}

	params := make(map[string]string)
	if intentResult.Params != nil {
		var raw map[string]interface{}
		if json.Unmarshal(intentResult.Params, &raw) == nil {
			for k, v := range raw {
				params[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	raw, synth := m.executor.FindAndExecute(intentResult.Event, params, item.RawInput, summaryText)

	var reply string
	if synth != raw {
		reply = synth
	} else if raw != "" {
		reply = raw
	} else if intentResult.Description != "" {
		reply = intentResult.Description
	} else {
		reply = "Action completed"
	}

	if raw != "" {
		m.logger.Info("EXECUTOR", fmt.Sprintf("Raw output:\n%s", raw))
	}

	m.logger.Info("DHI", reply)
	m.convMgr.UpdateSummary(item.RawInput, reply)
	m.events <- convMsg{input: item.RawInput, response: reply}
	m.events <- pipelineStatusMsg{name: pipelineName, status: "IDLE"}
	m.logger.Info("EXECUTOR", fmt.Sprintf("Pipeline %s completed", intentResult.Event))
	output := queue.NewOutputItem(pipelineName, item.ID, raw, reply)
	m.queueMgr.EnqueueOutput(output)
}

// ─── Content Rendering ───

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

func (m *Model) render() string {
	rw := m.rightWidth()

	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		m.renderMetrics(),
		"",
		m.renderQueues(),
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

func (m *Model) renderQueues() string {
	content := fmt.Sprintf("IN:  %d items\nOUT: %d items", m.inputCount, m.outputCount)
	return borderStyle.Width(28).Render(
		lipgloss.NewStyle().Bold(true).Render(" QUEUES ") + "\n" + content,
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
