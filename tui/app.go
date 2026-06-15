package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"dhi_harness/internal/intent"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/services"
	"dhi_harness/internal/settings"
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

type FeedbackRecorder interface {
	RecordOutcome(input, predictedEvent, pipelineUsed string, confidence float64, success bool, wasConversation bool, errMsg string)
	RecordConversation(input string)
	RecordRating(input string, positive bool)
	SetModel(name string)
}

type focusPanel int

const (
	focusLogs focusPanel = iota
	focusInput
	focusConv
	focusTopics
	focusModels
)

type Model struct {
	logger      *services.Logger
	taskMgr     *task.Manager
	router      *intent.Router
	executor    *pipeline.Executor
	topicMgr    *topic.Manager
	sysSvc      *services.SystemService
	todoMgr     *todo.Manager
	feedback    FeedbackRecorder
	settingsMgr *settings.Manager

	width    int
	height   int
	events   chan any
	focused  focusPanel
	focusCmd tea.Cmd

	lastInput      string
	lastIntent     *intent.ParsedIntent
	lastSearchURLs []string

	dhiOS    dhiOS
	system   systemPanel
	pipeline pipelinePanel
	cc       commandCenter
	topic    topicPanel
	model    modelPanel
	help     helpPanel
	log      logPanel
	tracker  trackerPanel
	todo     todoPanel
	schedule schedulePanel
	welcome  welcomePanel
}

func NewModel(logger *services.Logger, tm *task.Manager,
	router *intent.Router, pipeExec *pipeline.Executor, topicMgr *topic.Manager, sysSvc *services.SystemService, todoMgr *todo.Manager, feedback FeedbackRecorder, settingsMgr *settings.Manager) *Model {

	ti := textinput.New()
	ti.Placeholder = " Type a command... (/help for help)"
	ti.Prompt = "> "
	ti.SetWidth(26)
	focusCmd := ti.Focus()

	initW := 40
	cv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(10))
	cv.SetContent("Start a conversation...")

	pv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(6))
	tv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(8))
	mv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(5))
	tv2 := viewport.New(viewport.WithWidth(26), viewport.WithHeight(8))
	sv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(8))
	lv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(10))
	lv.SoftWrap = true
	lv.SetContent("No logs yet...")

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
				"░░░░",
				"▒░░░",
				"▓░░░",
				"█░░░",
				"█▒░░",
				"█▓░░",
				"██░░",
				"██▒░",
				"██▓░",
				"███░",
				"███▒",
				"███▓",
				"████",
			},
			FPS: time.Second / 13,
		}),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(accentColor)),
	)

	metrics := sysSvc.GetMetrics()

	statusPipelines := make([]pipelineEntry, 0)
	for _, p := range pipeExec.ListPipelines() {
		statusPipelines = append(statusPipelines, pipelineEntry{name: p, status: "IDLE"})
	}

	m := &Model{
		logger:      logger,
		taskMgr:     tm,
		router:      router,
		executor:    pipeExec,
		topicMgr:    topicMgr,
		sysSvc:      sysSvc,
		todoMgr:     todoMgr,
		feedback:    feedback,
		settingsMgr: settingsMgr,

		focused:  focusInput,
		focusCmd: focusCmd,
		events:   make(chan any, 200),
		width:    80,
		height:   30,

		dhiOS: dhiOS{
			hostname:  metrics.Hostname,
			kernel:    metrics.Kernel,
			uptimeSec: metrics.UptimeSec,
		},
		system: systemPanel{
			metrics:      metrics,
			routerStatus: "OFFLINE",
			cpuProgress:  cp,
			ramProgress:  rp,
			gpuProgress:  gp,
		},
		pipeline: pipelinePanel{
			entries: statusPipelines,
			view:    pv,
		},
		cc: commandCenter{
			spinner:   s,
			textInput: ti,
			convView:  cv,
		},
		topic: topicPanel{
			view: tv,
		},
		model: modelPanel{
			view: mv,
		},
		help: newHelpPanel(),
		log: logPanel{
			view:    lv,
			entries: []string{"[SYSTEM][INFO] DHI Harness initializing..."},
		},
		todo: todoPanel{
			view: tv2,
		},
		schedule: schedulePanel{
			view: sv,
		},
		welcome: welcomePanel{
			show:      true,
			status:    "CHECKING",
			startTime: time.Now(),
			minShow:   2 * time.Second,
		},
	}

	m.help.content = buildHelpContent(m.settingsMgr.Endpoint())
	m.topic.filterTopics(m.topicMgr.ListTopics())
	m.model.filterModels(m.settingsMgr.ListModels())
	m.pipeline.render()

	if cur := m.settingsMgr.CurrentModel(); cur != nil {
		m.router.SetModel(cur.Name)
		m.router.SetContextSize(m.settingsMgr.ContextLength())
		m.executor.SetModel(cur.Name)
		m.executor.SetContextSize(m.settingsMgr.ContextLength())
	}

	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		turns := m.topicMgr.LoadTopicTurns(cur.ID)
		convTurns := make([]conversationEntry, len(turns))
		for i, t := range turns {
			convTurns[i] = conversationEntry{input: t.Input, response: stripMarkdown(t.Response)}
		}
		m.cc.setConversationsFromTurns(convTurns)
		m.cc.convView.GotoBottom()
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
		func() tea.Msg { return m.cc.spinner.Tick() },
		m.focusCmd,
	)
}

