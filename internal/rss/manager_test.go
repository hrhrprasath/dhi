package rss

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetOnUpdate(t *testing.T) {
	m := NewManager(t.TempDir())
	called := false
	m.SetOnUpdate(func() { called = true })
	if m.onUpdate == nil {
		t.Fatal("SetOnUpdate did not store the callback")
	}
	m.onUpdate()
	if !called {
		t.Error("stored callback was not callable")
	}
}

func TestFetchAll_NoSources(t *testing.T) {
	m := NewManager(t.TempDir())
	m.FetchAll()
	if got := m.EntryCount(); got != 0 {
		t.Errorf("EntryCount() after FetchAll with no sources = %d, want 0", got)
	}
}

func TestSourceCount(t *testing.T) {
	t.Run("no sources", func(t *testing.T) {
		m := NewManager(t.TempDir())
		if got := m.SourceCount(); got != 0 {
			t.Errorf("SourceCount() = %d, want 0", got)
		}
	})

	t.Run("with sources via direct init", func(t *testing.T) {
		m := &Manager{
			dataDir:    t.TempDir(),
			looperStop: make(chan struct{}),
			sources: []FeedSource{
				{ID: "src1", Title: "Source 1", FeedURL: "https://example.com/feed"},
				{ID: "src2", Title: "Source 2", FeedURL: "https://example.org/feed"},
			},
		}
		if got := m.SourceCount(); got != 2 {
			t.Errorf("SourceCount() = %d, want 2", got)
		}
	})
}

func TestListSources(t *testing.T) {
	srcs := []FeedSource{
		{ID: "a", Title: "Alpha", FeedURL: "https://a.example.com/rss"},
		{ID: "b", Title: "Beta", FeedURL: "https://b.example.com/rss"},
	}
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		sources:    srcs,
	}

	got := m.ListSources()
	if len(got) != 2 {
		t.Fatalf("ListSources() returned %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("ListSources() returned %+v, want [{a} {b}]", got)
	}
}

func TestRemoveSource(t *testing.T) {
	t.Run("non-existent returns false", func(t *testing.T) {
		m := NewManager(t.TempDir())
		if got := m.RemoveSource("nonexistent"); got != false {
			t.Errorf("RemoveSource('nonexistent') = %v, want false", got)
		}
	})

	t.Run("removes source and associated entries", func(t *testing.T) {
		m := &Manager{
			dataDir:    t.TempDir(),
			looperStop: make(chan struct{}),
			sources: []FeedSource{
				{ID: "s1", Title: "Keep"},
				{ID: "s2", Title: "Remove"},
			},
			entries: []FeedEntry{
				{ID: "e1", SourceID: "s1", Title: "Entry from keep"},
				{ID: "e2", SourceID: "s2", Title: "Entry from remove"},
				{ID: "e3", SourceID: "s1", Title: "Another from keep"},
			},
		}

		if got := m.RemoveSource("s2"); got != true {
			t.Fatalf("RemoveSource('s2') = %v, want true", got)
		}
		if got := m.SourceCount(); got != 1 {
			t.Errorf("SourceCount() after removal = %d, want 1", got)
		}
		if got := m.EntryCount(); got != 2 {
			t.Errorf("EntryCount() after removal = %d, want 2", got)
		}
	})
}

func TestListEntries(t *testing.T) {
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", SourceID: "a", Title: "Entry A1"},
			{ID: "2", SourceID: "b", Title: "Entry B1"},
			{ID: "3", SourceID: "a", Title: "Entry A2"},
		},
	}

	t.Run("all entries", func(t *testing.T) {
		all := m.ListEntries("")
		if len(all) != 3 {
			t.Errorf("ListEntries('') = %d, want 3", len(all))
		}
	})

	t.Run("filtered by source", func(t *testing.T) {
		filtered := m.ListEntries("a")
		if len(filtered) != 2 {
			t.Errorf("ListEntries('a') = %d, want 2", len(filtered))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		none := m.ListEntries("z")
		if len(none) != 0 {
			t.Errorf("ListEntries('z') = %d, want 0", len(none))
		}
	})
}

func TestEntryCount(t *testing.T) {
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1"}, {ID: "2"}, {ID: "3"},
		},
	}
	if got := m.EntryCount(); got != 3 {
		t.Errorf("EntryCount() = %d, want 3", got)
	}
}

