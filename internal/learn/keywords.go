package learn

import (
	"fmt"
	"sort"
	"strings"
)

func extractWords(input string) []string {
	input = strings.ToLower(input)
	fields := strings.Fields(input)

	var words []string
	seen := make(map[string]bool)
	for _, f := range fields {
		f = strings.Trim(f, ".,!?;:'\"()[]{}/\\-")
		if len(f) < 3 {
			continue
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		words = append(words, f)
	}
	return words
}

func (e *Engine) ensureKeywordStat(word, pipeline string) *KeywordStat {
	pips := e.data.KeywordPipelines[word]
	if pips == nil {
		pips = make(map[string]*KeywordStat)
		e.data.KeywordPipelines[word] = pips
	}
	ks := pips[pipeline]
	if ks == nil {
		ks = &KeywordStat{Pipeline: pipeline}
		pips[pipeline] = ks
	}
	return ks
}

func (e *Engine) recordWordOutcome(input, pipeline string, success bool) {
	words := extractWords(input)
	if len(words) == 0 {
		return
	}
	for _, w := range words {
		ks := e.ensureKeywordStat(w, pipeline)
		ks.Count++
		if success {
			ks.Success++
		}
	}
}

func (e *Engine) BuildKeywordBlock(input string, pipelineKeywords map[string][]string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	words := extractWords(input)
	if len(words) == 0 && len(pipelineKeywords) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nKEYWORD-PIPELINE INSIGHTS:\n")

	if len(pipelineKeywords) > 0 {
		sb.WriteString("PIPELINE HINTS (from config):\n")
		for pip, kws := range pipelineKeywords {
			fmt.Fprintf(&sb, "  %s: %s\n", strings.ToUpper(pip), strings.Join(kws, ", "))
		}
	}

	type assoc struct {
		word     string
		pipeline string
		score    float64
		count    int
	}
	var learned []assoc

	for _, w := range words {
		pips := e.data.KeywordPipelines[w]
		if len(pips) == 0 {
			continue
		}
		for _, s := range pips {
			if s.Count >= 1 {
				pct := float64(s.Success) / float64(s.Count) * 100
				learned = append(learned, assoc{
					word:     w,
					pipeline: s.Pipeline,
					score:    pct,
					count:    s.Count,
				})
			}
		}
	}

	if len(learned) > 0 {
		sort.Slice(learned, func(i, j int) bool {
			if learned[i].word != learned[j].word {
				return learned[i].word < learned[j].word
			}
			return learned[i].score > learned[j].score
		})

		sb.WriteString("LEARNED (from feedback):\n")
		for _, a := range learned {
			fmt.Fprintf(&sb, "  \"%s\" → %s (%.0f%% success, %d uses)\n",
				a.word, strings.ToUpper(a.pipeline), a.score, a.count)
		}
	}

	return sb.String()
}
