package rss

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gilliek/go-opml/opml"
	"github.com/mmcdole/gofeed"
)

type FeedSource struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	FeedURL string    `json:"feed_url"`
	SiteURL string    `json:"site_url,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

type FeedEntry struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	FeedURL     string    `json:"feed_url"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	FetchedAt   time.Time `json:"fetched_at"`
	IsBookmark  bool      `json:"is_bookmark"`
}

type Manager struct {
	mu         sync.Mutex
	dataDir    string
	opmlPath   string
	sources    []FeedSource
	entries    []FeedEntry
	looperStop chan struct{}
	onUpdate   func()
}

func NewManager(dataDir string) *Manager {
	dir := filepath.Join(dataDir, "rss")
	os.MkdirAll(dir, 0755)

	opmlPath := filepath.Join(dir, "feed.opml")
	if e := os.Getenv("DHI_RSS_OPML_PATH"); e != "" {
		opmlPath = e
	}

	m := &Manager{
		dataDir:    dir,
		opmlPath:   opmlPath,
		looperStop: make(chan struct{}),
	}

	m.syncFromOPML()
	m.entries = m.loadEntries()
	return m
}

func (m *Manager) SetOnUpdate(fn func()) {
	m.onUpdate = fn
}

// ─── OPML ───

func (m *Manager) syncFromOPML() {
	m.sources = nil
	data, err := os.ReadFile(m.opmlPath)
	if err != nil {
		return
	}
	doc, err := opml.NewOPML(data)
	if err != nil {
		return
	}
	if len(doc.Body.Outlines) == 0 {
		return
	}
	now := time.Now()
	for _, outline := range doc.Body.Outlines {
		m.collectOutlines(outline, "", now)
	}
}

func (m *Manager) collectOutlines(outline opml.Outline, parentCategory string, now time.Time) {
	category := parentCategory
	if outline.Text != "" && outline.XMLURL == "" {
		category = outline.Text
	}
	if outline.XMLURL != "" {
		id := fmt.Sprintf("%x", outline.XMLURL)
		m.sources = append(m.sources, FeedSource{
			ID:      id,
			Title:   outline.Title,
			FeedURL: outline.XMLURL,
			SiteURL: outline.URL,
			AddedAt: now,
		})
	}
	for _, child := range outline.Outlines {
		m.collectOutlines(child, category, now)
	}
}

func (m *Manager) saveOPML() {
	doc := opml.OPML{
		Version: "2.0",
		Head: opml.Head{
			Title: "DHI Harness Feeds",
		},
	}
	var outlines []opml.Outline
	for _, src := range m.sources {
		outlines = append(outlines, opml.Outline{
			Text:   src.Title,
			Title:  src.Title,
			Type:   "rss",
			XMLURL: src.FeedURL,
			URL:    src.SiteURL,
		})
	}
	doc.Body = opml.Body{Outlines: outlines}
	data, err := doc.XML()
	if err != nil {
		return
	}
	os.WriteFile(m.opmlPath, []byte(data), 0644)
}

// ─── Sources ───

func (m *Manager) AddSource(feedURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sources {
		if s.FeedURL == feedURL {
			return fmt.Errorf("feed already exists: %s", feedURL)
		}
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(feedURL)
	if err != nil {
		return fmt.Errorf("failed to parse feed: %w", err)
	}

	title := feed.Title
	if title == "" {
		title = feedURL
	}
	siteURL := ""
	if feed.Link != "" {
		siteURL = feed.Link
	}

	id := fmt.Sprintf("%x", feedURL)
	src := FeedSource{
		ID:      id,
		Title:   title,
		FeedURL: feedURL,
		SiteURL: siteURL,
		AddedAt: time.Now(),
	}
	m.sources = append(m.sources, src)
	m.saveOPML()
	return nil
}

func (m *Manager) RemoveSource(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, s := range m.sources {
		if s.ID == id {
			m.sources = append(m.sources[:i], m.sources[i+1:]...)
			m.saveOPML()
			var kept []FeedEntry
			for _, e := range m.entries {
				if e.SourceID != id {
					kept = append(kept, e)
				}
			}
			m.entries = kept
			m.saveEntries()
			return true
		}
	}
	return false
}

func (m *Manager) ListSources() []FeedSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]FeedSource, len(m.sources))
	copy(out, m.sources)
	return out
}

func (m *Manager) SourceCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sources)
}

// ─── Entries ───

func (m *Manager) ListEntries(sourceID string) []FeedEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sourceID == "" {
		out := make([]FeedEntry, len(m.entries))
		copy(out, m.entries)
		return out
	}
	var out []FeedEntry
	for _, e := range m.entries {
		if e.SourceID == sourceID {
			out = append(out, e)
		}
	}
	return out
}

func (m *Manager) GetBookmarks() []FeedEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []FeedEntry
	for _, e := range m.entries {
		if e.IsBookmark {
			out = append(out, e)
		}
	}
	return out
}

