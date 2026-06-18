package task

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewManager_UsesDefaultParserWhenNil(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	_, ok := m.Parser().(*DefaultParser)
	if !ok {
		t.Fatal("expected DefaultParser when nil provided")
	}
}

func TestAddOnce(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	task := m.AddOnce("hello world")
	if task == nil {
		t.Fatal("expected non-nil task")
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.RawInput != "hello world" {
		t.Fatalf("expected 'hello world', got %q", task.RawInput)
	}
	if task.Status != StatusPending {
		t.Fatalf("expected StatusPending, got %v", task.Status)
	}
	if task.Type != TypeOnce {
		t.Fatalf("expected TypeOnce, got %v", task.Type)
	}
	if task.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestAddScheduled(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	future := time.Now().Add(1 * time.Hour)

	task := m.AddScheduled("scheduled task", future, 3600)
	if task == nil {
		t.Fatal("expected non-nil task")
	}
	if task.Status != StatusWaiting {
		t.Fatalf("expected StatusWaiting, got %v", task.Status)
	}
	if task.Type != TypeScheduled {
		t.Fatalf("expected TypeScheduled, got %v", task.Type)
	}
	if task.ScheduledFor == nil {
		t.Fatal("expected non-nil ScheduledFor")
	}
	if !task.ScheduledFor.Equal(future) {
		t.Fatalf("expected ScheduledFor %v, got %v", future, task.ScheduledFor)
	}
	if task.IntervalSec != 3600 {
		t.Fatalf("expected IntervalSec 3600, got %d", task.IntervalSec)
	}
}

func TestNextDue_ReturnsPendingTask(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	t1 := m.AddOnce("first")
	m.AddOnce("second")

	due := m.NextDue()
	if due == nil {
		t.Fatal("expected a task, got nil")
	}
	if due.ID != t1.ID {
		t.Fatalf("expected first added task, got %s", due.ID)
	}
	if due.Status != StatusProcessing {
		t.Fatalf("expected StatusProcessing, got %v", due.Status)
	}

	// Verify the in-memory status is also updated
	tasks := m.ListByStatus(StatusProcessing)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 processing task, got %d", len(tasks))
	}
}

func TestNextDue_ReturnsNilWhenNoPending(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	due := m.NextDue()
	if due != nil {
		t.Fatalf("expected nil, got %v", due)
	}
}

func TestNextDue_ActivatesDueWaitingTask(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	past := time.Now().Add(-1 * time.Hour)

	m.AddScheduled("overdue", past, 0)

	due := m.NextDue()
	if due == nil {
		t.Fatal("expected due waiting task to be returned")
	}
	if due.RawInput != "overdue" {
		t.Fatalf("expected 'overdue', got %q", due.RawInput)
	}
	if due.Status != StatusProcessing {
		t.Fatalf("expected StatusProcessing, got %v", due.Status)
	}
}

func TestNextDue_DoesNotReturnFutureWaitingTask(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	future := time.Now().Add(1 * time.Hour)

	m.AddScheduled("future", future, 0)

	due := m.NextDue()
	if due != nil {
		t.Fatalf("expected nil for future waiting task, got %v", due)
	}
}

func TestUpdate(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	task := m.AddOnce("update me")

	updated := m.Update(task.ID, func(t *Task) {
		t.Status = StatusCompleted
		t.Result = "done"
		t.RetryCount = 1
	})
	if updated == nil {
		t.Fatal("expected non-nil updated task")
	}
	if updated.Status != StatusCompleted {
		t.Fatalf("expected StatusCompleted, got %v", updated.Status)
	}
	if updated.Result != "done" {
		t.Fatalf("expected result 'done', got %q", updated.Result)
	}
	if updated.RetryCount != 1 {
		t.Fatalf("expected RetryCount 1, got %d", updated.RetryCount)
	}

	// Verify persistence
	tasks := m.ListByStatus(StatusCompleted)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(tasks))
	}
}