func (m *Model) eventPoller() tea.Cmd {
	return func() tea.Msg {
		return <-m.events
	}
}

// ─── Update ───

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
		m.log.view.SetWidth(vw)
		m.log.view.SetHeight(lh)
		m.cc.convView.SetWidth(vw)
		m.cc.convView.SetHeight(ch)
		m.topic.view.SetWidth(vw - 2)
		m.topic.view.SetHeight(ch)
		m.model.view.SetWidth(vw - 2)
		m.model.view.SetHeight(ch)
		m.help.view.SetWidth(vw - 2)
		m.help.view.SetHeight(ch)
		m.todo.view.SetWidth(vw - 2)
		m.todo.view.SetHeight(ch + 1)
		m.schedule.view.SetWidth(vw - 2)
		m.schedule.view.SetHeight(ch + 1)
		m.cc.textInput.SetWidth(rw - 24)
		return m, m.eventPoller()

	case tickMsg:
		m.dhiOS.uptimeSec++
		m.system.metrics = m.sysSvc.GetMetrics()
		return m, tea.Batch(
			m.eventPoller(),
			tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
		)

	case routerStatusMsg:
		m.system.routerOnline = msg.online
		if msg.online {
			m.system.routerStatus = "READY"
		} else {
			m.system.routerStatus = "OFFLINE"
		}
		if m.welcome.show && !m.welcome.checked {
			m.welcome.checked = true
			if msg.online {
				m.welcome.status = "READY"
			} else {
				m.welcome.status = "OFFLINE"
			}
			if time.Since(m.welcome.startTime) >= m.welcome.minShow {
				m.welcome.show = false
			}
		}
		return m, m.eventPoller()

	case logMsg:
		m.log.append(msg.message)
		m.log.view.GotoBottom()
		return m, m.eventPoller()

	case convMsg:
		if !m.cc.setResponse(msg.input, stripMarkdown(msg.response)) {
			m.cc.conversations = append(m.cc.conversations, conversationEntry{
				input:    msg.input,
				response: stripMarkdown(msg.response),
			})
		}
		m.cc.convView.GotoBottom()
		return m, m.eventPoller()

	case pipelineStatusMsg:
		for i, p := range m.pipeline.entries {
			if p.name == msg.name {
				m.pipeline.entries[i].status = msg.status
				return m, m.eventPoller()
			}
		}
		m.pipeline.entries = append(m.pipeline.entries, pipelineEntry{name: msg.name, status: msg.status})
		return m, m.eventPoller()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.cc.spinner, cmd = m.cc.spinner.Update(msg)
		if m.cc.updateSpinnerResponse() {
			m.cc.convView.GotoBottom()
		}
		return m, tea.Batch(cmd, m.eventPoller())

	case refreshMsg:
		return m, m.eventPoller()

	default:
		return m, m.eventPoller()
	}
}

// ─── View ───

func (m *Model) View() tea.View {
	return tea.View{
		Content:   m.render(),
		AltScreen: true,
	}
}

func (m *Model) render() string {
	if m.welcome.show {
		return m.welcome.render(m.width, m.height)
	}
	rw := m.rightWidth()

	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		m.dhiOS.render(),
		m.system.render(),
		m.pipeline.render(),
		m.tracker.render(m.todoMgr, m.taskMgr),
		m.topic.renderRecent(m.topicMgr.ListTopics(), m.topicMgr.CurrentTopic(), m.focused == focusTopics, m.height),
	)

	var mainBlock string
	switch {
	case m.help.popupOpen:
		mainBlock = m.help.renderPopup(rw)
	case m.log.popupOpen:
		mainBlock = m.log.renderPopup(rw)
	case m.topic.popupOpen:
		mainBlock = m.topic.renderPopup(rw, m.topicMgr.CurrentTopic())
	case m.model.popupOpen:
		mainBlock = m.model.renderPopup(rw, m.settingsMgr.CurrentModel())
	case m.todo.popupOpen:
		mainBlock = m.todo.renderPopup(rw)
	case m.schedule.popupOpen:
		mainBlock = m.schedule.renderPopup(rw)
	default:
		topicTitle := ""
		if cur := m.topicMgr.CurrentTopic(); cur != nil {
			topicTitle = cur.Title
		}
		mainBlock = m.cc.renderConv(rw, m.focused == focusConv, topicTitle)
	}

	modelName := ""
	if cur := m.settingsMgr.CurrentModel(); cur != nil {
		modelName = cur.Name
	}
	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		mainBlock,
		m.cc.renderInput(rw, m.system.routerOnline, m.focused == focusInput, modelName),
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

