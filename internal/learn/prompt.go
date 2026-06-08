package learn

import (
	"fmt"
	"sort"
	"strings"
)

func (e *Engine) BuildWeightLine(pipelineName string, baseWeight float64) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strings.ToLower(pipelineName)
	adjusted := e.computeWeightLocked(name)

	if adjusted == 0 {
		return fmt.Sprintf("weight: %.1f", baseWeight)
	}

	trend := e.computeTrendLocked(name)

	accuracy, calls := e.accuracyLocked(name)
	if calls < 3 {
		return fmt.Sprintf("weight: %.1f | learned: %.2f | trend: %s",
			baseWeight, adjusted, trend)
	}

	return fmt.Sprintf("weight: %.1f | learned: %.2f | accuracy: %.0f%% (%d) | trend: %s",
		baseWeight, adjusted, accuracy*100, calls, trend)
}

func (e *Engine) BuildCalibrationBlock() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	type entry struct {
		name    string
		accuracy float64
		calls   int
		trend   string
	}

	var entries []entry
	for name, stats := range e.data.PipelineStats {
		if stats.TotalCalls < 3 {
			continue
		}
		acc := float64(stats.SuccessCount) / float64(stats.TotalCalls)
		entries = append(entries, entry{
			name:     strings.ToUpper(name),
			accuracy: acc,
			calls:    stats.TotalCalls,
			trend:    e.computeTrendLocked(name),
		})
	}

	if len(entries) == 0 {
		return ""
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].accuracy > entries[j].accuracy
	})

	var sb strings.Builder
	sb.WriteString("\nCALIBRATION DATA:\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- EXECUTE_%s: %.0f%% accuracy (%d/%d calls) | trend: %s\n",
			e.name, e.accuracy*100, int(e.accuracy*float64(e.calls)), e.calls, e.trend))
	}

	return sb.String()
}

func (e *Engine) computeTrendLocked(pipeline string) string {
	name := strings.ToLower(pipeline)
	log := e.data.FeedbackLog
	if len(log) < 5 {
		return "→"
	}

	var recent, older int
	var recentSuccess, olderSuccess int
	for i := len(log) - 1; i >= 0; i-- {
		if strings.EqualFold(log[i].PipelineUsed, name) {
			if recent < 10 {
				if log[i].Success {
					recentSuccess++
				}
				recent++
			} else if older < 10 {
				if log[i].Success {
					olderSuccess++
				}
				older++
			}
			if recent >= 10 && older >= 10 {
				break
			}
		}
	}

	if recent < 3 || older < 3 {
		return "→"
	}

	recentRate := float64(recentSuccess) / float64(recent)
	olderRate := float64(olderSuccess) / float64(older)

	if recentRate > olderRate+0.1 {
		return "↑"
	} else if recentRate < olderRate-0.1 {
		return "↓"
	}
	return "→"
}

func (e *Engine) accuracyLocked(pipeline string) (float64, int) {
	name := strings.ToLower(pipeline)
	stats := e.data.PipelineStats[name]
	if stats == nil || stats.TotalCalls == 0 {
		return 0, 0
	}
	return float64(stats.SuccessCount) / float64(stats.TotalCalls), stats.TotalCalls
}

func (e *Engine) HasSufficientData(pipeline string, minSamples int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strings.ToLower(pipeline)
	stats := e.data.PipelineStats[name]
	return stats != nil && stats.TotalCalls >= minSamples
}
