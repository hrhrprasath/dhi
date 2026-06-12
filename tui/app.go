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
	"dhi_harness/tui/components"

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
	// Dependencies
	cfg      *config.Config
	logger   *services.Logger
	taskMgr  *task.Manager
	router   *intent.Router
	executor *pipeline.Executor
	topicMgr *topic.Manager
	sysSvc   *services.SystemService
	todoMgr  *todo.Manager
	feedback FeedbackRecorder

	// Layout
	width    int
	height   int
	events   chan interface{}
	focused  focusPanel
	focusCmd tea.Cmd

	// Last intent tracking
	lastInput  string
	lastIntent *intent.ParsedIntent

	// Components
	DHIOS   components.DHIOS
	System  components.System
	Pipeline components.Pipeline
	CC      components.CommandCenter
	Topic   components.Topic
	Log     components.Log
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
		spinner.WithStyle(lipgloss.NewStyle().Foreground(components.AccentColor)),
	)

	metrics := sysSvc.GetMetrics()

	statusPipelines := make([]components.PipelineEntry, 0)
	for _, p := range pipeExec.ListPipelines() {
		statusPipelines = append(statusPipelines, components.PipelineEntry{Name: p, Status: "IDLE"})
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
		events:   make(chan interface{}, 200),
		width:    80,
		height:   30,

		DHIOS: components.DHIOS{
			Hostname:  metrics.Hostname,
			Kernel:    metrics.Kernel,
			UptimeSec: metrics.UptimeSec,
		},
		System: components.System{
			Metrics:      metrics,
			RouterStatus: "OFFLINE",
			CPUProgress:  cp,
			RAMProgress:  rp,
			GPUProgress:  gp,
		},
		Pipeline: components.Pipeline{
			Entries: statusPipelines,
			View:    pv,
		},
		CC: components.CommandCenter{
			Spinner:   s,
			TextInput: ti,
			ConvView:  cv,
		},
		Topic: components.Topic{
			View: tv,
		},
		Log: components.Log{
			View: lv,
			Entries: []string{"[SYSTEM][INFO] DHI Harness initializing..."},
		},
	}

	m.Topic.FilterTopics(m.topicMgr.ListTopics())
	m.Pipeline.Render()

	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		turns := m.topicMgr.LoadTopicTurns(cur.ID)
		convTurns := make([]components.ConversationEntry, len(turns))
		for i, t := range turns {
			convTurns[i] = components.ConversationEntry{Input: t.Input, Response: stripMarkdown(t.Response)}
		}
		m.CC.SetConversationsFromTurns(convTurns)
		m.CC.ConvView.GotoBottom()
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
		func() tea.Msg { return m.CC.Spinner.Tick() },
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
		ch := components.Max(msg.Height-7, 3)
		lh := components.Max(ch-1, 6)
		m.Log.View.SetWidth(vw)
		m.Log.View.SetHeight(lh)
		m.CC.ConvView.SetWidth(vw)
		m.CC.ConvView.SetHeight(ch)
		m.Topic.View.SetWidth(vw - 2)
		m.Topic.View.SetHeight(ch)
		m.CC.TextInput.SetWidth(rw - 16)
		return m, m.eventPoller()

	case tickMsg:
		m.DHIOS.UptimeSec++
		m.System.Metrics = m.sysSvc.GetMetrics()
		return m, tea.Batch(
			m.eventPoller(),
			tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
		)

	case routerStatusMsg:
		m.System.RouterOnline = msg.online
		if msg.online {
			m.System.RouterStatus = "READY"
		} else {
			m.System.RouterStatus = "OFFLINE"
		}
		return m, m.eventPoller()

	case logMsg:
		m.Log.Append(msg.message)
		m.Log.View.GotoBottom()
		return m, m.eventPoller()

	case convMsg:
		if !m.CC.SetResponse(msg.input, stripMarkdown(msg.response)) {
			m.CC.Conversations = append(m.CC.Conversations, components.ConversationEntry{
				Input:    msg.input,
				Response: stripMarkdown(msg.response),
			})
		}
		m.CC.ConvView.GotoBottom()
		return m, m.eventPoller()

	case pipelineStatusMsg:
		for i, p := range m.Pipeline.Entries {
			if p.Name == msg.name {
				m.Pipeline.Entries[i].Status = msg.status
				return m, m.eventPoller()
			}
		}
		m.Pipeline.Entries = append(m.Pipeline.Entries, components.PipelineEntry{Name: msg.name, Status: msg.status})
		return m, m.eventPoller()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.CC.Spinner, cmd = m.CC.Spinner.Update(msg)
		if m.CC.UpdateSpinnerResponse() {
			m.CC.ConvView.GotoBottom()
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
		m.DHIOS.Render(),
		m.System.Render(),
		m.Pipeline.Render(),
		m.Topic.RenderRecent(m.topicMgr.ListTopics(), m.topicMgr.CurrentTopic(), m.focused == focusTopics, m.height),
	)

	var mainBlock string
	switch {
	case m.Log.PopupOpen:
		mainBlock = m.Log.RenderPopup(rw)
	case m.Topic.PopupOpen:
		mainBlock = m.Topic.RenderPopup(rw, m.topicMgr.CurrentTopic())
	default:
		topicTitle := ""
		if cur := m.topicMgr.CurrentTopic(); cur != nil {
			topicTitle = cur.Title
		}
		mainBlock = m.CC.RenderConv(rw, m.focused == focusConv, topicTitle)
	}

	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		mainBlock,
		m.CC.RenderInput(rw, m.System.RouterOnline, m.focused == focusInput),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftPanel,
		"  ",
		rightPanel,
	)
}

