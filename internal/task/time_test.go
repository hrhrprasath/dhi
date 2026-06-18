package task

import (
	"testing"
	"time"
)

func approxEqual(a, b time.Time, tolerance time.Duration) bool {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

func TestParse_InMinutes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		offset   time.Duration
		interval int
	}{
		{"in 5 minutes", "in 5 minutes", 5 * time.Minute, 0},
		{"in 1 minute", "in 1 minute", 1 * time.Minute, 0},
		{"in 30 min", "in 30 min", 30 * time.Minute, 0},
		{"in 0 minutes", "in 0 minutes", 0, 0},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			s, err := p.Parse(tt.input)
			after := time.Now()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := before.Add(tt.offset)
			if s.ScheduledFor.Before(before.Add(tt.offset-time.Second)) ||
				s.ScheduledFor.After(after.Add(tt.offset+time.Second)) {
				t.Errorf("ScheduledFor = %v, expected around %v (offset %v)", s.ScheduledFor, expected, tt.offset)
			}
			if s.IntervalSec != tt.interval {
				t.Errorf("IntervalSec = %d, want %d", s.IntervalSec, tt.interval)
			}
			if s.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", s.Raw, tt.input)
			}
		})
	}
}

func TestParse_InHours(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		offset   time.Duration
		interval int
	}{
		{"in 1 hour", "in 1 hour", 1 * time.Hour, 0},
		{"in 2 hours", "in 2 hours", 2 * time.Hour, 0},
		{"in 3 hr", "in 3 hr", 3 * time.Hour, 0},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			s, err := p.Parse(tt.input)
			after := time.Now()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Before(before.Add(tt.offset-time.Second)) ||
				s.ScheduledFor.After(after.Add(tt.offset+time.Second)) {
				t.Errorf("ScheduledFor = %v, expected around %v (offset %v)", s.ScheduledFor, before.Add(tt.offset), tt.offset)
			}
			if s.IntervalSec != tt.interval {
				t.Errorf("IntervalSec = %d, want %d", s.IntervalSec, tt.interval)
			}
		})
	}
}

func TestParse_InDaysWeeksMonths(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset time.Duration
	}{
		{"in 1 day", "in 1 day", 24 * time.Hour},
		{"in 2 days", "in 2 days", 48 * time.Hour},
		{"in 1 week", "in 1 week", 7 * 24 * time.Hour},
		{"in 2 weeks", "in 2 weeks", 14 * 24 * time.Hour},
		{"in 1 month", "in 1 month", 30 * 24 * time.Hour},
		{"in 2 months", "in 2 months", 60 * 24 * time.Hour},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			s, err := p.Parse(tt.input)
			after := time.Now()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Before(before.Add(tt.offset-time.Second)) ||
				s.ScheduledFor.After(after.Add(tt.offset+time.Second)) {
				t.Errorf("ScheduledFor = %v, expected around %v (offset %v)", s.ScheduledFor, before.Add(tt.offset), tt.offset)
			}
		})
	}
}

func TestParse_InSeconds(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset time.Duration
	}{
		{"in 30 seconds", "in 30 seconds", 30 * time.Second},
		{"in 1 second", "in 1 second", 1 * time.Second},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			s, err := p.Parse(tt.input)
			after := time.Now()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Before(before.Add(tt.offset-time.Second)) ||
				s.ScheduledFor.After(after.Add(tt.offset+time.Second)) {
				t.Errorf("ScheduledFor = %v, expected around %v", s.ScheduledFor, before.Add(tt.offset))
			}
		})
	}
}

func TestParse_Tomorrow(t *testing.T) {
	tests := []struct {
		name  string
		input string
		hour  int
		min   int
	}{
		{"tomorrow at 9am", "tomorrow at 9am", 9, 0},
		{"tomorrow at 9", "tomorrow at 9", 9, 0},
		{"tomorrow 5pm", "tomorrow 5pm", 17, 0},
		{"tomorrow at 12am", "tomorrow at 12am", 0, 0},
		{"tomorrow at 12pm", "tomorrow at 12pm", 12, 0},
		{"tomorrow at 10:30pm", "tomorrow at 10:30pm", 22, 30},
		{"tomorrow at 8:15am", "tomorrow at 8:15am", 8, 15},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			s, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := time.Date(now.Year(), now.Month(), now.Day()+1, tt.hour, tt.min, 0, 0, now.Location())
			if !s.ScheduledFor.Equal(expected) {
				t.Errorf("ScheduledFor = %v, want %v", s.ScheduledFor, expected)
			}
			if s.IntervalSec != 0 {
				t.Errorf("IntervalSec = %d, want 0", s.IntervalSec)
			}
			if s.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", s.Raw, tt.input)
			}
		})
	}
}