// ─── Keyboard Handling ───

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	if m.welcome.show {
		m.welcome.show = false
		return m.cc.textInput.Focus()
	}
	switch {
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		return tea.Quit
	case msg.Code == 'd' && msg.Mod == tea.ModCtrl:
		return tea.Quit

	case msg.Code == tea.KeyEscape:
		return m.handleEscape()

	case msg.Code == tea.KeyTab:
		return m.handleTab()

	case msg.Code == tea.KeyEnter:
		return m.handleEnter()

	case msg.Code == tea.KeyUp:
		return m.handleArrowUp()

	case msg.Code == tea.KeyDown:
		return m.handleArrowDown()

	case msg.Code == tea.KeyPgUp:
		return m.handlePageUp()

	case msg.Code == tea.KeyPgDown:
		return m.handlePageDown()

	case msg.Code == tea.KeyLeft:
		return m.handleArrowLeft()

	case msg.Code == tea.KeyRight:
		return m.handleArrowRight()
	}

	if m.todo.popupOpen || m.schedule.popupOpen {
		switch {
		case msg.Code == 'r' || msg.Code == 'R':
			if m.todo.popupOpen {
				return m.todoRemoveSelected()
			}
			if m.schedule.popupOpen {
				return m.scheduleRemoveSelected()
			}
		case msg.Code == 'd' || msg.Code == 'D':
			if m.todo.popupOpen {
				return m.todoDoneSelected()
			}
			if m.schedule.popupOpen {
				return m.scheduleResetSelected()
			}
		}
	}

	if m.focused == focusInput {
		var cmd tea.Cmd
		m.cc.textInput, cmd = m.cc.textInput.Update(msg)

		if m.topic.popupOpen {
			oldFilter := m.topic.filter
			m.topic.filter = m.cc.textInput.Value()
			if m.topic.filter != oldFilter {
				m.topic.filterTopics(m.topicMgr.ListTopics())
				m.topic.selIdx = 0
			}
		}
		if m.model.popupOpen {
			oldFilter := m.model.filter
			m.model.filter = m.cc.textInput.Value()
			if m.model.filter != oldFilter {
				m.model.filterModels(m.settingsMgr.ListModels())
				m.model.selIdx = 0
			}
		}

		return cmd
	}
	return nil
}

func (m *Model) handleEscape() tea.Cmd {
	if m.todo.popupOpen {
		m.toggleTodoPopup()
		return nil
	}
	if m.schedule.popupOpen {
		m.toggleSchedulePopup()
		return nil
	}
	if m.topic.popupOpen {
		m.toggleTopicPopup()
		return nil
	}
	if m.model.popupOpen {
		m.toggleModelPopup()
		return nil
	}
	if m.help.popupOpen {
		m.toggleHelpPopup()
		return nil
	}
	if m.log.popupOpen {
		m.log.popupOpen = false
		if m.focused == focusLogs {
			m.focused = focusInput
			return m.cc.textInput.Focus()
		}
	}
	return nil
}

func (m *Model) handleTab() tea.Cmd {
	if m.topic.popupOpen {
		m.selectCurrentTopic()
		return nil
	}
	if m.model.popupOpen {
		m.selectCurrentModel()
		return nil
	}
	if m.focused == focusInput {
		val := m.cc.textInput.Value()
		if strings.HasPrefix(val, "/") {
			m.computeSlashMatches(val)
			if len(m.cc.slashMatches) > 0 {
				m.cc.slashIndex = (m.cc.slashIndex + 1) % len(m.cc.slashMatches)
				m.cc.textInput.SetValue(m.cc.slashMatches[m.cc.slashIndex])
				m.cc.textInput.SetCursor(len(m.cc.slashMatches[m.cc.slashIndex]))
			}
			return nil
		}
	}
	switch m.focused {
	case focusLogs:
		m.focused = focusInput
		return m.cc.textInput.Focus()
	case focusInput:
		m.focused = focusConv
		m.cc.textInput.Blur()
		return nil
	case focusConv:
		if m.log.popupOpen {
			m.focused = focusLogs
			m.cc.textInput.Blur()
			m.log.view.GotoBottom()
			return nil
		}
		m.focused = focusTopics
		m.cc.textInput.Blur()
		return nil
	case focusTopics:
		m.focused = focusInput
		return m.cc.textInput.Focus()
	}
	return nil
}

