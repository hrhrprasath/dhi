package pipeline

import (
	"fmt"
	"os/exec"
	"strings"

	"dhi_harness/internal/todo"
)

type Handlers struct {
	todoMgr *todo.Manager
}

func NewHandlers(todoMgr *todo.Manager) *Handlers {
	return &Handlers{todoMgr: todoMgr}
}

func (h *Handlers) Execute(handler string, params map[string]string) string {
	switch handler {
	case "system_metrics":
		return h.systemMetrics()
	case "todo":
		return h.handleTodo(params["action"], params["text"])
	default:
		return fmt.Sprintf("Unknown handler: %s", handler)
	}
}

func (h *Handlers) handleTodo(action, text string) string {
	switch action {
	case "add":
		if text == "" {
			return "Missing text for todo. Try: add buy milk"
		}
		id := h.todoMgr.Add(text)
		return "✅ Todo created: " + text + " (id: " + id + ")"
	case "done", "complete":
		if text == "" {
			return "Missing todo id to mark done. Try: done 123456"
		}
		if h.todoMgr.Complete(text) {
			return "✅ Marked todo as done: " + text
		}
		return "Todo not found: " + text
	default:
		return h.todoMgr.List(false)
	}
}

func (h *Handlers) systemMetrics() string {
	kernel, _ := exec.Command("uname", "-r").Output()
	hostname, _ := exec.Command("uname", "-n").Output()
	uptime, _ := exec.Command("uptime").Output()

	return fmt.Sprintf(`Kernel: %s
Hostname: %s
Uptime: %s`,
		strings.TrimSpace(string(kernel)),
		strings.TrimSpace(string(hostname)),
		strings.TrimSpace(string(uptime)))
}