func TestUpdate_ReturnsNilForMissingID(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	updated := m.Update("nonexistent", func(t *Task) {
		t.Status = StatusCompleted
	})
	if updated != nil {
		t.Fatal("expected nil for nonexistent ID")
	}
}

func TestRetryCountIncrementViaUpdate(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	task := m.AddOnce("retry me")

	m.Update(task.ID, func(t *Task) {
		t.Status = StatusFailed
		t.RetryCount = 1
	})
	m.Update(task.ID, func(t *Task) {
		t.RetryCount = 2
	})

	tasks := m.ListByStatus(StatusFailed)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 failed task, got %d", len(tasks))
	}
	if tasks[0].RetryCount != 2 {
		t.Fatalf("expected RetryCount 2, got %d", tasks[0].RetryCount)
	}
}

func TestListByStatus(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	m.AddOnce("task1")
	m.AddOnce("task2")
	m.AddOnce("task3")

	tasks := m.ListByStatus(StatusPending)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 pending tasks, got %d", len(tasks))
	}

	// List with no filter returns all
	all := m.ListByStatus()
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks with no filter, got %d", len(all))
	}

	// List with no match returns empty
	completed := m.ListByStatus(StatusCompleted)
	if len(completed) != 0 {
		t.Fatalf("expected 0 completed, got %d", len(completed))
	}
}

func TestListByStatus_MultipleStatuses(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	m.AddOnce("pending")
	future := time.Now().Add(1 * time.Hour)
	m.AddScheduled("waiting", future, 0)

	tasks := m.ListByStatus(StatusPending, StatusWaiting)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks (pending+waiting), got %d", len(tasks))
	}
}

func TestCountByStatus(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	m.AddOnce("task1")
	future := time.Now().Add(1 * time.Hour)
	m.AddScheduled("task2", future, 0)

	if n := m.CountByStatus(StatusPending); n != 1 {
		t.Fatalf("expected 1 pending, got %d", n)
	}
	if n := m.CountByStatus(StatusWaiting); n != 1 {
		t.Fatalf("expected 1 waiting, got %d", n)
	}
	if n := m.CountByStatus(StatusCompleted); n != 0 {
		t.Fatalf("expected 0 completed, got %d", n)
	}
}

func TestCheckSchedule_ActivatesDueTasks(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	past := time.Now().Add(-30 * time.Minute)
	future := time.Now().Add(1 * time.Hour)

	m.AddScheduled("overdue", past, 0)
	m.AddScheduled("future", future, 0)

	activated := m.CheckSchedule()
	if activated != 1 {
		t.Fatalf("expected 1 activated, got %d", activated)
	}

	// Verify overdue is now pending
	tasks := m.ListByStatus(StatusPending)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 pending task after check, got %d", len(tasks))
	}
	if tasks[0].RawInput != "overdue" {
		t.Fatalf("expected 'overdue', got %q", tasks[0].RawInput)
	}
}

func TestCheckSchedule_ActivatesMultipleDueTasks(t *testing.T) {
	m := NewManager(t.TempDir(), nil)

	past1 := time.Now().Add(-2 * time.Hour)
	past2 := time.Now().Add(-1 * time.Hour)

	m.AddScheduled("overdue1", past1, 0)
	m.AddScheduled("overdue2", past2, 0)

	activated := m.CheckSchedule()
	if activated != 2 {
		t.Fatalf("expected 2 activated, got %d", activated)
	}

	pending := m.ListByStatus(StatusPending)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending tasks, got %d", len(pending))
	}
}

func TestCheckSchedule_DoesNotActivateFutureTasks(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	future := time.Now().Add(1 * time.Hour)

	m.AddScheduled("future", future, 0)

	activated := m.CheckSchedule()
	if activated != 0 {
		t.Fatalf("expected 0 activated, got %d", activated)
	}

	pending := m.ListByStatus(StatusPending)
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(pending))
	}
}