func (m *Model) handleEnter() tea.Cmd {
	if m.focused == focusTopics {
		m.selectRecentTopic()
		return nil
	}
	if m.focused == focusModels {
		m.selectRecentModel()
		return nil
	}
	if m.focused == focusInput {
		input := strings.TrimSpace(m.cc.textInput.Value())

		if len(m.cc.slashMatches) > 0 && strings.HasPrefix(input, "/") && input != m.cc.slashMatches[m.cc.slashIndex] {
			input = m.cc.slashMatches[m.cc.slashIndex]
		}

		if m.topic.popupOpen {
			m.selectCurrentTopic()
			return nil
		}
		if m.model.popupOpen {
			m.selectCurrentModel()
			return nil
		}

		if input == "" {
			return nil
		}

		if !strings.HasPrefix(input, "/") && m.cc.thinking > 0 {
			return nil
		}

		m.cc.textInput.SetValue("")
		m.cc.slashMatches = nil
		m.cc.slashIndex = 0

		if input == "/help" || input == "/?" || input == ":h" {
			// refresh content in case endpoint changed
			m.help.content = buildHelpContent(m.settingsMgr.Endpoint())
			m.toggleHelpPopup()
			return nil
		}
		if input == "quit" || input == "exit" || input == ":q" || input == "/quit" || input == "/q" {
			return tea.Quit
		}
		if input == "/log" || input == "/logs" || input == "/l" {
			m.toggleLogPopup()
			return nil
		}
		if input == "/good" || input == "/bad" {
			positive := input == "/good"
			if m.lastInput != "" {
				m.feedback.RecordRating(m.lastInput, positive)
				msg := fmt.Sprintf("Feedback recorded: %s for \"%s\"", input, m.lastInput)
				m.logger.Info("FEEDBACK", msg)
				m.events <- logMsg{message: fmt.Sprintf("[FEEDBACK][INFO] %s", msg)}
			}
			return nil
		}
		if input == "/new" {
			m.handleNewTopic()
			return nil
		}
		if input == "/fork" {
			m.handleForkTopic()
			return nil
		}
		if input == "/resume" {
			m.handleResumeTopic()
			return nil
		}
		if input == "/todo" {
			m.toggleTodoPopup()
			return nil
		}
		if input == "/schedule" {
			m.toggleSchedulePopup()
			return nil
		}
		if input == "/model" || input == "/models" {
			m.toggleModelPopup()
			return nil
		}
		if strings.HasPrefix(input, "/model ") {
			m.handleModelString(strings.TrimSpace(strings.TrimPrefix(input, "/model ")))
			return nil
		}
		if u := parseOpenLink(input, m.lastSearchURLs); u != "" {
			m.handleOpenURL(u)
			return nil
		}
		if input == "/topic" || input == "/topics" {
			m.toggleTopicPopup()
			return nil
		}
		if strings.HasPrefix(input, "/topic ") {
			m.handleSwitchTopic(strings.TrimSpace(strings.TrimPrefix(input, "/topic ")))
			return nil
		}

		m.logger.Info("INPUT", fmt.Sprintf("User: %s", input))
		m.cc.lastSubmitted = input
		if !m.system.routerOnline {
			m.cc.conversations = append(m.cc.conversations, conversationEntry{input: input, response: "Router offline"})
			m.cc.convView.GotoBottom()
		} else {
			m.cc.setThinking(input)
			m.cc.convView.GotoBottom()
			m.taskMgr.AddOnce(input)
		}
	}
	return nil
}

func (m *Model) handleArrowUp() tea.Cmd {
	if m.todo.popupOpen {
		if m.todo.selIdx > 0 {
			m.todo.selIdx--
		}
		return nil
	}
	if m.schedule.popupOpen {
		if m.schedule.selIdx > 0 {
			m.schedule.selIdx--
		}
		return nil
	}
	if m.topic.popupOpen {
		if m.topic.selIdx > 0 {
			m.topic.selIdx--
		}
		return nil
	}
	if m.model.popupOpen {
		if m.model.selIdx > 0 {
			m.model.selIdx--
		}
		return nil
	}
	switch m.focused {
	case focusLogs:
		m.log.view.ScrollUp(3)
	case focusConv:
		m.cc.convView.ScrollUp(3)
	case focusTopics:
		if m.topic.recentIdx > 0 {
			m.topic.recentIdx--
		}
	case focusInput:
		if m.cc.textInput.Value() == "" && m.cc.lastSubmitted != "" {
			m.cc.textInput.SetValue(m.cc.lastSubmitted)
			m.cc.textInput.SetCursor(len(m.cc.lastSubmitted))
		}
	}
	return nil
}

