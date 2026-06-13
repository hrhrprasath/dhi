package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"dhi_harness/internal/task"
	"dhi_harness/internal/todo"
)

type Handlers struct {
	todoMgr *todo.Manager
	taskMgr *task.Manager
}

func NewHandlers(todoMgr *todo.Manager, taskMgr *task.Manager) *Handlers {
	return &Handlers{todoMgr: todoMgr, taskMgr: taskMgr}
}

func (h *Handlers) Execute(handler string, params map[string]string) string {
	switch handler {
	case "todo":
		return h.handleTodo(params["action"], params["text"])
	case "schedule":
		return h.handleSchedule(params["action"], params["when"], params["command"], params["schedule_id"], params["schedules"])
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

func (h *Handlers) handleSchedule(action, when, command, scheduleID, schedulesJSON string) string {
	switch action {
	case "add":
		if schedulesJSON != "" {
			var entries []task.ScheduleEntry
			if err := json.Unmarshal([]byte(schedulesJSON), &entries); err != nil {
				return "Invalid schedules JSON: " + err.Error()
			}
			created := h.taskMgr.AddScheduledFromEntries(entries)
			return fmt.Sprintf("✅ Created %d schedule(s)", len(created))
		}
		if when == "" || command == "" {
			return "Schedule add requires 'when' and 'command' params, or 'schedules' JSON array"
		}
		sched, err := h.taskMgr.Parser().Parse(when)
		if err != nil {
			return "Could not parse time: " + err.Error() + ". Try: 'in 5 minutes', 'daily at 9am', 'tomorrow at 5pm'"
		}
		h.taskMgr.AddScheduled(command, sched.ScheduledFor, sched.IntervalSec)
		return fmt.Sprintf("✅ Scheduled: \"%s\" at %s", command, task.FormatTaskTime(sched.ScheduledFor))

	case "list":
		waiting := h.taskMgr.ListByStatus(task.StatusWaiting)
		if len(waiting) == 0 {
			return "No schedules."
		}
		var sb strings.Builder
		for _, t := range waiting {
			due := task.FormatTaskTime(*t.ScheduledFor)
			recur := ""
			if t.IntervalSec > 0 {
				recur = " (recurring)"
			}
			sb.WriteString(fmt.Sprintf("  %s: \"%s\" → %s%s\n", t.ID[:8], t.RawInput, due, recur))
		}
		return sb.String()

	case "delete":
		if scheduleID == "" {
			return "Schedule delete requires 'schedule_id'"
		}
		if h.taskMgr.Delete(scheduleID) {
			return "✅ Schedule deleted"
		}
		if h.taskMgr.DeleteByPrefix(scheduleID) {
			return "✅ Schedule deleted"
		}
		return "Schedule not found: " + scheduleID

	case "update":
		if scheduleID == "" {
			return "Schedule update requires 'schedule_id'"
		}
		updated := h.taskMgr.Update(scheduleID, func(t *task.Task) {
			if command != "" {
				t.RawInput = command
			}
			if when != "" {
				sched, err := h.taskMgr.Parser().Parse(when)
				if err == nil {
					t.ScheduledFor = &sched.ScheduledFor
					if sched.IntervalSec > 0 {
						t.IntervalSec = sched.IntervalSec
					}
				}
			}
		})
		if updated != nil {
			return "✅ Schedule updated"
		}
		return "Schedule not found: " + scheduleID

	default:
		return "Schedule action must be: add, list, delete, update"
	}
}


