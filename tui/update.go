package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dhi_harness/internal/intent"
	"dhi_harness/internal/services"
	"dhi_harness/internal/task"
	"dhi_harness/internal/topic"

	tea "charm.land/bubbletea/v2"
)

// ─── Key Handling ───

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case msg.Code == 'c' && msg.Mod == tea.ModCtrl:
		fallthrough
	case msg.Code == 'd' && msg.Mod == tea.ModCtrl:
		return tea.Quit

	case msg.Code == tea.KeyEscape:
		if m.topicPopupVisible {
			m.toggleTopicPopup()
			return nil
		}
		if m.logPopupVisible {
			m.logPopupVisible = false
			if m.focused == focusLogs {
				m.focused = focusInput
				return m.textInput.Focus()
			}
		}
		return nil

	case msg.Code == tea.KeyTab:
		if m.topicPopupVisible {
			m.selectCurrentTopic()
			return nil
		}
		if m.focused == focusInput {
			val := m.textInput.Value()
			if strings.HasPrefix(val, "/") {
				m.computeSlashMatches(val)
				if len(m.slashMatches) > 0 {
					m.slashIndex = (m.slashIndex + 1) % len(m.slashMatches)
					m.textInput.SetValue(m.slashMatches[m.slashIndex])
					m.textInput.SetCursor(len(m.slashMatches[m.slashIndex]))
				}
				return nil
			}
		}
		switch m.focused {
		case focusLogs:
			m.focused = focusInput
			return m.textInput.Focus()
		case focusInput:
			m.focused = focusConv
			m.textInput.Blur()
			return nil
		case focusConv:
			if m.logPopupVisible {
				m.focused = focusLogs
				m.textInput.Blur()
				m.logView.GotoBottom()
				return nil
			}
			m.focused = focusTopics
			m.textInput.Blur()
			return nil
		case focusTopics:
			m.focused = focusInput
			return m.textInput.Focus()
		}

	case msg.Code == tea.KeyEnter:
		if m.focused == focusTopics {
			m.selectRecentTopic()
			return nil
		}
		if m.focused == focusInput {
			input := strings.TrimSpace(m.textInput.Value())

			if len(m.slashMatches) > 0 && strings.HasPrefix(input, "/") && input != m.slashMatches[m.slashIndex] {
				input = m.slashMatches[m.slashIndex]
			}

			if m.topicPopupVisible {
				m.selectCurrentTopic()
				return nil
			}

			m.textInput.SetValue("")
			m.slashMatches = nil
			m.slashIndex = 0

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
				m.lastSubmitted = input
				if m.routerOnline {
					m.taskMgr.AddOnce(input)
				}
			}
		}
		return nil

	case msg.Code == tea.KeyUp:
		if m.topicPopupVisible {
			if m.topicSelIdx > 0 {
				m.topicSelIdx--
			}
			return nil
		}
		switch m.focused {
		case focusLogs:
			m.logView.ScrollUp(3)
		case focusConv:
			m.convView.ScrollUp(3)
		case focusTopics:
			if m.recentSelIdx > 0 {
				m.recentSelIdx--
			}
		case focusInput:
			if m.textInput.Value() == "" && m.lastSubmitted != "" {
				m.textInput.SetValue(m.lastSubmitted)
				m.textInput.SetCursor(len(m.lastSubmitted))
			}
		}
		return nil

	case msg.Code == tea.KeyDown:
		if m.topicPopupVisible {
			if m.topicSelIdx < len(m.topicFiltered)-1 {
				m.topicSelIdx++
			}
			return nil
		}
		switch m.focused {
		case focusLogs:
			m.logView.ScrollDown(3)
		case focusConv:
			m.convView.ScrollDown(3)
		case focusTopics:
			recent := m.topicMgr.ListTopics()
			limit := 5
			if len(recent) > limit {
				recent = recent[:limit]
			}
			if m.recentSelIdx < len(recent)-1 {
				m.recentSelIdx++
			}
		}
		return nil

	case msg.Code == tea.KeyPgUp:
		switch m.focused {
		case focusLogs:
			m.logView.ScrollUp(m.logView.Height())
		case focusConv:
			m.convView.ScrollUp(m.convView.Height())
		case focusTopics:
			m.topicView.ScrollUp(m.topicView.Height())
		}
		return nil

	case msg.Code == tea.KeyPgDown:
		switch m.focused {
		case focusLogs:
			m.logView.ScrollDown(m.logView.Height())
		case focusConv:
			m.convView.ScrollDown(m.convView.Height())
		case focusTopics:
			m.topicView.ScrollDown(m.topicView.Height())
		}
		return nil
	}

	if m.focused == focusInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)

		// If topic popup is visible, update filter on each keystroke
		if m.topicPopupVisible {
			oldFilter := m.topicFilter
			m.topicFilter = m.textInput.Value()
			if m.topicFilter != oldFilter {
				m.refreshTopicFiltered()
				m.topicSelIdx = 0
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
			m.logView.ScrollUp(3)
		case focusConv:
			m.convView.ScrollUp(3)
		case focusTopics:
			m.topicView.ScrollUp(3)
		}
	} else if mouse.Button == tea.MouseWheelDown {
		switch m.focused {
		case focusLogs:
			m.logView.ScrollDown(3)
		case focusConv:
			m.convView.ScrollDown(3)
		case focusTopics:
			m.topicView.ScrollDown(3)
		}
	}
}

