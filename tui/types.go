package tui

import (
	"dhi_harness/internal/config"
	"dhi_harness/internal/conversation"
	"dhi_harness/internal/intent"
	"dhi_harness/internal/pipeline"
	"dhi_harness/internal/queue"
	"dhi_harness/internal/services"
	"dhi_harness/internal/todo"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/bubbletea/v2"
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

type pipelineEntry struct {
	Name   string
	Status string
}

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
