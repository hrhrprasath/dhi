package tui

import (
	"reflect"
	"testing"
)

func TestStripMarkdown_CodeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes fenced code block",
			input: "before\n```\ncode block\n```\nafter",
			want:  "before\n\nafter",
		},
		{
			name:  "removes fenced code block with language",
			input: "text\n```go\npackage main\n```\nmore",
			want:  "text\n\nmore",
		},
		{
			name:  "two code blocks",
			input: "```a\n1\n```\nmid\n```b\n2\n```",
			want:  "mid",
		},
		{
			name:  "no code block",
			input: "hello world",
			want:  "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_InlineCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes inline code",
			input: "use `fmt.Println()` to print",
			want:  "use  to print",
		},
		{
			name:  "multiple inline code",
			input: "`a` and `b`",
			want:  "and",
		},
		{
			name:  "inline code at start",
			input: "`code` here",
			want:  "here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_CodeBlocksBeforeInline(t *testing.T) {
	// code blocks are stripped before inline code,
	// so triple backticks won't be caught by inline.
	input := "```\ncode\n```\nand `inline`"
	want := "and"
	got := stripMarkdown(input)
	if got != want {
		t.Errorf("stripMarkdown(%q) = %q, want %q", input, got, want)
	}
}

func TestStripMarkdown_Links(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "converts markdown link",
			input: "[Google](https://google.com)",
			want:  "Google (https://google.com)",
		},
		{
			name:  "link with text",
			input: "click [here](https://example.com) now",
			want:  "click here (https://example.com) now",
		},
		{
			name:  "multiple links",
			input: "[a](x) and [b](y)",
			want:  "a (x) and b (y)",
		},
		{
			name:  "no links",
			input: "plain text",
			want:  "plain text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_Images(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "converts image",
			input: "![alt](img.png)",
			want:  "alt (img.png)",
		},
		{
			name:  "image in text",
			input: "see ![logo](logo.png) here",
			want:  "see logo (logo.png) here",
		},
		{
			name:  "multiple images",
			input: "![a](1.png) ![b](2.png)",
			want:  "a (1.png) b (2.png)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_Bold(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes bold **",
			input: "**bold text**",
			want:  "bold text",
		},
		{
			name:  "bold in sentence",
			input: "this is **very** important",
			want:  "this is very important",
		},
		{
			name:  "multiple bold",
			input: "**a** and **b**",
			want:  "a and b",
		},
		{
			name:  "removes bold __",
			input: "__bold__ text",
			want:  "bold text",
		},
		{
			name:  "no bold",
			input: "plain",
			want:  "plain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_Italic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes italic *",
			input: "*italic text*",
			want:  "italic text",
		},
		{
			name:  "italic in sentence",
			input: "that is *great*",
			want:  "that is great",
		},
		{
			name:  "removes italic _",
			input: "_italic_ text",
			want:  "italic text",
		},
		{
			name:  "multiple italic",
			input: "*a* and _b_",
			want:  "a and b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_Strikethrough(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes strikethrough",
			input: "~~strikethrough~~",
			want:  "strikethrough",
		},
		{
			name:  "strikethrough in sentence",
			input: "this is ~~wrong~~ corrected",
			want:  "this is wrong corrected",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_Headings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes h1",
			input: "# Title",
			want:  "Title",
		},
		{
			name:  "removes h2",
			input: "## Section",
			want:  "Section",
		},
		{
			name:  "removes h3",
			input: "### Subsection",
			want:  "Subsection",
		},
		{
			name:  "removes h6",
			input: "###### smallest",
			want:  "smallest",
		},
		{
			name:  "no heading",
			input: "not a # heading",
			want:  "not a # heading",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_Blockquotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes blockquote with space",
			input: "> quoted text",
			want:  "quoted text",
		},
		{
			name:  "removes blockquote without space",
			input: ">quoted text",
			want:  "quoted text",
		},
		{
			name:  "multi-line blockquotes first line only",
			input: "> line1\n> line2",
			want:  "line1\n> line2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_UnorderedLists(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "dash list item",
			input: "- item",
			want:  "item",
		},
		{
			name:  "asterisk list item",
			input: "* item",
			want:  "item",
		},
		{
			name:  "plus list item",
			input: "+ item",
			want:  "item",
		},
		{
			name:  "not a list item",
			input: "text - not a list",
			want:  "text - not a list",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_OrderedLists(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "numbered list item",
			input: "1. item",
			want:  "item",
		},
		{
			name:  "multi-digit list item",
			input: "42. answer",
			want:  "answer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_HorizontalRules(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "dash hr",
			input: "---",
			want:  "",
		},
		{
			name:  "asterisk hr partially consumed by italic",
			input: "***",
			want:  "*",
		},
		{
			name:  "underscore hr partially consumed by italic",
			input: "___",
			want:  "_",
		},
		{
			name:  "long dash hr",
			input: "-------",
			want:  "",
		},
		{
			name:  "hr only as entire string",
			input: "---\ntext after",
			want:  "---\ntext after",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_CollapsesNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "collapses triple newlines to double",
			input: "a\n\n\nb",
			want:  "a\n\nb",
		},
		{
			name:  "collapses many newlines",
			input: "a\n\n\n\n\nb",
			want:  "a\n\nb",
		},
		{
			name:  "preserves double newline",
			input: "a\n\nb",
			want:  "a\n\nb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_TrimsSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "trims leading whitespace",
			input: "  hello",
			want:  "hello",
		},
		{
			name:  "trims trailing whitespace",
			input: "hello  ",
			want:  "hello",
		},
		{
			name:  "trims both sides",
			input: "  hello world  ",
			want:  "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_CombinedFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bold and italic nested",
			input: "**bold and *italic***",
			want:  "bold and italic",
		},
		{
			name:  "heading with bold",
			input: "# **Title**",
			want:  "Title",
		},
		{
			name:  "link inside bold",
			input: "**[text](url)**",
			want:  "text (url)",
		},
		{
			name:  "code block and link",
			input: "```\ncode\n```\nand [link](url)",
			want:  "and link (url)",
		},
		{
			name:  "full markdown document",
			input: "# Header\n\n**bold** and *italic*\n\n> quote\n\n- list\n\n`code`\n\n[link](url)\n\n---\n\n![img](pic.png)",
			want:  "Header\n\nbold and italic\n\n> quote\n\n- list\n\nlink (url)\n\n---\n\nimg (pic.png)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripMarkdown_EmptyString(t *testing.T) {
	got := stripMarkdown("")
	want := ""
	if got != want {
		t.Errorf("stripMarkdown('') = %q, want %q", got, want)
	}
}

func TestStripMarkdown_PlainTextUnchanged(t *testing.T) {
	input := "hello world, this is plain text"
	got := stripMarkdown(input)
	if got != input {
		t.Errorf("stripMarkdown(%q) = %q, want %q", input, got, input)
	}
}

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

func TestLinkify(t *testing.T) {
	t.Run("wraps url", func(t *testing.T) {
		got := linkify("visit https://example.com")
		want := "visit \x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\"
		if got != want {
			t.Errorf("linkify() = %q, want %q", got, want)
		}
	})
	t.Run("no url unchanged", func(t *testing.T) {
		got := linkify("plain text")
		if got != "plain text" {
			t.Errorf("linkify('plain text') = %q, want 'plain text'", got)
		}
	})
	t.Run("empty string", func(t *testing.T) {
		got := linkify("")
		if got != "" {
			t.Errorf("linkify('') = %q, want ''", got)
		}
	})
}

func TestLinkify_TruncatesLongURL(t *testing.T) {
	longURL := "https://example.com/" + "abcdefghijklmnopqrstuvwxyz0123456789/abcdefghijklmnopqrstuvwxyz0123456789/abcdefghij"
	got := linkify(longURL)
	// The display part should be truncated to 77 chars + "..."
	if len(got) <= len(longURL) {
		t.Errorf("linkify should produce hyperlink wrapping for long URL, got len=%d", len(got))
	}
	if !contains(got, "...") {
		t.Errorf("linkify should truncate long URL display with '...', got: %s", got)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
