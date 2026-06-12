package tui

import (
	"regexp"
	"strings"
	"time"

	"dhi_harness/internal/config"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/services"
	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"
	"dhi_harness/internal/topic"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func NewModel(cfg *config.Config, logger *services.Logger, tm *task.Manager,
	router *intent.Router, pipeExec *pipeline.Executor, topicMgr *topic.Manager, sysSvc *services.SystemService, todoMgr *todo.Manager, feedback FeedbackRecorder) *Model {

	ti := textinput.New()
	ti.Placeholder = " Type a command... (/topic → topics /log → logs)"
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
	tv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(8))

	cp := progress.New(progress.WithWidth(21))
	cp.Empty = '░'
	cp.Full = '█'
	rp := progress.New(progress.WithWidth(21))
	rp.Empty = '░'
	rp.Full = '█'
	gp := progress.New(progress.WithWidth(21))
	gp.Empty = '░'
	gp.Full = '█'

	s := spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{
				"░▒▓█",
				"█░▒▓",
				"▓█░▒",
				"▒▓█░",
				"░▒▓█",
				"█░▒▓",
				"▓█░▒",
				"▒▓█░",
				"░▒▓█",
				"█░▒▓",
				"▓█░▒",
				"▒▓█░",
			},
			FPS: time.Second / 6,
		}),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(accentColor)),
	)

	metrics := sysSvc.GetMetrics()

	statusPipelines := make([]pipelineEntry, 0)
	for _, p := range pipeExec.ListPipelines() {
		statusPipelines = append(statusPipelines, pipelineEntry{Name: p, Status: "IDLE"})
	}

	m := &Model{
		cfg:             cfg,
		logger:          logger,
		taskMgr:         tm,
		router:          router,
		executor:        pipeExec,
		topicMgr:        topicMgr,
		sysSvc:          sysSvc,
		todoMgr:         todoMgr,
		feedback:        feedback,
		thinkingSpinner: s,
		textInput:       ti,
		logView:         lv,
		convView:        cv,
		pipelineView:    pv,
		topicView:       tv,
		cpuProgress:     cp,
		ramProgress:     rp,
		gpuProgress:     gp,
		focused:         focusInput,
		pipelines:       statusPipelines,
		routerStatus:    "OFFLINE",
		hostname:        metrics.Hostname,
		kernel:          metrics.Kernel,
		uptimeSec:       metrics.UptimeSec,
		metrics:         metrics,
		focusCmd:        focusCmd,
		events:          make(chan interface{}, 200),
		width:           80,
		height:          30,
		logEntries:      []string{"[SYSTEM][INFO] DHI Harness initializing..."},
	}

	m.refreshTopicFiltered()
	m.refreshPipelineContent()

	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		turns := m.topicMgr.LoadTopicTurns(cur.ID)
		for _, t := range turns {
			m.conversations = append(m.conversations, conversationEntry{Input: t.Input, Response: stripMarkdown(t.Response)})
		}
		m.refreshConvContent()
		m.convView.GotoBottom()
	}

	return m
}

func (m *Model) Init() tea.Cmd {
	go m.consumerLoop()
	go m.healthLoop()
	go m.loggerForwardLoop()
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
		func() tea.Msg { return m.thinkingSpinner.Tick() },
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

	case tea.MouseClickMsg:
		cmd := m.handleMouseClick(msg.Mouse())
		return m, tea.Batch(cmd, m.eventPoller())

	case tea.MouseWheelMsg:
		m.handleMouseWheel(msg.Mouse())
		return m, m.eventPoller()

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
		m.topicView.SetWidth(vw - 2)
		m.topicView.SetHeight(ch)
		m.textInput.SetWidth(rw - 16)
		m.refreshLogContent()
		return m, m.eventPoller()

	case tickMsg:
		m.uptimeSec++
		m.metrics = m.sysSvc.GetMetrics()
		m.refreshLogContent()
		m.refreshTopicListContent()
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
		replaced := false
		for i := len(m.conversations) - 1; i >= 0; i-- {
			if m.conversations[i].Input == msg.input && m.conversations[i].thinking {
				m.conversations[i].Response = stripMarkdown(msg.response)
				m.conversations[i].thinking = false
				m.thinking--
				replaced = true
				break
			}
		}
		if !replaced {
			m.conversations = append(m.conversations, conversationEntry{Input: msg.input, Response: stripMarkdown(msg.response)})
		}
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.thinkingSpinner, cmd = m.thinkingSpinner.Update(msg)
		if m.thinking > 0 {
			for i := len(m.conversations) - 1; i >= 0; i-- {
				if m.conversations[i].thinking {
					m.conversations[i].Response = m.thinkingSpinner.View()
					break
				}
			}
			m.refreshConvContent()
		}
		return m, tea.Batch(cmd, m.eventPoller())

	case refreshMsg:
		m.refreshLogContent()
		m.refreshConvContent()
		m.refreshPipelineContent()
		m.refreshTopicListContent()
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
