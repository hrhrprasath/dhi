package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
)

// OSC 8 hyperlink escape sequences.
const (
	osc8Start = "\x1b]8;;"
	osc8Sep   = "\x1b\\"
	osc8End   = "\x1b]8;;\x1b\\"
)

func hyperlink(url, text string) string {
	return osc8Start + url + osc8Sep + text + osc8End
}

// extractURLs extracts all http/https URLs from a string.
func extractURLs(s string) []string {
	re := regexp.MustCompile(`https?://[^\s<>"']+`)
	matches := re.FindAllString(s, -1)
	seen := make(map[string]bool)
	var uniq []string
	for _, u := range matches {
		u = strings.TrimRightFunc(u, unicode.IsPunct)
		if !seen[u] {
			seen[u] = true
			uniq = append(uniq, u)
		}
	}
	return uniq
}

func fmtURLs(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n")
	for i, u := range urls {
		fmt.Fprintf(&sb, "  %d. %s\n", i+1, u)
	}
	return sb.String()
}

var (
	reURL       = regexp.MustCompile(`https?://[^\s<>"']+`)
	reOpenLink  = regexp.MustCompile(`(?i)^(?:open|visit|go\s+to|launch)\s+(?:link\s+)?#?(\d+)$`)
	reOpenExact = regexp.MustCompile(`(?i)^(?:open|visit|go\s+to|launch)\s+(.+)$`)
)

func parseOpenLink(input string, urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	input = strings.TrimSpace(input)

	if m := reOpenLink.FindStringSubmatch(input); m != nil {
		idx := parseInt(m[1]) - 1
		if idx >= 0 && idx < len(urls) {
			return urls[idx]
		}
		return ""
	}

	if m := reOpenExact.FindStringSubmatch(input); m != nil {
		rest := strings.TrimSpace(m[1])
		if reURL.MatchString(rest) {
			return rest
		}
		for _, u := range urls {
			if strings.Contains(strings.ToLower(u), strings.ToLower(rest)) {
				return u
			}
		}
	}

	return ""
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func dhiDarkStyle() ansi.StyleConfig {
	accent := "#FFA500"
	dim := "#888888"
	green := "#00FF00"
	border := "#444444"

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       strPtr("#F0F0F0"),
			},
			Margin: uintPtr(2),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  strPtr(dim),
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: strPtr("┃ "),
		},
		Paragraph: ansi.StyleBlock{},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       strPtr(accent),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "◆ ",
				Suffix:          " ",
				Color:           strPtr("#FFFFFF"),
				BackgroundColor: strPtr(accent),
				Bold:            boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "▪ ",
				Color:  strPtr(accent),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "▸ ",
				Color:  strPtr(accent),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "▹ ",
				Color:  strPtr(dim),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "• ",
				Color:  strPtr(dim),
				Italic: boolPtr(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "– ",
				Color:  strPtr(dim),
				Italic: boolPtr(true),
				Faint:  boolPtr(true),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  strPtr(border),
			Format: "\n" + strings.Repeat("━", 40) + "\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		ImageText: ansi.StylePrimitive{
			Format: "🖼 {{.text}}",
		},
		Link: ansi.StylePrimitive{
			Color:     strPtr(accent),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: strPtr("#FFFFFF"),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     strPtr(dim),
			Underline: boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           strPtr(green),
				BackgroundColor: strPtr("#1A1A1A"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           strPtr("#E0E0E0"),
					BackgroundColor: strPtr("#121212"),
				},
				Margin: uintPtr(2),
			},
		},
		Table: ansi.StyleTable{
			CenterSeparator: strPtr("┼"),
			ColumnSeparator: strPtr("│"),
			RowSeparator:    strPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n  → ",
		},
	}
}

func dhiLightStyle() ansi.StyleConfig {
	accent := "#FFA500"
	dim := "#666666"
	border := "#CCCCCC"

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       strPtr("#333333"),
			},
			Margin: uintPtr(2),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  strPtr(dim),
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(1),
			IndentToken: strPtr("┃ "),
		},
		Paragraph: ansi.StyleBlock{},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       strPtr(accent),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          "◆ ",
				Suffix:          " ",
				Color:           strPtr("#FFFFFF"),
				BackgroundColor: strPtr(accent),
				Bold:            boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "▪ ",
				Color:  strPtr(accent),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "▸ ",
				Color:  strPtr(accent),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "▹ ",
				Color:  strPtr(dim),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "• ",
				Color:  strPtr(dim),
				Italic: boolPtr(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "– ",
				Color:  strPtr(dim),
				Italic: boolPtr(true),
				Faint:  boolPtr(true),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  strPtr(border),
			Format: "\n" + strings.Repeat("━", 40) + "\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		ImageText: ansi.StylePrimitive{
			Format: "🖼 {{.text}}",
		},
		Link: ansi.StylePrimitive{
			Color:     strPtr(accent),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: strPtr("#333333"),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     strPtr(dim),
			Underline: boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           strPtr("#D2691E"),
				BackgroundColor: strPtr("#F0F0F0"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           strPtr("#333333"),
					BackgroundColor: strPtr("#F5F5F5"),
				},
				Margin: uintPtr(2),
			},
		},
		Table: ansi.StyleTable{
			CenterSeparator: strPtr("┼"),
			ColumnSeparator: strPtr("│"),
			RowSeparator:    strPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n  → ",
		},
	}
}

func uintPtr(u uint) *uint { return &u }

func renderMarkdown(s string, width int, isDark bool) string {
	if s == "" {
		return ""
	}
	var cfg ansi.StyleConfig
	if isDark {
		cfg = dhiDarkStyle()
	} else {
		cfg = dhiLightStyle()
	}
	rw := width - 3
	if rw < 10 {
		rw = 10
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(cfg),
		glamour.WithWordWrap(rw),
	)
	if err != nil {
		return s
	}
	out, err := r.Render(s)
	if err != nil {
		return s
	}
	return out
}

func cleanResponse(response, userInput string) string {
	idx := strings.Index(response, "[USER]")
	if idx < 0 {
		idx = strings.Index(response, "---BEGIN INPUT---")
	}
	if idx < 0 {
		idx = strings.Index(response, "User says:")
	}
	if idx < 0 && len(userInput) > 0 {
		idx = strings.Index(response, "User: "+userInput)
	}
	if idx >= 0 {
		response = strings.TrimSpace(response[:idx])
	}
	return response
}

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'.,;:!?`)
	parts := strings.Fields(s)
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, " ")
}