func (m *Model) handleArrowDown() tea.Cmd {
	if m.todo.popupOpen {
		if m.todo.selIdx < len(m.todo.items)-1 {
			m.todo.selIdx++
		}
		return nil
	}
	if m.schedule.popupOpen {
		if m.schedule.selIdx < len(m.schedule.items)-1 {
			m.schedule.selIdx++
		}
		return nil
	}
	if m.topic.popupOpen {
		if m.topic.selIdx < len(m.topic.filtered)-1 {
			m.topic.selIdx++
		}
		return nil
	}
	if m.model.popupOpen {
		if m.model.selIdx < len(m.model.filtered)-1 {
			m.model.selIdx++
		}
		return nil
	}
	switch m.focused {
	case focusLogs:
		m.log.view.ScrollDown(3)
	case focusConv:
		m.cc.convView.ScrollDown(3)
	case focusTopics:
		all := m.topicMgr.ListTopics()
		limit := 5
		if len(all) > limit {
			all = all[:limit]
		}
		if m.topic.recentIdx < len(all)-1 {
			m.topic.recentIdx++
		}
	}
	return nil
}

func (m *Model) handlePageUp() tea.Cmd {
	switch m.focused {
	case focusLogs:
		m.log.view.ScrollUp(m.log.view.Height())
	case focusConv:
		m.cc.convView.ScrollUp(m.cc.convView.Height())
	case focusTopics:
		m.topic.view.ScrollUp(m.topic.view.Height())
	}
	return nil
}

func (m *Model) handlePageDown() tea.Cmd {
	switch m.focused {
	case focusLogs:
		m.log.view.ScrollDown(m.log.view.Height())
	case focusConv:
		m.cc.convView.ScrollDown(m.cc.convView.Height())
	case focusTopics:
		m.topic.view.ScrollDown(m.topic.view.Height())
	}
	return nil
}

// ─── Mouse Handling ───

func (m *Model) handleMouseWheel(mouse tea.Mouse) {
	if mouse.Button == tea.MouseWheelUp {
		switch m.focused {
		case focusLogs:
			m.log.view.ScrollUp(3)
		case focusConv:
			m.cc.convView.ScrollUp(3)
		case focusTopics:
			m.topic.view.ScrollUp(3)
		}
	} else if mouse.Button == tea.MouseWheelDown {
		switch m.focused {
		case focusLogs:
			m.log.view.ScrollDown(3)
		case focusConv:
			m.cc.convView.ScrollDown(3)
		case focusTopics:
			m.topic.view.ScrollDown(3)
		}
	}
}

func (m *Model) handleMouseClick(mouse tea.Mouse) tea.Cmd {
	if mouse.Button != tea.MouseLeft {
		return nil
	}
	if mouse.X < 30 {
		return nil
	}
	if m.topic.popupOpen {
		itemY := mouse.Y - 2 - 1
		if itemY >= 0 && itemY < len(m.topic.filtered) {
			m.toggleTopicPopup()
			m.handleSwitchTopic(m.topic.filtered[itemY].ID)
		}
	}
	if m.model.popupOpen {
		itemY := mouse.Y - 2 - 1
		if itemY >= 0 && itemY < len(m.model.filtered) {
			m.toggleModelPopup()
			m.handleModelSelect(m.model.filtered[itemY].Name)
		}
	}
	return nil
}

// ─── Slash Completion ───

var slashCommands = []string{
	"/good", "/bad", "/log", "/logs", "/quit",
	"/new", "/fork", "/resume", "/topic", "/topics",
	"/model", "/models", "/todo", "/schedule", "/help", "/?",
}

func (m *Model) computeSlashMatches(prefix string) {
	m.cc.slashMatches = nil
	m.cc.slashIndex = 0
	if !strings.HasPrefix(prefix, "/") || prefix == "/" {
		return
	}
	if strings.HasPrefix(prefix, "/topic ") || prefix == "/topic" {
		m.computeTopicMatches(prefix)
		return
	}
	if strings.HasPrefix(prefix, "/model ") || prefix == "/model" {
		m.computeModelMatches(prefix)
		return
	}
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd, prefix) && cmd != prefix {
			m.cc.slashMatches = append(m.cc.slashMatches, cmd)
		}
	}
}

