package task

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Schedule struct {
	ScheduledFor time.Time
	IntervalSec  int
	Raw          string
}

type TimeParser interface {
	Parse(when string) (*Schedule, error)
}

type DefaultParser struct{}

var (
	reIn       = regexp.MustCompile(`(?i)^in\s+(\d+)\s*(second|minute|min|hour|hr|day|week|month)s?\s*$`)
	reAt       = regexp.MustCompile(`(?i)^(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*$`)
	reEvery    = regexp.MustCompile(`(?i)^(?:every|each)\s+(\d+)\s*(second|minute|min|hour|hr|day)s?\s*$`)
	reDaily    = regexp.MustCompile(`(?i)^(?:every\s+)?daily\s+(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*$`)
	reWeekly   = regexp.MustCompile(`(?i)^(?:every\s+)?weekly\s+(?:on\s+\w+)?\s*(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*$`)
	reTomorrow = regexp.MustCompile(`(?i)^tomorrow\s+(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*$`)
	reWeekday  = regexp.MustCompile(`(?i)^(?:every\s+)?weekdays?\s+(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*$`)
	reWeekend  = regexp.MustCompile(`(?i)^(?:every\s+)?weekends?\s+(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*$`)
)

func (p *DefaultParser) Parse(when string) (*Schedule, error) {
	s := strings.TrimSpace(when)
	if s == "" {
		return nil, fmt.Errorf("empty time expression")
	}

	now := time.Now()

	if m := reIn.FindStringSubmatch(s); m != nil {
		return p.parseIn(m, now)
	}
	if m := reEvery.FindStringSubmatch(s); m != nil {
		return p.parseEvery(m, now)
	}
	if m := reDaily.FindStringSubmatch(s); m != nil {
		return p.parseDaily(m, now)
	}
	if m := reWeekly.FindStringSubmatch(s); m != nil {
		return p.parseWeekly(m, now)
	}
	if m := reTomorrow.FindStringSubmatch(s); m != nil {
		return p.parseTomorrow(m, now)
	}
	if m := reWeekday.FindStringSubmatch(s); m != nil {
		return p.parseWeekday(m, now, true)
	}
	if m := reWeekend.FindStringSubmatch(s); m != nil {
		return p.parseWeekday(m, now, false)
	}
	if m := reAt.FindStringSubmatch(s); m != nil {
		return p.parseAt(m, now)
	}

	if strings.Contains(s, "next") {
		return p.parseNext(s, now)
	}

	return nil, fmt.Errorf("unrecognized time expression: %s", s)
}

func (p *DefaultParser) parseIn(m []string, now time.Time) (*Schedule, error) {
	n, _ := strconv.Atoi(m[1])
	unit := strings.ToLower(m[2])
	dur := map[string]time.Duration{
		"second": time.Second, "minute": time.Minute, "min": time.Minute,
		"hour": time.Hour, "hr": time.Hour,
		"day": 24 * time.Hour, "week": 7 * 24 * time.Hour,
		"month": 30 * 24 * time.Hour,
	}[unit]
	return &Schedule{
		ScheduledFor: now.Add(time.Duration(n) * dur),
		Raw:          m[0],
	}, nil
}

func (p *DefaultParser) parseEvery(m []string, now time.Time) (*Schedule, error) {
	n, _ := strconv.Atoi(m[1])
	unit := strings.ToLower(m[2])
	secs := map[string]int{
		"second": 1, "minute": 60, "min": 60,
		"hour": 3600, "hr": 3600, "day": 86400,
	}[unit]
	interval := n * secs
	return &Schedule{
		ScheduledFor: now.Add(time.Duration(interval) * time.Second),
		IntervalSec:  interval,
		Raw:          m[0],
	}, nil
}

func (p *DefaultParser) parseAt(m []string, now time.Time) (*Schedule, error) {
	hour, min, _ := parseTimeParts(m)
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !t.After(now) {
		t = t.Add(24 * time.Hour)
	}
	return &Schedule{ScheduledFor: t, Raw: m[0]}, nil
}