func (m *Manager) EntryCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func (m *Manager) BookmarkCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.entries {
		if e.IsBookmark {
			n++
		}
	}
	return n
}

func (m *Manager) ToggleBookmark(entryID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.entries {
		if m.entries[i].ID == entryID {
			m.entries[i].IsBookmark = !m.entries[i].IsBookmark
			m.saveEntries()
			return
		}
	}
}

// ─── Fetch ───

func (m *Manager) FetchAll() {
	m.mu.Lock()
	sources := make([]FeedSource, len(m.sources))
	copy(sources, m.sources)
	m.mu.Unlock()

	fp := gofeed.NewParser()
	now := time.Now()

	for _, src := range sources {
		feed, err := fp.ParseURL(src.FeedURL)
		if err != nil {
			continue
		}
		m.mu.Lock()
		for _, item := range feed.Items {
			guid := item.GUID
			if guid == "" {
				guid = item.Link
				if guid == "" {
					guid = item.Title
				}
			}
			if guid == "" {
				continue
			}
			pubAt := now
			if item.PublishedParsed != nil {
				pubAt = *item.PublishedParsed
			}
			desc := ""
			if item.Description != "" {
				desc = stripTags(item.Description)
				if len(desc) > 200 {
					desc = desc[:200]
				}
			}
			entry := FeedEntry{
				ID:          guid,
				SourceID:    src.ID,
				FeedURL:     src.FeedURL,
				Title:       item.Title,
				Link:        item.Link,
				Description: desc,
				PublishedAt: pubAt,
				FetchedAt:   now,
			}
			m.upsertEntryLocked(entry)
		}
		m.mu.Unlock()
	}

	m.CleanOld()
	m.mu.Lock()
	m.saveEntries()
	m.mu.Unlock()
}

func (m *Manager) upsertEntryLocked(e FeedEntry) {
	for i, ex := range m.entries {
		if ex.ID == e.ID && ex.SourceID == e.SourceID {
			m.entries[i].Title = e.Title
			m.entries[i].Link = e.Link
			m.entries[i].Description = e.Description
			m.entries[i].PublishedAt = e.PublishedAt
			m.entries[i].FetchedAt = e.FetchedAt
			return
		}
	}
	m.entries = append(m.entries, e)
}

func (m *Manager) CleanOld() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var kept []FeedEntry
	for _, e := range m.entries {
		if e.IsBookmark || e.PublishedAt.After(cutoff) || e.FetchedAt.After(cutoff) {
			kept = append(kept, e)
		}
	}
	m.entries = kept
}

// ─── Persistence ───

func (m *Manager) loadEntries() []FeedEntry {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "entries.json"))
	if err != nil {
		return nil
	}
	var entries []FeedEntry
	json.Unmarshal(data, &entries)
	return entries
}

func (m *Manager) saveEntries() {
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(m.dataDir, "entries.json"), data, 0644)
}

// ─── Looper ───

func (m *Manager) StartLooper(interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.FetchAll()
				m.CleanOld()
				if m.onUpdate != nil {
					m.onUpdate()
				}
			case <-m.looperStop:
				return
			}
		}
	}()
}

func (m *Manager) StopLooper() {
	close(m.looperStop)
}

// ─── Summarize ───

type summarizer interface {
	Summarize(rawData, pipeline, userInput, conversationContext string) string
}

func (m *Manager) Summarize(s summarizer, max int) string {
	if s == nil {
		return ""
	}
	m.mu.Lock()
	entries := make([]FeedEntry, len(m.entries))
	copy(entries, m.entries)
	m.mu.Unlock()

	if len(entries) == 0 {
		return "No entries to summarize."
	}

	var sb strings.Builder
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)
	count := 0
	for _, e := range entries {
		if count >= max {
			break
		}
		if e.PublishedAt.Before(cutoff) {
			continue
		}
		sourceName := ""
		for _, src := range m.sources {
			if src.ID == e.SourceID {
				sourceName = src.Title
				break
			}
		}
		sb.WriteString(fmt.Sprintf("[Source: %s]\n", sourceName))
		sb.WriteString(fmt.Sprintf("Title: %s\n", e.Title))
		sb.WriteString(fmt.Sprintf("Link: %s\n", e.Link))
		if e.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", e.Description))
		}
		sb.WriteString("\n")
		count++
	}

	prompt := fmt.Sprintf(
		"Summarize the latest RSS feed entries into a curated list. For each entry provide:\n"+
			"- Bold title\n"+
			"- One-line highlight (what makes it interesting)\n"+
			"- The URL on a separate line prefixed with →\n\n"+
			"Group by source. Be concise. Highlight the most important stories first.\n\n%s",
		sb.String(),
	)

	return s.Summarize(prompt, "rss_feeds", "Show me my RSS feeds", "")
}

// ─── Helpers ───

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