func (m *Model) computeModelMatches(prefix string) {
	models := m.settingsMgr.ListModels()
	for i := range models {
		label := fmt.Sprintf("/model %d", i+1)
		if strings.HasPrefix(label, prefix) {
			m.cc.slashMatches = append(m.cc.slashMatches, label)
		}
	}
}

func (m *Model) computeTopicMatches(prefix string) {
	topics := m.topicMgr.ListTopics()
	for i := range topics {
		label := fmt.Sprintf("/topic %d", i+1)
		if strings.HasPrefix(label, prefix) {
			m.cc.slashMatches = append(m.cc.slashMatches, label)
		}
	}
}

// ─── Popup Toggles ───

func (m *Model) toggleTopicPopup() {
	m.topic.popupOpen = !m.topic.popupOpen
	if m.topic.popupOpen {
		m.topic.filter = ""
		m.topic.selIdx = 0
		m.cc.textInput.SetValue("")
		m.cc.textInput.Placeholder = " Search topics (↑↓ navigate, Enter/Tab select)"
		m.topic.filterTopics(m.topicMgr.ListTopics())
		m.focused = focusInput
		m.cc.textInput.Focus()
		m.topic.view.GotoBottom()
	} else {
		m.topic.filter = ""
		m.cc.textInput.Placeholder = " Type a command... (/help for help)"
		m.focused = focusInput
		m.cc.textInput.Focus()
	}
}

func (m *Model) toggleLogPopup() {
	m.log.popupOpen = !m.log.popupOpen
	if m.log.popupOpen {
		m.focused = focusLogs
		m.cc.textInput.Blur()
		m.log.view.GotoBottom()
	} else {
		m.focused = focusInput
		m.cc.textInput.Focus()
	}
}

func (m *Model) toggleHelpPopup() {
	m.help.popupOpen = !m.help.popupOpen
	if m.help.popupOpen {
		m.cc.textInput.Blur()
		m.focused = focusLogs
		m.help.view.GotoBottom()
	} else {
		m.focused = focusInput
		m.cc.textInput.Focus()
	}
}

// ─── Model Popup / Commands ───

func (m *Model) toggleModelPopup() {
	m.model.popupOpen = !m.model.popupOpen
	if m.model.popupOpen {
		m.model.filter = ""
		m.model.selIdx = 0
		m.cc.textInput.SetValue("")
		m.cc.textInput.Placeholder = " Search models (↑↓ navigate, Enter/Tab select)"
		m.model.filterModels(m.settingsMgr.ListModels())
		m.focused = focusInput
		m.cc.textInput.Focus()
		m.model.view.GotoBottom()
	} else {
		m.model.filter = ""
		m.cc.textInput.Placeholder = " Type a command... (/help for help)"
		m.focused = focusInput
		m.cc.textInput.Focus()
	}
}

func (m *Model) handleModelString(arg string) {
	models := m.settingsMgr.ListModels()
	if n, err := strconv.Atoi(arg); err == nil && n >= 1 && n <= len(models) {
		m.handleModelSelect(models[n-1].Name)
		return
	}
	argLower := strings.ToLower(arg)
	for _, mdl := range models {
		if strings.Contains(strings.ToLower(mdl.Name), argLower) {
			m.handleModelSelect(mdl.Name)
			return
		}
	}
	msg := fmt.Sprintf("Model not found: %s", arg)
	m.logger.Info("MODEL", msg)
	m.events <- logMsg{message: fmt.Sprintf("[MODEL][INFO] %s", msg)}
}

func (m *Model) handleModelSelect(name string) {
	if err := m.settingsMgr.SelectModel(name); err != nil {
		m.logger.Info("MODEL", fmt.Sprintf("Failed to select model: %v", err))
		m.events <- logMsg{message: fmt.Sprintf("[MODEL][ERROR] %v", err)}
		return
	}
	m.router.SetModel(name)
	m.router.SetContextSize(m.settingsMgr.ContextLength())
	m.executor.SetModel(name)
	m.executor.SetContextSize(m.settingsMgr.ContextLength())
	m.feedback.SetModel(name)

	msg := fmt.Sprintf("Switched to model: %s (%dK ctx)", name, m.settingsMgr.ContextLength()/1024)
	m.logger.Info("MODEL", msg)
	m.events <- logMsg{message: fmt.Sprintf("[MODEL][INFO] %s", msg)}
}

