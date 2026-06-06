package tui

import (
	"regexp"
	"strings"
	"time"

	"dhi_harness/internal/config"
	"dhi_harness/internal/conversation"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/services"
	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func NewModel(cfg *config.Config, logger *services.Logger, tm *task.Manager,
	router *intent.Router, pipeExec *pipeline.Executor, convMgr *conversation.Manager, sysSvc *services.SystemService, todoMgr *todo.Manager) *Model {

	ti := textinput.New()
	ti.Placeholder = " Type a command... (/log /logs /l → logs popup)"
	ti.Prompt = "> "
	ti.SetWidth(34)
	focusCmd := ti.Focus()

	initW := 40
	lv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(10))
	lv.SoftWrap = true
	lv.SetContent("No logs yet...")
	cv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(10))
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
		taskMgr:   tm,
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
		pendingTasks: tm.CountByStatus(task.StatusPending),
		waitingTasks: tm.CountByStatus(task.StatusWaiting),
		focused:      focusInput,
		pipelines:    statusPipelines,
		routerStatus:    "OFFLINE",
		logPopupVisible: false,
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
	go m.fireLoop()
	m.taskMgr.StartLooper(60 * time.Second)

	go func() {
		activated := m.taskMgr.CheckSchedule()
		if activated > 0 {
			m.events <- refreshMsg{}
		}
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
		ch := max(msg.Height-7, 3)
		lh := max(ch-1, 6)
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

	case fireTickMsg:
		m.advanceFire()
		m.refreshOrbContent()
		return m, m.eventPoller()

	case tickMsg:
		m.frame++
		m.uptimeSec++
		m.pendingTasks = m.taskMgr.CountByStatus(task.StatusPending)
		m.waitingTasks = m.taskMgr.CountByStatus(task.StatusWaiting)
		m.metrics = m.sysSvc.GetMetrics()
		m.refreshLogContent()
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