func TestToggleBookmark(t *testing.T) {
	t.Run("toggle on and off", func(t *testing.T) {
		m := &Manager{
			dataDir:    t.TempDir(),
			looperStop: make(chan struct{}),
			entries:    []FeedEntry{{ID: "1", Title: "Test", IsBookmark: false}},
		}

		m.ToggleBookmark("1")
		if !m.entries[0].IsBookmark {
			t.Error("expected entry to be bookmarked after first toggle")
		}

		m.ToggleBookmark("1")
		if m.entries[0].IsBookmark {
			t.Error("expected entry to be unbookmarked after second toggle")
		}
	})

	t.Run("non-existent entry does not panic", func(t *testing.T) {
		m := &Manager{
			dataDir:    t.TempDir(),
			looperStop: make(chan struct{}),
			entries:    []FeedEntry{{ID: "1"}},
		}
		m.ToggleBookmark("unknown")
		if m.entries[0].IsBookmark {
			t.Error("non-existent toggle should not affect existing entries")
		}
	})
}

func TestGetBookmarks(t *testing.T) {
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", Title: "Bookmarked", IsBookmark: true},
			{ID: "2", Title: "Not bookmarked", IsBookmark: false},
			{ID: "3", Title: "Also bookmarked", IsBookmark: true},
		},
	}

	bookmarks := m.GetBookmarks()
	if len(bookmarks) != 2 {
		t.Errorf("GetBookmarks() = %d, want 2", len(bookmarks))
	}
}

func TestBookmarkCount(t *testing.T) {
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", IsBookmark: true},
			{ID: "2", IsBookmark: false},
			{ID: "3", IsBookmark: true},
		},
	}
	if got := m.BookmarkCount(); got != 2 {
		t.Errorf("BookmarkCount() = %d, want 2", got)
	}
}

func TestCleanOld(t *testing.T) {
	now := time.Now()
	recent := now
	old := now.Add(-14 * 24 * time.Hour)

	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", Title: "Recent", PublishedAt: recent, FetchedAt: recent},
			{ID: "2", Title: "Old non-bookmark", PublishedAt: old, FetchedAt: old},
			{ID: "3", Title: "Old but bookmarked", PublishedAt: old, FetchedAt: old, IsBookmark: true},
			{ID: "4", Title: "Old but recently fetched", PublishedAt: old, FetchedAt: recent},
		},
	}

	m.CleanOld()

	// Expect: entries 1 (recent), 3 (bookmarked), 4 (recently fetched) kept
	//         entry 2 removed
	if len(m.entries) != 3 {
		t.Fatalf("CleanOld left %d entries, want 3", len(m.entries))
	}
	kept := map[string]bool{}
	for _, e := range m.entries {
		kept[e.ID] = true
	}
	if !kept["1"] || !kept["3"] || !kept["4"] {
		t.Errorf("CleanOld unexpectedly removed entries. Kept: %v", kept)
	}
	if kept["2"] {
		t.Error("CleanOld should have removed old non-bookmark entry 2")
	}
}

func TestCleanOld_NoEntries(t *testing.T) {
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
	}
	m.CleanOld() // should not panic
	if len(m.entries) != 0 {
		t.Errorf("CleanOld on empty = %d, want 0", len(m.entries))
	}
}

func TestSummarize_NilSummarizer(t *testing.T) {
	m := NewManager(t.TempDir())
	if got := m.Summarize(nil, 10); got != "" {
		t.Errorf("Summarize(nil, 10) = %q, want empty string", got)
	}
}

func TestSummarize_NoEntries(t *testing.T) {
	m := NewManager(t.TempDir())
	got := m.Summarize(&dummySummarizer{}, 10)
	if got != "No entries to summarize." {
		t.Errorf("Summarize() with no entries = %q, want %q", got, "No entries to summarize.")
	}
}

type dummySummarizer struct{}

func (d *dummySummarizer) Summarize(rawData, _, _, _ string) string {
	return "summarized: " + rawData
}