func TestCheckSchedule_DoesNotActivateNonWaitingTasks(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	past := time.Now().Add(-1 * time.Hour)

	m.AddScheduled("overdue", past, 0)

	// Mark it as processing
	m.NextDue()

	// CheckSchedule should not activate it again
	activated := m.CheckSchedule()
	if activated != 0 {
		t.Fatalf("expected 0 activated (already processing), got %d", activated)
	}
}

func TestDeleteByPrefix(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	task := m.AddOnce("delete me")

	deleted := m.DeleteByPrefix(task.ID[:8])
	if !deleted {
		t.Fatal("expected DeleteByPrefix to return true")
	}

	tasks := m.ListByStatus()
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks after deletion, got %d", len(tasks))
	}
}

func TestDeleteByPrefix_ReturnsFalseOnNoMatch(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	m.AddOnce("keep me")

	deleted := m.DeleteByPrefix("nonexistent")
	if deleted {
		t.Fatal("expected DeleteByPrefix to return false")
	}

	tasks := m.ListByStatus()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestTaskCRUDPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create and add tasks
	m1 := NewManager(dir, nil)
	t1 := m1.AddOnce("persist me")
	m1.AddScheduled("scheduled task", time.Now().Add(1*time.Hour), 0)

	m1.Update(t1.ID, func(t *Task) {
		t.Status = StatusCompleted
		t.Result = "persisted"
	})

	// Load from same directory
	m2 := NewManager(dir, nil)

	tasks := m2.ListByStatus()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks loaded from disk, got %d", len(tasks))
	}

	completed := m2.ListByStatus(StatusCompleted)
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(completed))
	}
	if completed[0].Result != "persisted" {
		t.Fatalf("expected result 'persisted', got %q", completed[0].Result)
	}

	waiting := m2.ListByStatus(StatusWaiting)
	if len(waiting) != 1 {
		t.Fatalf("expected 1 waiting task, got %d", len(waiting))
	}
}

func TestAddScheduledWithFutureTimeCreatesWaitingTask(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	future := time.Now().Add(24 * time.Hour)

	task := m.AddScheduled("future task", future, 86400)

	if task.Status != StatusWaiting {
		t.Fatalf("expected StatusWaiting, got %v", task.Status)
	}
	if task.Type != TypeScheduled {
		t.Fatalf("expected TypeScheduled, got %v", task.Type)
	}
	if task.IntervalSec != 86400 {
		t.Fatalf("expected IntervalSec 86400, got %d", task.IntervalSec)
	}

	// Should not be returned by NextDue since it's in the future
	due := m.NextDue()
	if due != nil {
		t.Fatalf("expected nil from NextDue for future scheduled task")
	}
}

func TestPersistInternalTaskFields(t *testing.T) {
	dir := t.TempDir()
	m1 := NewManager(dir, nil)

	past := time.Now().Add(-1 * time.Hour)
	task := m1.AddScheduled("recurring", past, 300)
	m1.Update(task.ID, func(t *Task) {
		t.RetryCount = 3
	})

	// Load fresh
	m2 := NewManager(dir, nil)
	tasks := m2.ListByStatus(StatusWaiting, StatusPending)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].RetryCount != 3 {
		t.Fatalf("expected RetryCount 3, got %d", tasks[0].RetryCount)
	}
	if tasks[0].IntervalSec != 300 {
		t.Fatalf("expected IntervalSec 300, got %d", tasks[0].IntervalSec)
	}
	if tasks[0].ScheduledFor == nil {
		t.Fatal("expected ScheduledFor to be persisted")
	}
}

func TestMultipleUpdates(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	task := m.AddOnce("multi-update")

	states := []Status{StatusProcessing, StatusCompleted}
	for _, s := range states {
		m.Update(task.ID, func(t *Task) {
			t.Status = s
		})
	}

	tasks := m.ListByStatus(StatusCompleted)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 completed task, got %d", len(tasks))
	}
}
