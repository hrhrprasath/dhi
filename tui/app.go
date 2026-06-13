package tui

import (
	"fmt"
	"strconv"
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

type FeedbackRecorder interface {
	RecordOutcome(input, predictedEvent, pipelineUsed string, confidence float64, success bool, wasConversation bool, errMsg string)
	RecordConversation(input string)
	RecordRating(input string, positive bool)
}

type focusPanel int

const (
	focusLogs focusPanel = iota
	focusInput
	focusConv
	focusTopics
)

type Model struct {
	cfg      *config.Config
	logger   *services.Logger
	taskMgr  *task.Manager
	router   *intent.Router
	executor *pipeline.Executor
	topicMgr *topic.Manager
	sysSvc   *services.SystemService
	todoMgr  *todo.Manager
	feedback FeedbackRecorder

	width    int
	height   int
	events   chan any
	focused  focusPanel
	focusCmd tea.Cmd

	lastInput  string
	lastIntent *intent.ParsedIntent

	dhiOS    dhiOS
	system   systemPanel
	pipeline pipelinePanel
	cc       commandCenter
	topic    topicPanel
	log      logPanel
}

func NewModel(cfg *config.Config, logger *services.Logger, tm *task.Manager,
	router *intent.Router, pipeExec *pipeline.Executor, topicMgr *topic.Manager, sysSvc *services.SystemService, todoMgr *todo.Manager, feedback FeedbackRecorder) *Model {

	ti := textinput.New()
	ti.Placeholder = " Type a command... (/topic → topics /log → logs)"
	ti.Prompt = "> "
	ti.SetWidth(34)
	focusCmd := ti.Focus()

	initW := 40
	cv := viewport.New(viewport.WithWidth(initW), viewport.WithHeight(10))
	cv.SetContent("Start a conversation...")

	pv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(6))
	tv := viewport.New(viewport.WithWidth(26), viewport.WithHeight(8))
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
		cfg:      cfg,
		logger:   logger,
		taskMgr:  tm,
		router:   router,
		executor: pipeExec,
		topicMgr: topicMgr,
		sysSvc:   sysSvc,
		todoMgr:  todoMgr,
		feedback: feedback,

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
		log: logPanel{
			view:    lv,
			entries: []string{"[SYSTEM][INFO] DHI Harness initializing..."},
		},
	}

	m.topic.filterTopics(m.topicMgr.ListTopics())
	m.pipeline.render()

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
		m.cc.textInput.SetWidth(rw - 16)
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
	rw := m.rightWidth()

	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		m.dhiOS.render(),
		m.system.render(),
		m.pipeline.render(),
		m.topic.renderRecent(m.topicMgr.ListTopics(), m.topicMgr.CurrentTopic(), m.focused == focusTopics, m.height),
	)

	var mainBlock string
	switch {
	case m.log.popupOpen:
		mainBlock = m.log.renderPopup(rw)
	case m.topic.popupOpen:
		mainBlock = m.topic.renderPopup(rw, m.topicMgr.CurrentTopic())
	default:
		topicTitle := ""
		if cur := m.topicMgr.CurrentTopic(); cur != nil {
			topicTitle = cur.Title
		}
		mainBlock = m.cc.renderConv(rw, m.focused == focusConv, topicTitle)
	}

	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		mainBlock,
		m.cc.renderInput(rw, m.system.routerOnline, m.focused == focusInput),
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

		return cmd
	}
	return nil
}

func (m *Model) handleEscape() tea.Cmd {
	if m.topic.popupOpen {
		m.toggleTopicPopup()
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
	if m.focused == focusInput {
		input := strings.TrimSpace(m.cc.textInput.Value())

		if len(m.cc.slashMatches) > 0 && strings.HasPrefix(input, "/") && input != m.cc.slashMatches[m.cc.slashIndex] {
			input = m.cc.slashMatches[m.cc.slashIndex]
		}

		if m.topic.popupOpen {
			m.selectCurrentTopic()
			return nil
		}

		m.cc.textInput.SetValue("")
		m.cc.slashMatches = nil
		m.cc.slashIndex = 0

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
		if input == "/topic" || input == "/topics" {
			m.toggleTopicPopup()
			return nil
		}
		if strings.HasPrefix(input, "/topic ") {
			m.handleSwitchTopic(strings.TrimSpace(strings.TrimPrefix(input, "/topic ")))
			return nil
		}

		m.logger.Info("INPUT", fmt.Sprintf("User: %s", input))
		if input != "" {
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
	}
	return nil
}

func (m *Model) handleArrowUp() tea.Cmd {
	if m.topic.popupOpen {
		if m.topic.selIdx > 0 {
			m.topic.selIdx--
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
	if m.topic.popupOpen {
		if m.topic.selIdx < len(m.topic.filtered)-1 {
			m.topic.selIdx++
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
	return nil
}

// ─── Slash Completion ───

var slashCommands = []string{
	"/good", "/bad", "/log", "/logs", "/quit",
	"/new", "/fork", "/resume", "/topic", "/topics",
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
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd, prefix) && cmd != prefix {
			m.cc.slashMatches = append(m.cc.slashMatches, cmd)
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
		m.cc.textInput.Placeholder = " Type a command... (/topic → topics /log → logs)"
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
