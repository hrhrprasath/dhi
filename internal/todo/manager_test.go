package todo

import (
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestAdd(t *testing.T) {
	m := NewManager(t.TempDir())

	id := m.Add("buy milk")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	items := m.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Text != "buy milk" {
		t.Fatalf("expected 'buy milk', got %q", items[0].Text)
	}
	if items[0].Done {
		t.Fatal("expected new item to be not done")
	}
}

func TestAddMultiple(t *testing.T) {
	m := NewManager(t.TempDir())

	m.Add("task one")
	m.Add("task two")
	m.Add("task three")

	items := m.ListItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestDone(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.Add("finish homework")

	ok := m.Complete(id)
	if !ok {
		t.Fatal("expected Complete to return true")
	}

	items := m.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].Done {
		t.Fatal("expected item to be marked done")
	}
}

func TestDone_WithPrefixID(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.Add("buy groceries")

	ok := m.Complete(id[:8])
	if !ok {
		t.Fatal("expected Complete with prefix to return true")
	}

	items := m.ListItems()
	if !items[0].Done {
		t.Fatal("expected item to be marked done via prefix match")
	}
}

func TestDone_ReturnsFalseForMissingID(t *testing.T) {
	m := NewManager(t.TempDir())
	m.Add("some task")

	ok := m.Complete("nonexistent")
	if ok {
		t.Fatal("expected Complete to return false for missing ID")
	}
}

func TestUncomplete(t *testing.T) {
	t.Run("exact id", func(t *testing.T) {
		m2 := NewManager(t.TempDir())
		id := m2.Add("task")
		m2.Complete(id)
		ok := m2.Uncomplete(id)
		if !ok {
			t.Fatal("expected Uncomplete to return true")
		}
		items := m2.ListItems()
		if items[0].Done {
			t.Fatal("expected item to be unmarked")
		}
	})

	t.Run("prefix id", func(t *testing.T) {
		m2 := NewManager(t.TempDir())
		id := m2.Add("task")
		m2.Complete(id)
		ok := m2.Uncomplete(id[:8])
		if !ok {
			t.Fatal("expected Uncomplete with prefix to return true")
		}
		items := m2.ListItems()
		if items[0].Done {
			t.Fatal("expected item to be unmarked via prefix")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		m2 := NewManager(t.TempDir())
		ok := m2.Uncomplete("nonexistent")
		if ok {
			t.Fatal("expected Uncomplete to return false for missing ID")
		}
	})
}

func TestList_WithoutDone(t *testing.T) {
	m := NewManager(t.TempDir())

	m.Add("active task")
	id := m.Add("done task")
	m.Complete(id)

	result := m.List(false)
	if strings.Contains(result, "done task") {
		t.Fatal("expected done task to be excluded from List(false)")
	}
	if !strings.Contains(result, "active task") {
		t.Fatal("expected active task to be included in List(false)")
	}
}

func TestList_WithDone(t *testing.T) {
	m := NewManager(t.TempDir())

	m.Add("active task")
	id := m.Add("done task")
	m.Complete(id)

	result := m.List(true)
	if !strings.Contains(result, "done task") {
		t.Fatal("expected done task to be included in List(true)")
	}
	if !strings.Contains(result, "active task") {
		t.Fatal("expected active task to be included in List(true)")
	}
}

func TestList_Empty(t *testing.T) {
	m := NewManager(t.TempDir())

	result := m.List(false)
	if result != "No todos." {
		t.Fatalf("expected 'No todos.', got %q", result)
	}
}

func TestList_AllDone(t *testing.T) {
	m := NewManager(t.TempDir())
	id := m.Add("only task")
	m.Complete(id)

	result := m.List(false)
	if result != "No pending todos." {
		t.Fatalf("expected 'No pending todos.', got %q", result)
	}
}

func TestActiveCount(t *testing.T) {
	m := NewManager(t.TempDir())

	if n := m.ActiveCount(); n != 0 {
		t.Fatalf("expected 0 active, got %d", n)
	}

	m.Add("task 1")
	m.Add("task 2")
	id := m.Add("task 3")
	m.Complete(id)

	if n := m.ActiveCount(); n != 2 {
		t.Fatalf("expected 2 active, got %d", n)
	}
}

func TestDelete(t *testing.T) {
	m := NewManager(t.TempDir())

	id := m.Add("delete me")
	ok := m.Delete(id)
	if !ok {
		t.Fatal("expected Delete to return true")
	}

	items := m.ListItems()
	if len(items) != 0 {
		t.Fatalf("expected 0 items after delete, got %d", len(items))
	}
}

func TestDelete_ReturnsFalseForMissingID(t *testing.T) {
	m := NewManager(t.TempDir())
	m.Add("keep me")

	ok := m.Delete("nonexistent")
	if ok {
		t.Fatal("expected Delete to return false for missing ID")
	}

	items := m.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestDelete_OnlyExactMatch(t *testing.T) {
	m := NewManager(t.TempDir())

	id1 := m.Add("task one")
	m.Add("task two")

	ok := m.Delete(id1)
	if !ok {
		t.Fatal("expected Delete to return true")
	}

	items := m.ListItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Text != "task two" {
		t.Fatalf("expected 'task two', got %q", items[0].Text)
	}
}

func TestPersistenceAcrossOperations(t *testing.T) {
	dir := t.TempDir()

	m1 := NewManager(dir)
	id1 := m1.Add("first todo")
	m1.Add("second todo")
	m1.Complete(id1)

	// Create new manager loading from the same directory
	m2 := NewManager(dir)

	if n := m2.ActiveCount(); n != 1 {
		t.Fatalf("expected 1 active item, got %d", n)
	}

	items := m2.ListItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 items total, got %d", len(items))
	}

	// Complete the remaining item via m2
	remainingID := items[0].ID
	if items[0].Done {
		remainingID = items[1].ID
	}
	m2.Complete(remainingID)

	// Verify third load
	m3 := NewManager(dir)
	if n := m3.ActiveCount(); n != 0 {
		t.Fatalf("expected 0 active items, got %d", n)
	}
}

func TestFormatForPrompt(t *testing.T) {
	m := NewManager(t.TempDir())
	m.Add("active item")

	result := m.FormatForPrompt()
	if !strings.Contains(result, "active item") {
		t.Fatalf("expected 'active item' in prompt, got %q", result)
	}
}

func TestCompleteThenDelete(t *testing.T) {
	m := NewManager(t.TempDir())

	id := m.Add("temp task")
	m.Complete(id)
	m.Delete(id)

	items := m.ListItems()
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}