func (m *Model) rightWidth() int {
	return components.Max(m.width-30, 40)
}

// ─── Keyboard Handling ───

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		fallthrough
	case msg.Code == 'd' && msg.Mod == tea.ModCtrl:
		return tea.Quit

	case msg.Code == tea.KeyEscape:
		if m.Topic.PopupOpen {
			m.toggleTopicPopup()
			return nil
		}
		if m.Log.PopupOpen {
			m.Log.PopupOpen = false
			if m.focused == focusLogs {
				m.focused = focusInput
				return m.CC.TextInput.Focus()
			}
		}
		return nil

	case msg.Code == tea.KeyTab:
		if m.Topic.PopupOpen {
			m.selectCurrentTopic()
			return nil
		}
		if m.focused == focusInput {
			val := m.CC.TextInput.Value()
			if strings.HasPrefix(val, "/") {
				m.computeSlashMatches(val)
				if len(m.CC.SlashMatches) > 0 {
					m.CC.SlashIndex = (m.CC.SlashIndex + 1) % len(m.CC.SlashMatches)
					m.CC.TextInput.SetValue(m.CC.SlashMatches[m.CC.SlashIndex])
					m.CC.TextInput.SetCursor(len(m.CC.SlashMatches[m.CC.SlashIndex]))
				}
				return nil
			}
		}
		switch m.focused {
		case focusLogs:
			m.focused = focusInput
			return m.CC.TextInput.Focus()
		case focusInput:
			m.focused = focusConv
			m.CC.TextInput.Blur()
			return nil
		case focusConv:
			if m.Log.PopupOpen {
				m.focused = focusLogs
				m.CC.TextInput.Blur()
				m.Log.View.GotoBottom()
				return nil
			}
			m.focused = focusTopics
			m.CC.TextInput.Blur()
			return nil
		case focusTopics:
			m.focused = focusInput
			return m.CC.TextInput.Focus()
		}

	case msg.Code == tea.KeyEnter:
		if m.focused == focusTopics {
			m.selectRecentTopic()
			return nil
		}
		if m.focused == focusInput {
			input := strings.TrimSpace(m.CC.TextInput.Value())

			if len(m.CC.SlashMatches) > 0 && strings.HasPrefix(input, "/") && input != m.CC.SlashMatches[m.CC.SlashIndex] {
				input = m.CC.SlashMatches[m.CC.SlashIndex]
			}

			if m.Topic.PopupOpen {
				m.selectCurrentTopic()
				return nil
			}

			m.CC.TextInput.SetValue("")
			m.CC.SlashMatches = nil
			m.CC.SlashIndex = 0

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
				m.CC.LastSubmitted = input
				if !m.System.RouterOnline {
					m.CC.Conversations = append(m.CC.Conversations, components.ConversationEntry{Input: input, Response: "Router offline"})
					m.CC.ConvView.GotoBottom()
				} else {
					m.CC.SetThinking(input)
					m.CC.ConvView.GotoBottom()
					m.taskMgr.AddOnce(input)
				}
			}
		}
		return nil

	case msg.Code == tea.KeyUp:
		if m.Topic.PopupOpen {
			if m.Topic.SelIdx > 0 {
				m.Topic.SelIdx--
			}
			return nil
		}
		switch m.focused {
		case focusLogs:
			m.Log.View.ScrollUp(3)
		case focusConv:
			m.CC.ConvView.ScrollUp(3)
		case focusTopics:
			if m.Topic.RecentIdx > 0 {
				m.Topic.RecentIdx--
			}
		case focusInput:
			if m.CC.TextInput.Value() == "" && m.CC.LastSubmitted != "" {
				m.CC.TextInput.SetValue(m.CC.LastSubmitted)
				m.CC.TextInput.SetCursor(len(m.CC.LastSubmitted))
			}
		}
		return nil

	case msg.Code == tea.KeyDown:
		if m.Topic.PopupOpen {
			if m.Topic.SelIdx < len(m.Topic.Filtered)-1 {
				m.Topic.SelIdx++
			}
			return nil
		}
		switch m.focused {
		case focusLogs:
			m.Log.View.ScrollDown(3)
		case focusConv:
			m.CC.ConvView.ScrollDown(3)
		case focusTopics:
			all := m.topicMgr.ListTopics()
			limit := 5
			if len(all) > limit {
				all = all[:limit]
			}
			if m.Topic.RecentIdx < len(all)-1 {
				m.Topic.RecentIdx++
			}
		}
		return nil

	case msg.Code == tea.KeyPgUp:
		switch m.focused {
		case focusLogs:
			m.Log.View.ScrollUp(m.Log.View.Height())
		case focusConv:
			m.CC.ConvView.ScrollUp(m.CC.ConvView.Height())
		case focusTopics:
			m.Topic.View.ScrollUp(m.Topic.View.Height())
		}
		return nil

	case msg.Code == tea.KeyPgDown:
		switch m.focused {
		case focusLogs:
			m.Log.View.ScrollDown(m.Log.View.Height())
		case focusConv:
			m.CC.ConvView.ScrollDown(m.CC.ConvView.Height())
		case focusTopics:
			m.Topic.View.ScrollDown(m.Topic.View.Height())
		}
		return nil
	}

	if m.focused == focusInput {
		var cmd tea.Cmd
		m.CC.TextInput, cmd = m.CC.TextInput.Update(msg)

		if m.Topic.PopupOpen {
			oldFilter := m.Topic.Filter
			m.Topic.Filter = m.CC.TextInput.Value()
			if m.Topic.Filter != oldFilter {
				m.Topic.FilterTopics(m.topicMgr.ListTopics())
				m.Topic.SelIdx = 0
			}
		}

		return cmd
	}
	return nil
}

