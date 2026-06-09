package tui

import (
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
	"charm.land/bubbletea/v2"
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

type conversationEntry struct {
	Input    string
	Response string
	thinking bool
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
	cfg      *config.Config
	logger   *services.Logger
	taskMgr  *task.Manager
	router   *intent.Router
	executor *pipeline.Executor
	topicMgr *topic.Manager
	sysSvc   *services.SystemService
	todoMgr  *todo.Manager
	feedback FeedbackRecorder

	textInput    textinput.Model
	logView      viewport.Model
	convView     viewport.Model
	pipelineView viewport.Model
	topicView    viewport.Model
	cpuProgress  progress.Model
	ramProgress  progress.Model
	gpuProgress  progress.Model

	conversations  []conversationEntry
	logEntries     []string
	routerOnline   bool
	routerStatus   string
	pipelines      []pipelineEntry
	focused        focusPanel
	logPopupVisible bool

	topicPopupVisible bool
	topicFilter       string
	topicFiltered     []topic.Topic
	topicSelIdx       int
	recentSelIdx      int

	hostname  string
	kernel    string
	uptimeSec uint64

	metrics services.SystemMetrics

	events   chan interface{}
	focusCmd tea.Cmd
	width    int
	height   int

	lastInput     string
	lastIntent    *intent.ParsedIntent
	lastSubmitted string

	slashMatches []string
	slashIndex   int

	thinking         int
	thinkingSpinner spinner.Model
}
