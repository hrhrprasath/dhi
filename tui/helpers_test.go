package tui

import (
	"reflect"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "extracts http url",
			input: "visit http://example.com",
			want:  []string{"http://example.com"},
		},
		{
			name:  "extracts https url",
			input: "visit https://google.com",
			want:  []string{"https://google.com"},
		},
		{
			name:  "multiple urls",
			input: "https://a.com and https://b.com",
			want:  []string{"https://a.com", "https://b.com"},
		},
		{
			name:  "deduplicates urls",
			input: "https://a.com and https://a.com",
			want:  []string{"https://a.com"},
		},
		{
			name:  "strips trailing punctuation",
			input: "visit https://example.com.",
			want:  []string{"https://example.com"},
		},
		{
			name:  "strips trailing paren",
			input: "see (https://example.com)",
			want:  []string{"https://example.com"},
		},
		{
			name:  "url with path",
			input: "https://example.com/path/to/page",
			want:  []string{"https://example.com/path/to/page"},
		},
		{
			name:  "url with query params",
			input: "https://example.com?q=test&x=1",
			want:  []string{"https://example.com?q=test&x=1"},
		},
		{
			name:  "no urls",
			input: "plain text without any urls",
			want:  nil,
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractURLs(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractURLs(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFmtURLs(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "single url",
			input: []string{"https://example.com"},
			want:  "\n\n  1. https://example.com\n",
		},
		{
			name:  "multiple urls",
			input: []string{"https://a.com", "https://b.com"},
			want:  "\n\n  1. https://a.com\n  2. https://b.com\n",
		},
		{
			name:  "nil slice",
			input: nil,
			want:  "",
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtURLs(tt.input)
			if got != tt.want {
				t.Errorf("fmtURLs(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		input    string
		want     string
	}{
		{
			name:     "removes content after [USER]",
			response: "Hello there\n[USER] some stuff",
			input:    "",
			want:     "Hello there",
		},
		{
			name:     "removes content after ---BEGIN INPUT---",
			response: "Summary text\n---BEGIN INPUT---\nuser data",
			input:    "",
			want:     "Summary text",
		},
		{
			name:     "removes content after User says:",
			response: "Response here\nUser says: something",
			input:    "",
			want:     "Response here",
		},
		{
			name:     "removes content after User: <input>",
			response: "The answer is 42\nUser: what is the answer?",
			input:    "what is the answer?",
			want:     "The answer is 42",
		},
		{
			name:     "no marker returns original",
			response: "plain response text",
			input:    "",
			want:     "plain response text",
		},
		{
			name:     "[USER] at start",
			response: "[USER] ignore this",
			input:    "",
			want:     "",
		},
		{
			name:     "trims whitespace after cut",
			response: "hello  \n[USER]",
			input:    "",
			want:     "hello",
		},
		{
			name:     "empty response",
			response: "",
			input:    "",
			want:     "",
		},
		{
			name:     "first marker wins",
			response: "good\n[USER]\n---BEGIN INPUT---\nbad",
			input:    "",
			want:     "good",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanResponse(tt.response, tt.input)
			if got != tt.want {
				t.Errorf("cleanResponse(%q, %q) = %q, want %q", tt.response, tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanTitle_RemovesQuotes(t *testing.T) {
	got := cleanTitle(`"hello world"`)
	want := "hello world"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_RemovesNewlines(t *testing.T) {
	got := cleanTitle("hello\nworld")
	want := "hello world"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_RemovesTrailingPunctuation(t *testing.T) {
	got := cleanTitle("hello world!")
	want := "hello world"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_RemovesVariousPunctuation(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'hello world'", "hello world"},
		{"hello world.", "hello world"},
		{"hello world,", "hello world"},
		{"hello world;", "hello world"},
		{"hello world:", "hello world"},
		{"hello world?", "hello world"},
		{`"hello world"?`, "hello world"},
		{"hello world...", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanTitle(tt.input)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestCleanTitle_HandlesEmptyString(t *testing.T) {
	got := cleanTitle("")
	want := ""
	if got != want {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestCleanTitle_ValidTitleReturnsUnchanged(t *testing.T) {
	got := cleanTitle("hello world")
	want := "hello world"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_TrimsToFourWords(t *testing.T) {
	got := cleanTitle("one two three four five six")
	want := "one two three four"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_ThreeWordsPreserved(t *testing.T) {
	got := cleanTitle("one two three")
	want := "one two three"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_TrimsWhitespace(t *testing.T) {
	got := cleanTitle("  hello world  ")
	want := "hello world"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_RemovesTitlePrefix(t *testing.T) {
	got := cleanTitle("Title: hello world")
	want := "Title: hello world"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"simple", "hello world", "hello world"},
		{"single word", "hello", "hello"},
		{"with quotes", `"test title"`, "test title"},
		{"with single quotes", "'test title'", "test title"},
		{"trailing period", "hello world.", "hello world"},
		{"trailing exclamation", "hello!", "hello"},
		{"trailing question", "what is this?", "what is this"},
		{"trailing comma and period", "hello, world.", "hello, world"},
		{"tab separated", "hello\tworld", "hello world"},
		{"newlines and quotes", "\"hello\nworld\"\n", "hello world"},
		{"multiple trailing punct", "hello?!", "hello"},
		{"five words truncates", "a b c d e", "a b c d"},
		{"six words truncates", "a b c d e f", "a b c d"},
		{"mix of punct and spaces", "  \"hello, world!\"  ", "hello, world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanTitle(tt.input)
			if got != tt.want {
				t.Errorf("cleanTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanTitle_WithTitlePrefix(t *testing.T) {
	got := cleanTitle("Title: Hello World")
	want := "Title: Hello World"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCleanTitle_PunctuationInsideWordPreserved(t *testing.T) {
	got := cleanTitle("don't stop")
	want := "don't stop"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"single digit", "5", 5},
		{"multi digit", "42", 42},
		{"leading zeros", "007", 7},
		{"non-numeric", "abc", 0},
		{"mixed chars", "12a34", 0},
		{"empty string", "", 0},
		{"zero", "0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt(tt.input)
			if got != tt.want {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseOpenLink(t *testing.T) {
	tests := []struct {
		name  string
		input string
		urls  []string
		want  string
	}{
		{
			name:  "open by number",
			input: "open 1",
			urls:  []string{"https://example.com", "https://other.com"},
			want:  "https://example.com",
		},
		{
			name:  "open second link",
			input: "open 2",
			urls:  []string{"https://a.com", "https://b.com"},
			want:  "https://b.com",
		},
		{
			name:  "open with hash",
			input: "open #1",
			urls:  []string{"https://example.com"},
			want:  "https://example.com",
		},
		{
			name:  "visit by number",
			input: "visit 2",
			urls:  []string{"https://a.com", "https://b.com"},
			want:  "https://b.com",
		},
		{
			name:  "go to by number",
			input: "go to 3",
			urls:  []string{"a", "b", "c"},
			want:  "c",
		},
		{
			name:  "launch link by number",
			input: "launch link #1",
			urls:  []string{"https://example.com"},
			want:  "https://example.com",
		},
		{
			name:  "index out of bounds (too high)",
			input: "open 999",
			urls:  []string{"https://a.com"},
			want:  "",
		},
		{
			name:  "index out of bounds (zero)",
			input: "open 0",
			urls:  []string{"https://a.com"},
			want:  "",
		},
		{
			name:  "empty urls returns empty",
			input: "open 1",
			urls:  []string{},
			want:  "",
		},
		{
			name:  "exact url match",
			input: "open https://example.com",
			urls:  []string{"https://other.com"},
			want:  "https://example.com",
		},
		{
			name:  "fuzzy match by substring",
			input: "open example",
			urls:  []string{"https://example.com", "https://other.com"},
			want:  "https://example.com",
		},
		{
			name:  "fuzzy match case insensitive",
			input: "open EXAMPLE",
			urls:  []string{"https://example.com"},
			want:  "https://example.com",
		},
		{
			name:  "no match returns empty",
			input: "open nothing",
			urls:  []string{"https://a.com"},
			want:  "",
		},
		{
			name:  "go to specific url",
			input: "go to https://specific.com/path",
			urls:  []string{"https://a.com"},
			want:  "https://specific.com/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOpenLink(tt.input, tt.urls)
			if got != tt.want {
				t.Errorf("parseOpenLink(%q, %v) = %q, want %q", tt.input, tt.urls, got, tt.want)
			}
		})
	}
}

func TestHyperlink(t *testing.T) {
	got := hyperlink("https://example.com", "click here")
	want := "\x1b]8;;https://example.com\x1b\\click here\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("hyperlink() = %q, want %q", got, want)
	}
}
