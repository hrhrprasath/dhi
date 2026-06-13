package learn

import (
	"fmt"
	"math"
	"strings"
)

var defaultBuckets = []ConfidenceBucket{
	{Low: 0.5, High: 0.6},
	{Low: 0.6, High: 0.7},
	{Low: 0.7, High: 0.8},
	{Low: 0.8, High: 0.9},
	{Low: 0.9, High: 1.0},
}

func (e *Engine) Calibrate(rawConfidence float64, pipeline string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strings.ToLower(pipeline)
	buckets := e.data.Calibration[name]
	if len(buckets) == 0 {
		return rawConfidence
	}

	idx := bucketIndex(rawConfidence, buckets)
	if idx < 0 || buckets[idx].Count < 5 {
		return rawConfidence
	}

	b := buckets[idx]
	actualRate := float64(b.Success) / float64(b.Count)
	midPoint := (b.Low + b.High) / 2
	expectedRate := midPoint

	if actualRate == 0 {
		return rawConfidence * 0.8
	}

	calibrationFactor := actualRate / expectedRate
	calibrated := rawConfidence * calibrationFactor
	return math.Max(0.1, math.Min(1.0, calibrated))
}

func (e *Engine) CalibrateWithEvent(rawConfidence float64, event string) float64 {
	pipeline := strings.TrimPrefix(strings.ToUpper(event), "EXECUTE_")
	if pipeline == event {
		if strings.EqualFold(event, "CONVERSATION") {
			return rawConfidence
		}
		return rawConfidence
	}
	return e.Calibrate(rawConfidence, pipeline)
}

func (e *Engine) CalibrationSummary(event string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	name := strings.ToLower(strings.TrimPrefix(event, "EXECUTE_"))
	buckets := e.data.Calibration[name]
	if len(buckets) == 0 {
		return ""
	}

	var parts []string
	for _, b := range buckets {
		if b.Count < 3 {
			continue
		}
		pct := float64(b.Success) / float64(b.Count) * 100
		parts = append(parts, formatBucket(b.Low, b.High, pct, b.Count))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func formatBucket(low, high, pct float64, count int) string {
	return fmt.Sprintf("%.0f-%.0f: %.0f%% (%d)", low*100, high*100, pct, count)
}
