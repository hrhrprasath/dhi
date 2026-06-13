package learn

import (
	"math"
	"strings"
	"time"
)

func (e *Engine) ensureStats(pipeline string) *PipelineStats {
	name := strings.ToLower(pipeline)
	if s, ok := e.data.PipelineStats[name]; ok {
		return s
	}
	s := &PipelineStats{Name: name}
	e.data.PipelineStats[name] = s
	return s
}

func (e *Engine) RecordOutcome(input, predictedEvent, pipelineUsed string, confidence float64, success bool, wasConversation bool, errMsg string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry := FeedbackEntry{
		Input:           input,
		PredictedEvent:  predictedEvent,
		Confidence:      confidence,
		PipelineUsed:    pipelineUsed,
		Success:         success,
		WasConversation: wasConversation,
		ErrorMsg:        errMsg,
		Timestamp:       time.Now(),
	}

	e.data.FeedbackLog = append(e.data.FeedbackLog, entry)
	if len(e.data.FeedbackLog) > maxFeedbackLog {
		e.data.FeedbackLog = e.data.FeedbackLog[len(e.data.FeedbackLog)-maxFeedbackLog:]
	}

	if pipelineUsed != "" && !wasConversation {
		stats := e.ensureStats(pipelineUsed)
		stats.TotalCalls++
		stats.TotalConfidence += confidence
		stats.LastUsed = time.Now()
		if success {
			stats.SuccessCount++
		} else {
			stats.FailCount++
		}
		e.updateCalibrationLocked(pipelineUsed, confidence, success)
		e.recordWordOutcome(input, pipelineUsed, success)
	}

	if predictedEvent != "" && pipelineUsed != "" &&
		!wasConversation && !strings.EqualFold(predictedEvent, pipelineUsed) {
		predictedPipeline := strings.TrimPrefix(predictedEvent, "EXECUTE_")
		predStats := e.ensureStats(predictedPipeline)
		predStats.TotalCalls++
		predStats.FailCount++
		predStats.TotalConfidence += confidence
		predStats.LastUsed = time.Now()
		e.updateCalibrationLocked(predictedPipeline, confidence, false)
		e.recordWordOutcome(input, predictedPipeline, false)
	}

	e.save()
}

func (e *Engine) RecordConversation(input string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry := FeedbackEntry{
		Input:           input,
		PredictedEvent:  "CONVERSATION",
		WasConversation: true,
		Success:         true,
		Timestamp:       time.Now(),
	}

	e.data.FeedbackLog = append(e.data.FeedbackLog, entry)
	if len(e.data.FeedbackLog) > maxFeedbackLog {
		e.data.FeedbackLog = e.data.FeedbackLog[len(e.data.FeedbackLog)-maxFeedbackLog:]
	}

	e.save()
}

func (e *Engine) RecordRating(input string, positive bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := len(e.data.FeedbackLog) - 1; i >= 0; i-- {
		if e.data.FeedbackLog[i].Input == input {
			oldRated := e.data.FeedbackLog[i].Rated
			if positive {
				e.data.FeedbackLog[i].Rated = 1
			} else {
				e.data.FeedbackLog[i].Rated = -1
			}

			pipeline := e.data.FeedbackLog[i].PipelineUsed
			if pipeline != "" && !e.data.FeedbackLog[i].WasConversation {
				stats := e.ensureStats(pipeline)
				if oldRated == 0 {
					stats.TotalCalls++
				}
				if positive {
					stats.SuccessCount++
					if oldRated == -1 {
						stats.FailCount--
					}
				} else {
					stats.FailCount++
					if oldRated == 1 {
						stats.SuccessCount--
					}
				}
				e.recordWordOutcome(input, pipeline, positive)
			}
			break
		}
	}

	e.save()
}

func (e *Engine) updateCalibrationLocked(pipeline string, confidence float64, success bool) {
	name := strings.ToLower(pipeline)
	buckets := e.data.Calibration[name]
	if buckets == nil {
		buckets = defaultBuckets
	}

	idx := bucketIndex(confidence, buckets)
	if idx < 0 {
		return
	}

	buckets[idx].Count++
	if success {
		buckets[idx].Success++
	}
	e.data.Calibration[name] = buckets
}

func (e *Engine) GetWeight(pipeline string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.computeWeightLocked(pipeline)
}

func (e *Engine) GetWeights(pipelines []string) map[string]float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make(map[string]float64, len(pipelines))
	for _, name := range pipelines {
		out[name] = e.computeWeightLocked(name)
	}
	return out
}

func (e *Engine) computeWeightLocked(pipeline string) float64 {
	name := strings.ToLower(pipeline)
	stats := e.data.PipelineStats[name]
	if stats == nil || stats.TotalCalls < 3 {
		return 0
	}

	successRate := float64(stats.SuccessCount) / float64(stats.TotalCalls)
	if successRate == 0 {
		return 0.1
	}

	var totalRate float64
	var count int
	for _, s := range e.data.PipelineStats {
		if s.TotalCalls > 0 {
			totalRate += float64(s.SuccessCount) / float64(s.TotalCalls)
			count++
		}
	}
	avgRate := 0.5
	if count > 0 {
		avgRate = totalRate / float64(count)
	}

	successMultiplier := successRate / avgRate
	successMultiplier = math.Max(0.5, math.Min(1.5, successMultiplier))

	recentCalls := 0
	for i := len(e.data.FeedbackLog) - 1; i >= 0 && i >= len(e.data.FeedbackLog)-20; i-- {
		if strings.EqualFold(e.data.FeedbackLog[i].PipelineUsed, name) {
			recentCalls++
		}
	}
	recencyBoost := 1.0 + float64(recentCalls)/20.0*0.2
	recencyBoost = math.Min(recencyBoost, 1.2)

	weight := successMultiplier * recencyBoost
	return math.Max(0.1, math.Min(1.0, weight))
}

func (e *Engine) GetAccuracy(pipeline string) (successRate float64, totalCalls int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strings.ToLower(pipeline)
	stats := e.data.PipelineStats[name]
	if stats == nil || stats.TotalCalls == 0 {
		return 0, 0
	}
	return float64(stats.SuccessCount) / float64(stats.TotalCalls), stats.TotalCalls
}

func (e *Engine) GetTrend(pipeline string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.computeTrendLocked(pipeline)
}

func bucketIndex(confidence float64, buckets []ConfidenceBucket) int {
	for i, b := range buckets {
		if confidence >= b.Low && confidence < b.High {
			return i
		}
	}
	if confidence >= 0.95 {
		return len(buckets) - 1
	}
	return -1
}
