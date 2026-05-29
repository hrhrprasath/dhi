package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Level     LogLevel
	Component string
	Message   string
	Timestamp string
}

func NewLogEntry(level LogLevel, component, message string) LogEntry {
	return LogEntry{
		Level:     level,
		Component: component,
		Message:   message,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
}

type LogSubscriber struct {
	ch     chan LogEntry
	done   chan struct{}
	filter func(LogEntry) bool
}

type Logger struct {
	mu          sync.RWMutex
	entries     []LogEntry
	subscribers []*LogSubscriber
	file        *os.File
}

func NewLogger() *Logger {
	l := &Logger{
		entries: make([]LogEntry, 0, 1000),
	}
	return l
}

func (l *Logger) SetFile(path string) error {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	l.file = f
	return nil
}

func (l *Logger) Log(level LogLevel, component, message string) {
	entry := NewLogEntry(level, component, message)
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > 10000 {
		l.entries = l.entries[len(l.entries)-5000:]
	}
	subs := make([]*LogSubscriber, len(l.subscribers))
	copy(subs, l.subscribers)
	l.mu.Unlock()

	for _, sub := range subs {
		if sub.filter == nil || sub.filter(entry) {
			select {
			case sub.ch <- entry:
			default:
			}
		}
	}

	if l.file != nil {
		fmt.Fprintf(l.file, "[%s][%s] %s\n", entry.Timestamp, component, level.String())
	}

	if level >= WarnLevel {
		fmt.Fprintf(os.Stderr, "[%s][%s] %s\n", component, level.String(), message)
	}
}

func (l *Logger) Info(component, message string) {
	l.Log(InfoLevel, component, message)
}

func (l *Logger) Warn(component, message string) {
	l.Log(WarnLevel, component, message)
}

func (l *Logger) Error(component, message string) {
	l.Log(ErrorLevel, component, message)
}

func (l *Logger) Debug(component, message string) {
	l.Log(DebugLevel, component, message)
}

func (l *Logger) Subscribe(bufSize int) *LogSubscriber {
	sub := &LogSubscriber{
		ch:   make(chan LogEntry, bufSize),
		done: make(chan struct{}),
	}
	l.mu.Lock()
	l.subscribers = append(l.subscribers, sub)
	l.mu.Unlock()
	return sub
}

func (l *Logger) Unsubscribe(sub *LogSubscriber) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, s := range l.subscribers {
		if s == sub {
			l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
			close(sub.done)
			return
		}
	}
}

func (l *Logger) Recent(n int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) <= n {
		out := make([]LogEntry, len(l.entries))
		copy(out, l.entries)
		return out
	}
	out := make([]LogEntry, n)
	copy(out, l.entries[len(l.entries)-n:])
	return out
}

func (sub *LogSubscriber) Ch() <-chan LogEntry { return sub.ch }
func (sub *LogSubscriber) Done() <-chan struct{} { return sub.done }
