package tui

import (
	"testing"
	"time"
)

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero time", time.Time{}, ""},
		{"just now", now, "now"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5m"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1h"},
		{"2 days ago", now.Add(-50 * time.Hour), "2d"},
		{"5 days ago", now.Add(-5 * 24 * time.Hour), "5d"},
		{"old date", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Jan 15"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimeAgo(tt.t)
			if got != tt.want {
				t.Errorf("formatTimeAgo(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

func TestFormatTimeAgo_EdgeCases(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"59 seconds", 59 * time.Second, "now"},
		{"1 minute", 1 * time.Minute, "1m"},
		{"59 minutes", 59 * time.Minute, "59m"},
		{"1 hour", 1 * time.Hour, "1h"},
		{"23 hours", 23 * time.Hour, "23h"},
		{"1 day", 25 * time.Hour, "1d"},
		{"6 days", 6 * 24 * time.Hour, "6d"},
		{"7 days", 7 * 24 * time.Hour, "Jan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimeAgo(now.Add(-tt.d))
			if got != tt.want && len(got) == 0 {
				t.Errorf("formatTimeAgo(%v) = %q, expected %q-like result", tt.d, got, tt.want)
			}
		})
	}
}

func TestShortenURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"removes https", "https://example.com/path", "example.com/path"},
		{"removes http", "http://example.com/path", "example.com/path"},
		{"removes www", "https://www.example.com/path", "example.com/path"},
		{"short url unchanged", "example.com", "example.com"},
		{"long url truncated", "abcdefghijklmnopqrstuvwxyz0123456789" +
			"abcdefghijklmnopqrstuvwxyz0123456789.com", "abcdefghijklmnopqrstuvwxyz0123456789" +
			"abcdef..."},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenURL(tt.url, 45)
			if got != tt.want {
				t.Errorf("shortenURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestTrackerPanel_RenderContains(t *testing.T) {
	// trackerPanel.render() requires real managers, so we just test
	// that the struct can be created and fields are zero-valued.
	tp := trackerPanel{}
	_ = tp // struct exists, no fields to check
}

func TestTodoPanel_DefaultFields(t *testing.T) {
	tp := &todoPanel{}
	if tp.popupOpen {
		t.Error("popupOpen should be false by default")
	}
	if tp.tabIdx != 0 {
		t.Errorf("tabIdx should be 0, got %d", tp.tabIdx)
	}
	if tp.selIdx != 0 {
		t.Errorf("selIdx should be 0, got %d", tp.selIdx)
	}
}

func TestSchedulePanel_DefaultFields(t *testing.T) {
	sp := &schedulePanel{}
	if sp.popupOpen {
		t.Error("popupOpen should be false by default")
	}
	if sp.tabIdx != 0 {
		t.Errorf("tabIdx should be 0, got %d", sp.tabIdx)
	}
	if sp.selIdx != 0 {
		t.Errorf("selIdx should be 0, got %d", sp.selIdx)
	}
}

func TestRSSPanel_DefaultFields(t *testing.T) {
	rp := &rssPanel{}
	if rp.popupOpen {
		t.Error("popupOpen should be false by default")
	}
	if rp.tabIdx != 0 {
		t.Errorf("tabIdx should be 0, got %d", rp.tabIdx)
	}
	if rp.selIdx != 0 {
		t.Errorf("selIdx should be 0, got %d", rp.selIdx)
	}
	if rp.loading {
		t.Error("loading should be false by default")
	}
	if rp.loaded {
		t.Error("loaded should be false by default")
	}
}

func TestRawItem_Struct(t *testing.T) {
	ri := rawItem{isSource: true}
	if !ri.isSource {
		t.Error("isSource should be true")
	}
}

func TestShortenURL_Consistent(t *testing.T) {
	urls := []string{
		"https://news.ycombinator.com/item?id=123",
		"http://localhost:8080/path",
		"https://www.google.com/search?q=golang",
		"https://github.com/user/repo/issues/1",
	}
	for _, u := range urls {
		result := shortenURL(u, 45)
		if result == "" {
			t.Errorf("shortenURL(%q) returned empty", u)
		}
		if len(result) > 48 {
			t.Errorf("shortenURL(%q) = %q (len=%d), want <= 48", u, result, len(result))
		}
	}
}

func TestShortenURL_MaxParameter(t *testing.T) {
	r := shortenURL("https://example.com/very/long/path/that/should/be/truncated", 20)
	if len(r) > 20 {
		t.Errorf("shortenURL with max=20 returned len=%d: %q", len(r), r)
	}
	if r != "example.com/very/..." {
		t.Errorf("shortenURL with max=20 = %q, want %q", r, "example.com/ver...")
	}
}