func TestStripTags(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<p>Hello World</p>", "Hello World"},
		{"<div><b>Bold</b> text</div>", "Bold text"},
		{"<a href='link'>Click here</a>", "Click here"},
		{"no tags", "no tags"},
		{"", ""},
		{"<tag>", ""},
		{">>> not a tag", "not a tag"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := stripTags(tt.input); got != tt.want {
				t.Errorf("stripTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSaveAndLoadEntries(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{
		dataDir:    dir,
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", Title: "Entry 1"},
			{ID: "2", Title: "Entry 2", IsBookmark: true},
		},
	}
	m.saveEntries()

	data, err := os.ReadFile(filepath.Join(dir, "entries.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("entries.json is empty")
	}

	loaded := m.loadEntries()
	if len(loaded) != 2 {
		t.Fatalf("loadEntries() = %d entries, want 2", len(loaded))
	}
	if loaded[0].ID != "1" || loaded[1].ID != "2" {
		t.Errorf("loaded entries have wrong IDs: %+v", loaded)
	}
	if !loaded[1].IsBookmark {
		t.Error("expected second loaded entry to be bookmarked")
	}
}

func TestStartStopLooper(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	m.StartLooper(10 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	m.StopLooper()
}

// --- AddSource ---

func TestAddSource_InvalidURL(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.AddSource("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestAddSource_UnreachableURL(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.AddSource("http://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

func TestAddSource_WithValidFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Test Feed</title>
<link>http://example.com</link>
<description>A test feed</description>
<item>
<title>Item 1</title>
<link>http://example.com/item1</link>
<guid>guid-1</guid>
<description>Description 1</description>
</item>
</channel>
</rss>`))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	err := m.AddSource(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.SourceCount() != 1 {
		t.Fatalf("expected 1 source, got %d", m.SourceCount())
	}
}

func TestAddSource_DuplicateURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Test Feed</title>
<link>http://example.com</link>
<description>Test</description>
</channel>
</rss>`))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	if err := m.AddSource(srv.URL); err != nil {
		t.Fatalf("first AddSource failed: %v", err)
	}
	err := m.AddSource(srv.URL)
	if err == nil {
		t.Fatal("expected error for duplicate URL")
	}
	if !strings.Contains(err.Error(), "feed already exists") {
		t.Errorf("expected 'feed already exists', got %v", err)
	}
}

func TestAddSource_EmptyTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title></title>
<link></link>
<description>No title feed</description>
</channel>
</rss>`))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	err := m.AddSource(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sources := m.ListSources()
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Title != srv.URL {
		t.Errorf("expected title to default to URL, got %q", sources[0].Title)
	}
}

// --- FetchAll ---

func TestFetchAll_WithSourcesViaAdd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Fetch Feed</title>
<link>http://example.com</link>
<description>Test</description>
<item>
<title>Fetched Item</title>
<link>http://example.com/item</link>
<guid>fetch-guid</guid>
<description>Fetched desc</description>
</item>
</channel>
</rss>`))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	if err := m.AddSource(srv.URL); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	m.FetchAll()

	if m.EntryCount() != 1 {
		t.Fatalf("expected 1 entry after FetchAll, got %d", m.EntryCount())
	}
	entries := m.ListEntries("")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "Fetched Item" {
		t.Errorf("expected title 'Fetched Item', got %q", entries[0].Title)
	}
	if entries[0].Link != "http://example.com/item" {
		t.Errorf("expected link 'http://example.com/item', got %q", entries[0].Link)
	}
	if entries[0].Description != "Fetched desc" {
		t.Errorf("expected description 'Fetched desc', got %q", entries[0].Description)
	}
	if entries[0].PublishedAt.IsZero() {
		t.Errorf("expected PublishedAt to be non-zero, got zero")
	}
}

func TestFetchAll_IdsFromLinkAndTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Test</title>
<description>Test</description>
<item>
<title>No GUID</title>
<link>http://example.com/no-guid</link>
</item>
<item>
<title></title>
<link></link>
</item>
</channel>
</rss>`))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	if err := m.AddSource(srv.URL); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	m.FetchAll()

	if m.EntryCount() != 1 {
		t.Fatalf("expected 1 entry (only the one with link), got %d", m.EntryCount())
	}
}

func TestFetchAll_WithInvalidSourceURL(t *testing.T) {
	m := NewManager(t.TempDir())
	m.sources = []FeedSource{
		{ID: "bad", Title: "Bad Feed", FeedURL: "http://127.0.0.1:1/badfeed"},
	}
	m.FetchAll()
	if m.EntryCount() != 0 {
		t.Fatalf("expected 0 entries for bad source, got %d", m.EntryCount())
	}
}

func TestFetchAll_UpsertsExistingEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Test</title>
<description>Test</description>
<item>
<title>Updated Title</title>
<link>http://example.com/item</link>
<guid>guid-1</guid>
</item>
</channel>
</rss>`))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	if err := m.AddSource(srv.URL); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	m.FetchAll()
	if m.EntryCount() != 1 {
		t.Fatalf("expected 1 entry after first fetch, got %d", m.EntryCount())
	}

	m.FetchAll()
	if m.EntryCount() != 1 {
		t.Fatalf("expected still 1 entry after upsert, got %d", m.EntryCount())
	}
	entries := m.ListEntries("")
	if entries[0].Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", entries[0].Title)
	}
}