// ─── Mouse Handling ───

func (m *Model) handleMouseWheel(mouse tea.Mouse) {
	if mouse.Button == tea.MouseWheelUp {
		switch m.focused {
		case focusLogs:
			m.Log.View.ScrollUp(3)
		case focusConv:
			m.CC.ConvView.ScrollUp(3)
		case focusTopics:
			m.Topic.View.ScrollUp(3)
		}
	} else if mouse.Button == tea.MouseWheelDown {
		switch m.focused {
		case focusLogs:
			m.Log.View.ScrollDown(3)
		case focusConv:
			m.CC.ConvView.ScrollDown(3)
		case focusTopics:
			m.Topic.View.ScrollDown(3)
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
	if m.Topic.PopupOpen {
		itemY := mouse.Y - 2 - 1
		if itemY >= 0 && itemY < len(m.Topic.Filtered) {
			m.toggleTopicPopup()
			m.handleSwitchTopic(m.Topic.Filtered[itemY].ID)
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
	m.CC.SlashMatches = nil
	m.CC.SlashIndex = 0
	if !strings.HasPrefix(prefix, "/") || prefix == "/" {
		return
	}
	if strings.HasPrefix(prefix, "/topic ") || prefix == "/topic" {
		m.computeTopicMatches(prefix)
		return
	}
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd, prefix) && cmd != prefix {
			m.CC.SlashMatches = append(m.CC.SlashMatches, cmd)
		}
	}
}

func (m *Model) computeTopicMatches(prefix string) {
	topics := m.topicMgr.ListTopics()
	for i := range topics {
		label := fmt.Sprintf("/topic %d", i+1)
		if strings.HasPrefix(label, prefix) {
			m.CC.SlashMatches = append(m.CC.SlashMatches, label)
		}
	}
}

// ─── Popup Toggles ───

func (m *Model) toggleTopicPopup() {
	m.Topic.PopupOpen = !m.Topic.PopupOpen
	if m.Topic.PopupOpen {
		m.Topic.Filter = ""
		m.Topic.SelIdx = 0
		m.CC.TextInput.SetValue("")
		m.CC.TextInput.Placeholder = " Search topics (↑↓ navigate, Enter/Tab select)"
		m.Topic.FilterTopics(m.topicMgr.ListTopics())
		m.focused = focusInput
		m.CC.TextInput.Focus()
		m.Topic.View.GotoBottom()
	} else {
		m.Topic.Filter = ""
		m.CC.TextInput.Placeholder = " Type a command... (/topic → topics /log → logs)"
		m.focused = focusInput
		m.CC.TextInput.Focus()
	}
}

func (m *Model) toggleLogPopup() {
	m.Log.PopupOpen = !m.Log.PopupOpen
	if m.Log.PopupOpen {
		m.focused = focusLogs
		m.CC.TextInput.Blur()
		m.Log.View.GotoBottom()
	} else {
		m.focused = focusInput
		m.CC.TextInput.Focus()
	}
}

// ─── Topic Commands ───

func (m *Model) handleNewTopic() {
	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		m.logger.Info("TOPIC", fmt.Sprintf("Creating new topic (was: %s)", cur.Title))
	}

	_ = m.topicMgr.CreateTopic("Topic N")
	m.CC.SetConversationsFromTurns(nil)
	m.CC.ConvView.GotoBottom()

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
	convTurns := make([]components.ConversationEntry, len(turns))
	for i, turn := range turns {
		convTurns[i] = components.ConversationEntry{Input: turn.Input, Response: stripMarkdown(turn.Response)}
	}
	m.CC.SetConversationsFromTurns(convTurns)
	m.CC.ConvView.GotoBottom()

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
	convTurns := make([]components.ConversationEntry, len(turns))
	for i, turn := range turns {
		convTurns[i] = components.ConversationEntry{Input: turn.Input, Response: stripMarkdown(turn.Response)}
	}
	m.CC.SetConversationsFromTurns(convTurns)
	m.CC.ConvView.GotoBottom()

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
		convTurns := make([]components.ConversationEntry, len(turns))
		for i, turn := range turns {
			convTurns[i] = components.ConversationEntry{Input: turn.Input, Response: stripMarkdown(turn.Response)}
		}
		m.CC.SetConversationsFromTurns(convTurns)
		m.CC.ConvView.GotoBottom()

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
	if m.Topic.RecentIdx >= 0 && m.Topic.RecentIdx < len(all) {
		m.handleSwitchTopic(all[m.Topic.RecentIdx].ID)
	}
}

func (m *Model) selectCurrentTopic() {
	if len(m.Topic.Filtered) == 0 {
		return
	}
	idx := m.Topic.SelIdx
	if idx < 0 || idx >= len(m.Topic.Filtered) {
		return
	}
	m.toggleTopicPopup()
	m.handleSwitchTopic(m.Topic.Filtered[idx].ID)
}
