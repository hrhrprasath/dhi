package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
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

// linkify finds bare URLs and wraps them in clickable OSC 8 hyperlinks.
func linkify(s string) string {
	re := regexp.MustCompile(`https?://[^\s<>"']+`)
	return re.ReplaceAllStringFunc(s, func(url string) string {
		display := url
		if len(display) > 80 {
			display = display[:77] + "..."
		}
		return hyperlink(url, display)
	})
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
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, u))
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

func stripMarkdown(s string) string {
	s = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(s, "")
	s = regexp.MustCompile("`[^`]*`").ReplaceAllString(s, "")
	s = regexp.MustCompile("\\[([^\\]]*)\\]\\(([^)]+)\\)").ReplaceAllString(s, "$1 ($2)")
	s = regexp.MustCompile("!\\[([^\\]]*)\\]\\(([^)]+)\\)").ReplaceAllString(s, "$1 ($2)")
	s = regexp.MustCompile("\\*\\*(.*?)\\*\\*").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("__(.*?)__").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("\\*(.*?)\\*").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("_(.*?)_").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("~~(.*?)~~").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("^#{1,6}\\s+").ReplaceAllString(s, "")
	s = regexp.MustCompile("^>\\s?").ReplaceAllString(s, "")
	s = regexp.MustCompile("^[-*+]\\s+").ReplaceAllString(s, "")
	s = regexp.MustCompile("^\\d+\\.\\s+").ReplaceAllString(s, "")
	s = regexp.MustCompile("^[-*_]{3,}$").ReplaceAllString(s, "")
	s = regexp.MustCompile("\\n{3,}").ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
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