func (m *Model) selectRecentModel() {
	all := m.settingsMgr.ListModels()
	limit := 5
	if len(all) > limit {
		all = all[:limit]
	}
	if m.model.recentIdx >= 0 && m.model.recentIdx < len(all) {
		m.handleModelSelect(all[m.model.recentIdx].Name)
	}
}

func (m *Model) selectCurrentModel() {
	if len(m.model.filtered) == 0 {
		return
	}
	idx := m.model.selIdx
	if idx < 0 || idx >= len(m.model.filtered) {
		return
	}
	m.toggleModelPopup()
	m.handleModelSelect(m.model.filtered[idx].Name)
}

func (m *Model) handleOpenURL(url string) {
	m.logger.Info("OPEN", fmt.Sprintf("Opening URL: %s", url))
	if m.cc.thinking > 0 {
		return
	}
	m.cc.lastSubmitted = "open " + url
	m.cc.setThinking("open " + url)
	m.cc.convView.GotoBottom()
	m.taskMgr.AddOnce("open " + url)
}

// ─── Topic Commands ───

func (m *Model) handleNewTopic() {
	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		m.logger.Info("TOPIC", fmt.Sprintf("Creating new topic (was: %s)", cur.Title))
	}

	m.topicMgr.CreateTopic("Topic N")
	m.cc.setConversationsFromTurns(nil)
	m.cc.convView.GotoBottom()

	msg := fmt.Sprintf("Started new topic")
	m.logger.Info("TOPIC", msg)
	m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] %s", msg)}
}

func (m *Model) handleForkTopic() {
	cur := m.topicMgr.CurrentTopic()
	if cur == nil {
		return
	}

	t := m.topicMgr.ForkTopic("Fork of " + cur.Title)
	turns := m.topicMgr.LoadTopicTurns(t.ID)
	convTurns := make([]conversationEntry, len(turns))
	for i, turn := range turns {
		convTurns[i] = conversationEntry{input: turn.Input, response: stripMarkdown(turn.Response)}
	}
	m.cc.setConversationsFromTurns(convTurns)
	m.cc.convView.GotoBottom()

	msg := fmt.Sprintf("Forked topic: %s → %s", cur.Title, t.Title)
	m.logger.Info("TOPIC", msg)
	m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] %s", msg)}
}

func (m *Model) handleResumeTopic() {
	t := m.topicMgr.ResumeTopic()
	if t == nil {
		msg := "No previous topic to resume"
		m.logger.Info("TOPIC", msg)
		m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] %s", msg)}
		return
	}

	turns := m.topicMgr.LoadTopicTurns(t.ID)
	convTurns := make([]conversationEntry, len(turns))
	for i, turn := range turns {
		convTurns[i] = conversationEntry{input: turn.Input, response: stripMarkdown(turn.Response)}
	}
	m.cc.setConversationsFromTurns(convTurns)
	m.cc.convView.GotoBottom()

	msg := fmt.Sprintf("Resumed topic: %s (%s)", t.Title, t.ID)
	m.logger.Info("TOPIC", msg)
	m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] %s", msg)}
}

func (m *Model) handleSwitchTopic(arg string) {
	topics := m.topicMgr.ListTopics()

	if n, err := strconv.Atoi(arg); err == nil && n >= 1 && n <= len(topics) {
		arg = topics[n-1].ID
	}

	if m.topicMgr.SwitchToTopic(arg) {
		cur := m.topicMgr.CurrentTopic()
		turns := m.topicMgr.LoadTopicTurns(cur.ID)
		convTurns := make([]conversationEntry, len(turns))
		for i, turn := range turns {
			convTurns[i] = conversationEntry{input: turn.Input, response: stripMarkdown(turn.Response)}
		}
		m.cc.setConversationsFromTurns(convTurns)
		m.cc.convView.GotoBottom()

		msg := fmt.Sprintf("Switched to topic: %s (%d turns)", cur.Title, len(turns))
		m.logger.Info("TOPIC", msg)
		m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] %s", msg)}
	} else {
		msg := fmt.Sprintf("Topic not found: %s", arg)
		m.logger.Info("TOPIC", msg)
		m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] %s", msg)}
	}
}

func (m *Model) selectRecentTopic() {
	all := m.topicMgr.ListTopics()
	limit := 5
	if len(all) > limit {
		all = all[:limit]
	}
	if m.topic.recentIdx >= 0 && m.topic.recentIdx < len(all) {
		m.handleSwitchTopic(all[m.topic.recentIdx].ID)
	}
}

func (m *Model) selectCurrentTopic() {
	if len(m.topic.filtered) == 0 {
		return
	}
	idx := m.topic.selIdx
	if idx < 0 || idx >= len(m.topic.filtered) {
		return
	}
	m.toggleTopicPopup()
	m.handleSwitchTopic(m.topic.filtered[idx].ID)
}