func (m *Model) handleMouseClick(mouse tea.Mouse) tea.Cmd {
	if mouse.Button != tea.MouseLeft {
		return nil
	}
	// Left panel is ~30 chars wide
	if mouse.X < 30 {
		return nil
	}
	// Right panel — topic popup click handling
	if m.topicPopupVisible {
		itemY := mouse.Y - 2 - 1
		if itemY >= 0 && itemY < len(m.topicFiltered) {
			m.toggleTopicPopup()
			m.handleSwitchTopic(m.topicFiltered[itemY].ID)
		}
	}
	return nil
}

// ─── Topic Popup ───

func (m *Model) toggleTopicPopup() {
	m.topicPopupVisible = !m.topicPopupVisible
	if m.topicPopupVisible {
		m.topicFilter = ""
		m.topicSelIdx = 0
		m.textInput.SetValue("")
		m.textInput.Placeholder = " Search topics (↑↓ navigate, Enter/Tab select)"
		m.refreshTopicFiltered()
		m.focused = focusInput
		m.textInput.Focus()
		m.topicView.GotoBottom()
	} else {
		m.topicFilter = ""
		m.textInput.Placeholder = " Type a command... (/topic → topics /log → logs)"
		m.focused = focusInput
		m.textInput.Focus()
	}
}

// ─── Topic Commands ───

func (m *Model) handleNewTopic() {
	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		m.logger.Info("TOPIC", fmt.Sprintf("Creating new topic (was: %s)", cur.Title))
	}

	t := m.topicMgr.CreateTopic("Topic N")
	m.conversations = nil
	m.refreshConvContent()
	m.convView.GotoBottom()

	msg := fmt.Sprintf("Started new topic: %s (%s)", t.Title, t.ID)
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
	m.conversations = nil
	for _, turn := range turns {
		m.conversations = append(m.conversations, conversationEntry{Input: turn.Input, Response: stripMarkdown(turn.Response)})
	}
	m.refreshConvContent()
	m.convView.GotoBottom()

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
	m.conversations = nil
	for _, turn := range turns {
		m.conversations = append(m.conversations, conversationEntry{Input: turn.Input, Response: stripMarkdown(turn.Response)})
	}
	m.refreshConvContent()
	m.convView.GotoBottom()

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
		m.conversations = nil
		for _, turn := range turns {
			m.conversations = append(m.conversations, conversationEntry{Input: turn.Input, Response: stripMarkdown(turn.Response)})
		}
		m.refreshConvContent()
		m.convView.GotoBottom()

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
	if m.recentSelIdx >= 0 && m.recentSelIdx < len(all) {
		m.handleSwitchTopic(all[m.recentSelIdx].ID)
	}
}

func (m *Model) selectCurrentTopic() {
	if len(m.topicFiltered) == 0 {
		return
	}
	idx := m.topicSelIdx
	if idx < 0 || idx >= len(m.topicFiltered) {
		return
	}
	m.toggleTopicPopup()
	m.handleSwitchTopic(m.topicFiltered[idx].ID)
}

func (m *Model) toggleLogPopup() {
	m.logPopupVisible = !m.logPopupVisible
	if m.logPopupVisible {
		m.focused = focusLogs
		m.textInput.Blur()
		m.logView.GotoBottom()
	} else {
		m.focused = focusInput
		m.textInput.Focus()
	}
}

// ─── Background Loops ───

