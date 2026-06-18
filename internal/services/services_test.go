package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	l := NewLogger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestInfoLog(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)
	defer l.Unsubscribe(sub)

	l.Info("test", "info message")

	select {
	case entry := <-sub.Ch():
		if entry.Level != InfoLevel {
			t.Fatalf("expected InfoLevel, got %v", entry.Level)
		}
		if entry.Component != "test" {
			t.Fatalf("expected component 'test', got %q", entry.Component)
		}
		if entry.Message != "info message" {
			t.Fatalf("expected message 'info message', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestErrorLog(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)
	defer l.Unsubscribe(sub)

	l.Error("test", "error message")

	select {
	case entry := <-sub.Ch():
		if entry.Level != ErrorLevel {
			t.Fatalf("expected ErrorLevel, got %v", entry.Level)
		}
		if entry.Message != "error message" {
			t.Fatalf("expected message 'error message', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestWarnLog(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)
	defer l.Unsubscribe(sub)

	l.Warn("test", "warn message")

	select {
	case entry := <-sub.Ch():
		if entry.Level != WarnLevel {
			t.Fatalf("expected WarnLevel, got %v", entry.Level)
		}
		if entry.Message != "warn message" {
			t.Fatalf("expected message 'warn message', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestDebugLog(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)
	defer l.Unsubscribe(sub)

	l.Debug("test", "debug message")

	select {
	case entry := <-sub.Ch():
		if entry.Level != DebugLevel {
			t.Fatalf("expected DebugLevel, got %v", entry.Level)
		}
		if entry.Message != "debug message" {
			t.Fatalf("expected message 'debug message', got %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestSubscribeReceivesMessages(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)
	defer l.Unsubscribe(sub)

	l.Info("comp", "msg1")
	l.Warn("comp", "msg2")

	entry1 := <-sub.Ch()
	if entry1.Message != "msg1" {
		t.Fatalf("expected 'msg1', got %q", entry1.Message)
	}

	entry2 := <-sub.Ch()
	if entry2.Message != "msg2" {
		t.Fatalf("expected 'msg2', got %q", entry2.Message)
	}
}

func TestRecentReturnsEntries(t *testing.T) {
	l := NewLogger()

	l.Info("comp", "first")
	l.Warn("comp", "second")

	recent := l.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(recent))
	}
	if recent[0].Message != "first" {
		t.Fatalf("expected 'first', got %q", recent[0].Message)
	}
	if recent[1].Message != "second" {
		t.Fatalf("expected 'second', got %q", recent[1].Message)
	}
}

func TestRecentRespectsLimit(t *testing.T) {
	l := NewLogger()

	for i := 0; i < 10; i++ {
		l.Info("comp", "msg")
	}

	recent := l.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent entries, got %d", len(recent))
	}
}

func TestRecentReturnsCopy(t *testing.T) {
	l := NewLogger()

	l.Info("comp", "before")

	recent := l.Recent(10)
	if len(recent) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(recent))
	}

	l.Info("comp", "after")

	if len(recent) != 1 {
		t.Fatal("Recent should return a copy, but the slice grew")
	}
}

func TestUnsubscribeRemovesSubscriber(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)

	l.Info("comp", "before")

	<-sub.Ch()

	l.Unsubscribe(sub)

	l.Info("comp", "after")

	select {
	case _, ok := <-sub.Ch():
		if ok {
			t.Fatal("expected channel to be closed after unsubscribe")
		}
	default:
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestNewLogEntryHasTimestamp(t *testing.T) {
	entry := NewLogEntry(InfoLevel, "comp", "msg")
	if entry.Timestamp == "" {
		t.Fatal("expected non-empty timestamp")
	}
	if entry.Level != InfoLevel {
		t.Fatalf("expected InfoLevel, got %v", entry.Level)
	}
	if entry.Component != "comp" {
		t.Fatalf("expected component 'comp', got %q", entry.Component)
	}
	if entry.Message != "msg" {
		t.Fatalf("expected message 'msg', got %q", entry.Message)
	}
}

func TestLogLevelConstantsOrder(t *testing.T) {
	if DebugLevel >= InfoLevel {
		t.Fatal("expected DebugLevel < InfoLevel")
	}
	if InfoLevel >= WarnLevel {
		t.Fatal("expected InfoLevel < WarnLevel")
	}
	if WarnLevel >= ErrorLevel {
		t.Fatal("expected WarnLevel < ErrorLevel")
	}
}

func TestSubscribeWithBufferDoesNotBlock(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(1)

	for i := 0; i < 10; i++ {
		l.Info("comp", "msg")
	}

	count := 0
	for {
		select {
		case <-sub.Ch():
			count++
		default:
			goto done
		}
	}
done:
	if count == 0 {
		t.Fatal("expected at least 1 entry from subscriber")
	}
}

func TestSubscribeDoneClosedOnUnsubscribe(t *testing.T) {
	l := NewLogger()
	sub := l.Subscribe(10)
	l.Unsubscribe(sub)

	select {
	case _, ok := <-sub.Done():
		if ok {
			t.Fatal("expected done channel to be closed")
		}
	default:
		t.Fatal("expected immediate read from closed done channel")
	}
}

func TestLogTrimsExcessEntries(t *testing.T) {
	l := NewLogger()

	for i := 0; i < 15000; i++ {
		l.Info("comp", "msg")
	}

	recent := l.Recent(10000)
	if len(recent) > 10000 {
		t.Fatalf("expected at most 10000 entries, got %d", len(recent))
	}
	if len(recent) < 5000 {
		t.Fatalf("expected at least 5000 entries after trim, got %d", len(recent))
	}
}

func TestSetFile(t *testing.T) {
	l := NewLogger()
	path := filepath.Join(t.TempDir(), "test.log")

	err := l.SetFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected file, got directory")
	}

	l.Info("test", "file log message")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty log file")
	}
}

func TestNewSystemService(t *testing.T) {
	s := NewSystemService()
	if s == nil {
		t.Fatal("expected non-nil system service")
	}
}

func TestGetMetrics(t *testing.T) {
	s := NewSystemService()
	metrics := s.GetMetrics()

	if metrics.CPU < 0 {
		t.Fatal("expected CPU >= 0")
	}
	if metrics.RAMTotalMB < 1 {
		t.Fatal("expected RAM total > 0")
	}
	if metrics.RAMUsedMB == 0 {
		t.Fatal("expected RAM used > 0")
	}
}

func TestGetGPUPercent(t *testing.T) {
	pct := getGPUPercent()

	if pct < 0 {
		t.Fatal("expected GPU percent >= 0")
	}
	if pct > 100 {
		t.Fatal("expected GPU percent <= 100")
	}
}
