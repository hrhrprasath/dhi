package tui

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