// ─── Tracker Popup Toggles ───

func (m *Model) toggleTodoPopup() {
	m.todo.popupOpen = !m.todo.popupOpen
	if m.todo.popupOpen {
		m.todo.tabIdx = 0
		m.todo.selIdx = 0
		m.todo.loadItems(m.todoMgr)
		m.todo.view.GotoBottom()
		m.focused = focusLogs
		m.cc.textInput.Blur()
	} else {
		m.focused = focusInput
		m.cc.textInput.Focus()
	}
}

func (m *Model) toggleSchedulePopup() {
	m.schedule.popupOpen = !m.schedule.popupOpen
	if m.schedule.popupOpen {
		m.schedule.tabIdx = 0
		m.schedule.selIdx = 0
		m.schedule.loadItems(m.taskMgr)
		m.schedule.view.GotoBottom()
		m.focused = focusLogs
		m.cc.textInput.Blur()
	} else {
		m.focused = focusInput
		m.cc.textInput.Focus()
	}
}

func (m *Model) todoRemoveSelected() tea.Cmd {
	if m.todo.selIdx < 0 || m.todo.selIdx >= len(m.todo.items) {
		return nil
	}
	m.todoMgr.Delete(m.todo.items[m.todo.selIdx].ID)
	m.todo.loadItems(m.todoMgr)
	if m.todo.selIdx >= len(m.todo.items) && m.todo.selIdx > 0 {
		m.todo.selIdx--
	}
	return nil
}

func (m *Model) todoDoneSelected() tea.Cmd {
	if m.todo.selIdx < 0 || m.todo.selIdx >= len(m.todo.items) {
		return nil
	}
	id := m.todo.items[m.todo.selIdx].ID
	if m.todo.tabIdx == 0 {
		m.todoMgr.Complete(id)
	} else {
		m.todoMgr.Uncomplete(id)
	}
	m.todo.loadItems(m.todoMgr)
	if m.todo.selIdx >= len(m.todo.items) && m.todo.selIdx > 0 {
		m.todo.selIdx--
	}
	return nil
}

func (m *Model) scheduleRemoveSelected() tea.Cmd {
	if m.schedule.selIdx < 0 || m.schedule.selIdx >= len(m.schedule.items) {
		return nil
	}
	m.taskMgr.Delete(m.schedule.items[m.schedule.selIdx].ID)
	m.schedule.loadItems(m.taskMgr)
	if m.schedule.selIdx >= len(m.schedule.items) && m.schedule.selIdx > 0 {
		m.schedule.selIdx--
	}
	return nil
}

func (m *Model) scheduleResetSelected() tea.Cmd {
	if m.schedule.selIdx < 0 || m.schedule.selIdx >= len(m.schedule.items) {
		return nil
	}
	if m.schedule.tabIdx == 0 {
		return nil
	}
	id := m.schedule.items[m.schedule.selIdx].ID
	_ = m.taskMgr.Update(id, func(t *task.Task) {
		t.Status = task.StatusWaiting
	})
	m.schedule.loadItems(m.taskMgr)
	if m.schedule.selIdx >= len(m.schedule.items) && m.schedule.selIdx > 0 {
		m.schedule.selIdx--
	}
	return nil
}

func (m *Model) handleArrowLeft() tea.Cmd {
	if m.todo.popupOpen {
		if m.todo.tabIdx > 0 {
			m.todo.tabIdx--
			m.todo.selIdx = 0
			m.todo.loadItems(m.todoMgr)
			m.todo.view.GotoBottom()
		}
		return nil
	}
	if m.schedule.popupOpen {
		if m.schedule.tabIdx > 0 {
			m.schedule.tabIdx--
			m.schedule.selIdx = 0
			m.schedule.loadItems(m.taskMgr)
			m.schedule.view.GotoBottom()
		}
		return nil
	}
	return nil
}

func (m *Model) handleArrowRight() tea.Cmd {
	if m.todo.popupOpen {
		if m.todo.tabIdx < 1 {
			m.todo.tabIdx++
			m.todo.selIdx = 0
			m.todo.loadItems(m.todoMgr)
			m.todo.view.GotoBottom()
		}
		return nil
	}
	if m.schedule.popupOpen {
		if m.schedule.tabIdx < 1 {
			m.schedule.tabIdx++
			m.schedule.selIdx = 0
			m.schedule.loadItems(m.taskMgr)
			m.schedule.view.GotoBottom()
		}
		return nil
	}
	return nil
}
