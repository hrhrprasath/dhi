package tui

import (
	"regexp"
	"strings"
)

func stripMarkdown(s string) string {
	s = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(s, "")
	s = regexp.MustCompile("`[^`]*`").ReplaceAllString(s, "")
	s = regexp.MustCompile("\\[([^\\]]*)\\]\\([^)]*\\)").ReplaceAllString(s, "$1")
	s = regexp.MustCompile("!\\[([^\\]]*)\\]\\([^)]*\\)").ReplaceAllString(s, "$1")
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
