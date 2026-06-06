package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dhi_harness/internal/conversation"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/services"
	"dhi_harness/internal/task"

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
		if m.logPopupVisible {
			m.logPopupVisible = false
			if m.focused == focusLogs {
				m.focused = focusInput
				return m.textInput.Focus()
			}
		}
		return nil

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
			if m.logPopupVisible {
				m.focused = focusLogs
				m.textInput.Blur()
				m.logView.GotoBottom()
				return nil
			}
			m.focused = focusInput
			return m.textInput.Focus()
		}

	case msg.Code == tea.KeyEnter:
		if m.focused == focusInput {
			input := strings.TrimSpace(m.textInput.Value())
			m.textInput.SetValue("")
			if input == "quit" || input == "exit" || input == ":q" || input == "/quit" || input == "/q" {
				return tea.Quit
			}
			if input == "/log" || input == "/logs" || input == "/l" {
				m.toggleLogPopup()
				return nil
			}
			m.logger.Info("INPUT", fmt.Sprintf("User: %s", input))
			if input != "" && m.routerOnline {
				m.taskMgr.AddOnce(input)
				m.pendingTasks = m.taskMgr.CountByStatus(task.StatusPending)
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

func (m *Model) fireLoop() {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		m.events <- fireTickMsg{}
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

	summary := m.convMgr.LoadSummary()
	summaryText := m.convMgr.FormatForPrompt(&summary)

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
}

func (m *Model) handleConversation(t *task.Task, summaryText string, intentResult *intent.ParsedIntent) {
	m.logger.Info("CONSUMER", fmt.Sprintf("Intent: %s — chatting", intentResult.Event))

	terms := conversation.ExtractSearchTerms(intentResult.Params)
	searchResults := ""
	if len(terms) > 0 {
		sr, _ := conversation.SearchPastData(terms)
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
	m.convMgr.UpdateSummary(t.RawInput, response)
	m.events <- convMsg{input: t.RawInput, response: response}

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
	m.convMgr.UpdateSummary(t.RawInput, reply)
	m.events <- convMsg{input: t.RawInput, response: reply}
	m.events <- pipelineStatusMsg{name: pipelineName, status: "IDLE"}
	m.logger.Info("EXECUTOR", fmt.Sprintf("Pipeline %s completed", intentResult.Event))

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
