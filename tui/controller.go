package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dhi_harness/internal/intent"
	"dhi_harness/internal/services"
	"dhi_harness/internal/task"
	"dhi_harness/internal/topic"
)

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

	topicTitle := ""
	if cur := m.topicMgr.CurrentTopic(); cur != nil {
		topicTitle = cur.Title
	}
	intentResult, err := m.router.Route(t.RawInput, summaryText, topicTitle)
	if err != nil {
		m.logger.Error("CONSUMER", fmt.Sprintf("Classify failed: %s", err))
		m.taskMgr.Update(t.ID, func(upd *task.Task) {
			upd.RetryCount++
			upd.Status = task.StatusPending
		})
		return
	}

	if cur := m.topicMgr.CurrentTopic(); cur != nil && strings.HasPrefix(cur.Title, "Topic ") {
		if title, err := m.router.GenerateTitle(t.RawInput); err == nil {
			title = cleanTitle(title)
			if title != "" {
				m.topicMgr.UpdateTitle(cur.ID, title)
				m.events <- logMsg{message: fmt.Sprintf("[TOPIC][INFO] Auto-titled: %s", title)}
				m.events <- refreshMsg{}
			}
		}
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

	topicTitle := ""
	if cur := m.topicMgr.CurrentTopic(); cur != nil {
		topicTitle = cur.Title
	}
	response, err := m.router.Chat(t.RawInput, summaryText, searchResults, topicTitle)
	if err != nil {
		m.logger.Error("CONSUMER", fmt.Sprintf("Chat failed: %s", err))
		m.taskMgr.Update(t.ID, func(upd *task.Task) {
			upd.Status = task.StatusFailed
		})
		return
	}

	response = cleanResponse(response, t.RawInput)
	m.logger.Info("DHI", response)

	if cur := m.topicMgr.CurrentTopic(); cur != nil {
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
		var raw map[string]any
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

	if pipelineName == "search_inline" || pipelineName == "browse" {
		m.lastSearchURLs = extractURLs(raw + "\n" + synth)
		urls := m.lastSearchURLs
		if len(urls) > 0 {
			reply += fmtURLs(urls)
			m.logger.Info("SEARCH", fmt.Sprintf("Extracted %d URLs", len(urls)))
		}
	}

	if raw != "" {
		m.logger.Info("EXECUTOR", fmt.Sprintf("Raw output:\n%s", raw))
	}
	m.logger.Info("DHI", reply)

	if cur := m.topicMgr.CurrentTopic(); cur != nil {
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