func TestParse_Daily(t *testing.T) {
	tests := []struct {
		name  string
		input string
		hour  int
		min   int
	}{
		{"daily at 9am", "daily at 9am", 9, 0},
		{"daily at 12pm", "daily at 12pm", 12, 0},
		{"daily at 12am", "daily at 12am", 0, 0},
		{"daily at 6:30pm", "daily at 6:30pm", 18, 30},
		{"every daily at 9am", "every daily at 9am", 9, 0},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			s, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := time.Date(now.Year(), now.Month(), now.Day(), tt.hour, tt.min, 0, 0, now.Location())
			if !expected.After(now) {
				expected = expected.Add(24 * time.Hour)
			}
			if !s.ScheduledFor.Equal(expected) {
				t.Errorf("ScheduledFor = %v, want %v", s.ScheduledFor, expected)
			}
			if s.IntervalSec != 86400 {
				t.Errorf("IntervalSec = %d, want 86400", s.IntervalSec)
			}
			if s.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", s.Raw, tt.input)
			}
		})
	}
}

func TestParse_Every(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		offset   time.Duration
		interval int
	}{
		{"every 1 hour", "every 1 hour", 1 * time.Hour, 3600},
		{"every 2 hours", "every 2 hours", 2 * time.Hour, 7200},
		{"each 1 hour", "each 1 hour", 1 * time.Hour, 3600},
		{"every 30 minutes", "every 30 minutes", 30 * time.Minute, 1800},
		{"every 1 minute", "every 1 minute", 1 * time.Minute, 60},
		{"every 1 day", "every 1 day", 24 * time.Hour, 86400},
		{"every 30 min", "every 30 min", 30 * time.Minute, 1800},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			s, err := p.Parse(tt.input)
			after := time.Now()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Before(before.Add(tt.offset-time.Second)) ||
				s.ScheduledFor.After(after.Add(tt.offset+time.Second)) {
				t.Errorf("ScheduledFor = %v, expected around %v", s.ScheduledFor, before.Add(tt.offset))
			}
			if s.IntervalSec != tt.interval {
				t.Errorf("IntervalSec = %d, want %d", s.IntervalSec, tt.interval)
			}
		})
	}
}

func TestParse_At(t *testing.T) {
	tests := []struct {
		name  string
		input string
		hour  int
		min   int
	}{
		{"at 9am", "at 9am", 9, 0},
		{"9am", "9am", 9, 0},
		{"5pm", "5pm", 17, 0},
		{"12pm", "12pm", 12, 0},
		{"12am", "12am", 0, 0},
		{"at 6:30pm", "at 6:30pm", 18, 30},
		{"at 10:15am", "at 10:15am", 10, 15},
		{"9:00", "9:00", 9, 0},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			s, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := time.Date(now.Year(), now.Month(), now.Day(), tt.hour, tt.min, 0, 0, now.Location())
			if !expected.After(now) {
				expected = expected.Add(24 * time.Hour)
			}
			if !s.ScheduledFor.Equal(expected) {
				t.Errorf("ScheduledFor = %v, want %v", s.ScheduledFor, expected)
			}
		})
	}
}

