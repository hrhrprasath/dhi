package tui

type tickMsg struct{}
type routerStatusMsg struct{ online bool }
type logMsg struct{ message string }
type convMsg struct {
	input    string
	response string
}
type convTokenMsg struct {
	input string
	token string
}
type convCompleteMsg struct {
	input    string
	response string
}
type pipelineStatusMsg struct {
	name   string
	status string
}
type refreshMsg struct{}
type rssUpdateMsg struct{}
type rssAddMsg struct {
	url string
	err error
}
type rssFetchTopicMsg struct {
	url        string
	sourceName string
	input      string
	summary    string
	err        error
}