func (p *DefaultParser) parseDaily(m []string, now time.Time) (*Schedule, error) {
	hour, min, _ := parseTimeParts(m)
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !t.After(now) {
		t = t.Add(24 * time.Hour)
	}
	return &Schedule{ScheduledFor: t, IntervalSec: 86400, Raw: m[0]}, nil
}

func (p *DefaultParser) parseWeekly(m []string, now time.Time) (*Schedule, error) {
	hour, min, _ := parseTimeParts(m)
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !t.After(now) {
		t = t.Add(24 * time.Hour)
	}
	daysUntil := (7 - int(t.Weekday()) + int(now.Weekday())) % 7
	if daysUntil == 0 && !t.After(now) {
		daysUntil = 7
	}
	t = t.Add(time.Duration(daysUntil) * 24 * time.Hour)
	return &Schedule{ScheduledFor: t, IntervalSec: 7 * 86400, Raw: m[0]}, nil
}

func (p *DefaultParser) parseTomorrow(m []string, now time.Time) (*Schedule, error) {
	hour, min, _ := parseTimeParts(m)
	t := time.Date(now.Year(), now.Month(), now.Day()+1, hour, min, 0, 0, now.Location())
	return &Schedule{ScheduledFor: t, Raw: m[0]}, nil
}

func (p *DefaultParser) parseWeekday(m []string, now time.Time, isWeekday bool) (*Schedule, error) {
	hour, min, _ := parseTimeParts(m)
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())

	interval := 86400

	for i := 0; i < 7; i++ {
		check := t.Add(time.Duration(i) * 24 * time.Hour)
		if (isWeekday && check.Weekday() >= time.Monday && check.Weekday() <= time.Friday) ||
			(!isWeekday && (check.Weekday() == time.Saturday || check.Weekday() == time.Sunday)) {
			if check.After(now) || (i == 0 && check.Equal(t)) {
				return &Schedule{ScheduledFor: check, IntervalSec: interval, Raw: m[0]}, nil
			}
		}
	}
	return &Schedule{ScheduledFor: t.Add(7 * 24 * time.Hour), IntervalSec: interval, Raw: m[0]}, nil
}

func (p *DefaultParser) parseNext(s string, now time.Time) (*Schedule, error) {
	s = strings.ToLower(s)
	var n = 1
	if strings.Contains(s, "2 ") || strings.Contains(s, "two ") {
		n = 2
	}
	if strings.Contains(s, "hour") || strings.Contains(s, "hr") {
		return &Schedule{ScheduledFor: now.Add(time.Duration(n) * time.Hour), Raw: s}, nil
	}
	if strings.Contains(s, "min") {
		return &Schedule{ScheduledFor: now.Add(time.Duration(n) * time.Minute), Raw: s}, nil
	}
	if strings.Contains(s, "day") {
		return &Schedule{ScheduledFor: now.Add(time.Duration(n) * 24 * time.Hour), Raw: s}, nil
	}
	if strings.Contains(s, "week") {
		return &Schedule{ScheduledFor: now.Add(time.Duration(n) * 7 * 24 * time.Hour), Raw: s}, nil
	}
	return nil, fmt.Errorf("unrecognized next expression: %s", s)
}

func parseTimeParts(m []string) (hour, min int, mer string) {
	hour, _ = strconv.Atoi(m[1])
	if len(m) > 2 && m[2] != "" {
		min, _ = strconv.Atoi(m[2])
	}
	if len(m) > 3 {
		mer = strings.ToLower(m[3])
	}
	if mer == "pm" && hour < 12 {
		hour += 12
	}
	if mer == "am" && hour == 12 {
		hour = 0
	}
	return
}

func FormatTaskTime(t time.Time) string {
	diff := time.Until(t)
	if diff < 0 {
		return "overdue"
	}
	if diff < time.Minute {
		return "now"
	}
	if diff < time.Hour {
		mins := int(math.Round(diff.Minutes()))
		return fmt.Sprintf("in %dm", mins)
	}
	if diff < 24*time.Hour {
		hours := int(math.Round(diff.Hours()))
		mins := int(math.Round(math.Mod(diff.Minutes(), 60)))
		if mins == 0 {
			return fmt.Sprintf("in %dh", hours)
		}
		return fmt.Sprintf("in %dh%dm", hours, mins)
	}
	return t.Format("Jan 2 15:04")
}