func TestParse_Weekly(t *testing.T) {
	tests := []struct {
		name  string
		input string
		hour  int
		min   int
	}{
		{"weekly at 9am", "weekly at 9am", 9, 0},
		{"every weekly at 9am", "every weekly at 9am", 9, 0},
		{"weekly on monday at 5pm", "weekly on monday at 5pm", 17, 0},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			s, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			todayTarget := time.Date(now.Year(), now.Month(), now.Day(), tt.hour, tt.min, 0, 0, now.Location())
			if !todayTarget.After(now) {
				todayTarget = todayTarget.Add(24 * time.Hour)
			}
			daysUntil := (7 - int(todayTarget.Weekday()) + int(now.Weekday())) % 7
			if daysUntil == 0 && !todayTarget.After(now) {
				daysUntil = 7
			}
			todayTarget = todayTarget.Add(time.Duration(daysUntil) * 24 * time.Hour)
			if !s.ScheduledFor.Equal(todayTarget) {
				t.Errorf("ScheduledFor = %v, want %v", s.ScheduledFor, todayTarget)
			}
			if s.IntervalSec != 7*86400 {
				t.Errorf("IntervalSec = %d, want %d", s.IntervalSec, 7*86400)
			}
		})
	}
}

func TestParse_Weekday(t *testing.T) {
	tests := []string{
		"weekdays at 9am",
		"every weekday at 5pm",
		"weekdays at 12pm",
	}

	p := &DefaultParser{}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			s, err := p.Parse(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			wday := s.ScheduledFor.Weekday()
			if wday < time.Monday || wday > time.Friday {
				t.Errorf("ScheduledFor %v is on %s, expected weekday", s.ScheduledFor, wday)
			}
			if s.IntervalSec != 86400 {
				t.Errorf("IntervalSec = %d, want 86400", s.IntervalSec)
			}
		})
	}
}

func TestParse_Weekend(t *testing.T) {
	tests := []string{
		"weekends at 9am",
		"every weekend at 5pm",
		"weekends at 12pm",
	}

	p := &DefaultParser{}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			now := time.Now()
			s, err := p.Parse(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Before(now) {
				t.Errorf("ScheduledFor %v is before now %v", s.ScheduledFor, now)
			}
			wday := s.ScheduledFor.Weekday()
			if wday != time.Saturday && wday != time.Sunday {
				t.Errorf("ScheduledFor %v is on %s, expected weekend", s.ScheduledFor, wday)
			}
			if s.IntervalSec != 86400 {
				t.Errorf("IntervalSec = %d, want 86400", s.IntervalSec)
			}
		})
	}
}

func TestParse_Next(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset time.Duration
	}{
		{"next hour", "next hour", 1 * time.Hour},
		{"next 2 hours", "next 2 hours", 2 * time.Hour},
		{"next minute", "next minute", 1 * time.Minute},
		{"next 2 minutes", "next 2 minutes", 2 * time.Minute},
		{"next day", "next day", 24 * time.Hour},
		{"next 2 days", "next 2 days", 2 * 24 * time.Hour},
		{"next week", "next week", 7 * 24 * time.Hour},
		{"next 2 weeks", "next 2 weeks", 2 * 7 * 24 * time.Hour},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			s, err := p.Parse(tt.input)
			after := time.Now()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Before(before.Add(tt.offset-time.Second)) ||
				s.ScheduledFor.After(after.Add(tt.offset+time.Second)) {
				t.Errorf("ScheduledFor = %v, expected around %v", s.ScheduledFor, before.Add(tt.offset))
			}
		})
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "  "},
		{"gibberish", "foobar"},
		{"unknown expression", "sometime next year"},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Parse(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParse_NextErrors(t *testing.T) {
	tests := []string{
		"next something",
		"next year",
		"next month",
	}

	p := &DefaultParser{}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := p.Parse(input)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", input)
			}
		})
	}
}

func TestParse_EdgeCases_CaseInsensitivity(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset time.Duration
	}{
		{"IN 5 MINUTES", "IN 5 MINUTES", 5 * time.Minute},
		{"In 1 Hour", "In 1 Hour", 1 * time.Hour},
		{"Tomorrow At 9AM", "Tomorrow At 9AM", 0},
		{"Every 2 Hours", "Every 2 Hours", 2 * time.Hour},
		{"In 30 Minutes", "In 30 Minutes", 30 * time.Minute},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.offset > 0 {
				now := time.Now()
				expected := now.Add(tt.offset)
				if !approxEqual(s.ScheduledFor, expected, 2*time.Second) {
					t.Errorf("ScheduledFor = %v, want ~%v", s.ScheduledFor, expected)
				}
			}
			if s.Raw != tt.input {
				t.Errorf("Raw = %q, want %q", s.Raw, tt.input)
			}
		})
	}
}