// --- ToggleBookmark edge cases ---

func TestToggleBookmark_AlreadyBookmarked(t *testing.T) {
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries:    []FeedEntry{{ID: "1", Title: "Test", IsBookmark: true}},
	}

	m.ToggleBookmark("1")
	if m.entries[0].IsBookmark {
		t.Error("expected entry to be unbookmarked after toggle")
	}

	m.ToggleBookmark("1")
	if !m.entries[0].IsBookmark {
		t.Error("expected entry to be bookmarked after second toggle")
	}
}

// --- CleanOld with mixed entries ---

func TestCleanOld_AllRecent(t *testing.T) {
	now := time.Now()
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", Title: "Recent 1", PublishedAt: now, FetchedAt: now},
			{ID: "2", Title: "Recent 2", PublishedAt: now, FetchedAt: now},
		},
	}
	m.CleanOld()
	if len(m.entries) != 2 {
		t.Fatalf("expected 2 entries kept, got %d", len(m.entries))
	}
}

func TestCleanOld_AllOld(t *testing.T) {
	old := time.Now().Add(-14 * 24 * time.Hour)
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", Title: "Old 1", PublishedAt: old, FetchedAt: old},
			{ID: "2", Title: "Old 2", PublishedAt: old, FetchedAt: old},
		},
	}
	m.CleanOld()
	if len(m.entries) != 0 {
		t.Fatalf("expected 0 entries kept, got %d", len(m.entries))
	}
}

func TestCleanOld_OneKeptBecauseBookmarked(t *testing.T) {
	old := time.Now().Add(-14 * 24 * time.Hour)
	m := &Manager{
		dataDir:    t.TempDir(),
		looperStop: make(chan struct{}),
		entries: []FeedEntry{
			{ID: "1", Title: "Old bookmarked", PublishedAt: old, FetchedAt: old, IsBookmark: true},
			{ID: "2", Title: "Old not bookmarked", PublishedAt: old, FetchedAt: old},
		},
	}
	m.CleanOld()
	if len(m.entries) != 1 {
		t.Fatalf("expected 1 entry kept (bookmarked), got %d", len(m.entries))
	}
	if m.entries[0].ID != "1" {
		t.Errorf("expected kept entry to be the bookmarked one, got %q", m.entries[0].ID)
	}
}

// --- syncFromOPML ---

func TestSyncFromOPML_ValidFile(t *testing.T) {
	dir := t.TempDir()
	opmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<head><title>Test Feeds</title></head>
<body>
<outline text="Tech" title="Tech">
<outline type="rss" text="Feed 1" title="Feed 1" xmlUrl="http://example.com/feed1" htmlUrl="http://example.com"/>
</outline>
</body>
</opml>`
	opmlPath := filepath.Join(dir, "rss", "feed.opml")
	if err := os.MkdirAll(filepath.Dir(opmlPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opmlPath, []byte(opmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DHI_RSS_OPML_PATH", opmlPath)
	m := NewManager(dir)

	sources := m.ListSources()
	if len(sources) != 1 {
		t.Fatalf("expected 1 source from OPML, got %d", len(sources))
	}
	if sources[0].FeedURL != "http://example.com/feed1" {
		t.Errorf("expected feed URL 'http://example.com/feed1', got %q", sources[0].FeedURL)
	}
}

func TestSyncFromOPML_MissingFile(t *testing.T) {
	m := NewManager(t.TempDir())
	sources := m.ListSources()
	if len(sources) != 0 {
		t.Errorf("expected 0 sources when OPML missing, got %d", len(sources))
	}
}

func TestSyncFromOPML_NoOutlines(t *testing.T) {
	dir := t.TempDir()
	opmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<head><title>Empty</title></head>
<body></body>
</opml>`
	opmlPath := filepath.Join(dir, "rss", "feed.opml")
	if err := os.MkdirAll(filepath.Dir(opmlPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opmlPath, []byte(opmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DHI_RSS_OPML_PATH", opmlPath)
	m := NewManager(dir)

	sources := m.ListSources()
	if len(sources) != 0 {
		t.Errorf("expected 0 sources from empty OPML, got %d", len(sources))
	}
}