func (m *Model) consumerLoop() {
	for {
		time.Sleep(200 * time.Millisecond)
		m.processTasks()
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

// ─── Task Processing ───

func (m *Model) processTasks() {
	t := m.taskMgr.NextDue()
	if t == nil {
		return
	}
	if t.RetryCount >= 3 {
		m.logger.Error("CONSUMER", fmt.Sprintf("Task %s exceeded max retries", t.ID))
		m.taskMgr.Update(t.ID, func(upd *task.Task) {
			upd.Status = task.StatusFailed
		})
		return
	}

	summaryText := m.topicMgr.FormatForPrompt()

	todos := m.todoMgr.FormatForPrompt()
	if !strings.Contains(todos, "No todos") && !strings.Contains(todos, "No pending") {
		summaryText += "\n\nPending todos:\n" + todos
	}

	m.logger.Info("CONSUMER", fmt.Sprintf("Classifying: %s", t.RawInput))

	intentResult, err := m.router.Route(t.RawInput, summaryText)
	if err != nil {
		m.logger.Error("CONSUMER", fmt.Sprintf("Classify failed: %s", err))
		m.taskMgr.Update(t.ID, func(upd *task.Task) {
			upd.RetryCount++
			upd.Status = task.StatusPending
		})
		return
	}

	if intentResult.Confidence < 0.8 || !strings.HasPrefix(intentResult.Event, "EXECUTE_") {
		m.logger.Info("CONSUMER", fmt.Sprintf("Confidence %.2f < 0.8 — falling back to conversation", intentResult.Confidence))
		m.handleConversation(t, summaryText, intentResult)
	} else {
		m.handlePipeline(t, summaryText, intentResult)
	}

	m.lastInput = t.RawInput
	m.lastIntent = intentResult
}

func (m *Model) handleConversation(t *task.Task, summaryText string, intentResult *intent.ParsedIntent) {
	m.logger.Info("CONSUMER", fmt.Sprintf("Intent: %s — chatting", intentResult.Event))

	terms := topic.ExtractSearchTerms(intentResult.Params)
	searchResults := ""
	if len(terms) > 0 {
		sr, _ := topic.SearchPastData(terms)
		searchResults = sr
	}

	response, err := m.router.Chat(t.RawInput, summaryText, searchResults)
	if err != nil {
		m.logger.Error("CONSUMER", fmt.Sprintf("Chat failed: %s", err))
		m.taskMgr.Update(t.ID, func(upd *task.Task) {
			upd.Status = task.StatusFailed
		})
		return
	}

	response = cleanResponse(response, t.RawInput)

	m.logger.Info("DHI", response)

	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		m.topicMgr.AppendTurn(cur.ID, t.RawInput, response)
	}

	m.events <- convMsg{input: t.RawInput, response: response}

	if m.feedback != nil {
		m.feedback.RecordConversation(t.RawInput)
	}

	m.taskMgr.Update(t.ID, func(upd *task.Task) {
		upd.Status = task.StatusCompleted
		upd.Result = response
	})
}

func (m *Model) handlePipeline(t *task.Task, summaryText string, intentResult *intent.ParsedIntent) {
	m.logger.Info("CONSUMER", fmt.Sprintf("Intent: %s — executing", intentResult.Event))

	pipelineName := strings.ToLower(strings.TrimPrefix(intentResult.Event, "EXECUTE_"))

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

	raw, synth := m.executor.FindAndExecute(intentResult.Event, params, t.RawInput, summaryText)

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

	cur := m.topicMgr.CurrentTopic()
	if cur != nil {
		m.topicMgr.AppendTurn(cur.ID, t.RawInput, reply)
	}

	m.events <- convMsg{input: t.RawInput, response: reply}
	m.events <- pipelineStatusMsg{name: pipelineName, status: "IDLE"}
	m.logger.Info("EXECUTOR", fmt.Sprintf("Pipeline %s completed", intentResult.Event))

	success := raw != "" && !strings.Contains(raw, "failed") && !strings.Contains(raw, "error")
	errMsg := ""
	if !success {
		errMsg = raw
	}
	if m.feedback != nil {
		m.feedback.RecordOutcome(t.RawInput, intentResult.Event, "EXECUTE_"+pipelineName, intentResult.Confidence, success, false, errMsg)
	}

	m.taskMgr.Update(t.ID, func(upd *task.Task) {
		upd.Status = task.StatusCompleted
		upd.Result = reply
		if upd.Type == task.TypeScheduled && upd.IntervalSec > 0 {
			nextTime := time.Now().Add(time.Duration(upd.IntervalSec) * time.Second)
			upd.ScheduledFor = &nextTime
			upd.Status = task.StatusWaiting
		}
	})
}

// ─── Slash Completion ───

var slashCommands = []string{
	"/good",
	"/bad",
	"/log",
	"/logs",
	"/quit",
	"/new",
	"/fork",
	"/resume",
	"/topic",
	"/topics",
}

func (m *Model) computeSlashMatches(prefix string) {
	m.slashMatches = nil
	m.slashIndex = 0
	if !strings.HasPrefix(prefix, "/") || prefix == "/" {
		return
	}
	if strings.HasPrefix(prefix, "/topic ") || prefix == "/topic" {
		m.computeTopicMatches(prefix)
		return
	}
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd, prefix) && cmd != prefix {
			m.slashMatches = append(m.slashMatches, cmd)
		}
	}
}

func (m *Model) computeTopicMatches(prefix string) {
	topics := m.topicMgr.ListTopics()
	for i := range topics {
		label := fmt.Sprintf("/topic %d", i+1)
		if strings.HasPrefix(label, prefix) {
			m.slashMatches = append(m.slashMatches, label)
		}
	}
}