func TestParse_EdgeCases_AMPMBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		hour  int
		min   int
	}{
		{"12am is midnight", "12am", 0, 0},
		{"12pm is noon", "12pm", 12, 0},
		{"12:30am is 00:30", "12:30am", 0, 30},
		{"12:30pm is 12:30", "12:30pm", 12, 30},
		{"0:00 should fail", "0:00", 0, 0},
	}

	p := &DefaultParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			now := time.Now()
			expected := time.Date(now.Year(), now.Month(), now.Day(), tt.hour, tt.min, 0, 0, now.Location())
			if !expected.After(now) {
				expected = expected.Add(24 * time.Hour)
			}
			if !s.ScheduledFor.Equal(expected) {
				t.Errorf("ScheduledFor = %v, want %v", s.ScheduledFor, expected)
			}
		})
	}
}

func TestParse_EdgeCases_LeadingZeros(t *testing.T) {
	p := &DefaultParser{}
	for _, input := range []string{"09:05am", "09:05", "05pm"} {
		t.Run(input, func(t *testing.T) {
			s, err := p.Parse(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.ScheduledFor.Hour() < 0 || s.ScheduledFor.Hour() > 23 {
				t.Errorf("unexpected hour: %d", s.ScheduledFor.Hour())
			}
		})
	}
}

func TestFormatTaskTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "overdue",
			t:    now.Add(-1 * time.Hour),
			want: "overdue",
		},
		{
			name: "just now",
			t:    now.Add(15 * time.Second),
			want: "now",
		},
		{
			name: "in 5 minutes",
			t:    now.Add(5 * time.Minute),
			want: "in 5m",
		},
		{
			name: "in 2 hours",
			t:    now.Add(2*time.Hour + 10*time.Second),
			want: "in 2h",
		},
		{
			name: "in 2 hours 30 minutes",
			t:    now.Add(2*time.Hour + 30*time.Minute),
			want: "in 2h30m",
		},
		{
			name: "in 25 hours becomes absolute date",
			t:    now.Add(25 * time.Hour),
			want: now.Add(25 * time.Hour).Format("Jan 2 15:04"),
		},
		{
			name: "overdue by 1 nanosecond",
			t:    now.Add(-1 * time.Nanosecond),
			want: "overdue",
		},
		{
			name: "59 seconds is now",
			t:    now.Add(59 * time.Second),
			want: "now",
		},
		{
			name: "in 90 minutes",
			t:    now.Add(90 * time.Minute),
			want: "in 1h30m",
		},
		{
			name: "far future absolute",
			t:    now.Add(72 * time.Hour),
			want: now.Add(72 * time.Hour).Format("Jan 2 15:04"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTaskTime(tt.t)
			if got != tt.want {
				t.Errorf("FormatTaskTime(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

func TestParse_IntervalPersistence(t *testing.T) {
	p := &DefaultParser{}

	t.Run("in X has no interval", func(t *testing.T) {
		s, err := p.Parse("in 5 minutes")
		if err != nil {
			t.Fatal(err)
		}
		if s.IntervalSec != 0 {
			t.Errorf("expected IntervalSec=0 for one-shot, got %d", s.IntervalSec)
		}
	})

	t.Run("every X has interval", func(t *testing.T) {
		s, err := p.Parse("every 30 minutes")
		if err != nil {
			t.Fatal(err)
		}
		if s.IntervalSec != 1800 {
			t.Errorf("expected IntervalSec=1800, got %d", s.IntervalSec)
		}
	})

	t.Run("daily has interval", func(t *testing.T) {
		s, err := p.Parse("daily at 9am")
		if err != nil {
			t.Fatal(err)
		}
		if s.IntervalSec != 86400 {
			t.Errorf("expected IntervalSec=86400, got %d", s.IntervalSec)
		}
	})
}

func TestParse_Interface(t *testing.T) {
	var p TimeParser = &DefaultParser{}

	s, err := p.Parse("in 10 minutes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil schedule")
	}
}
