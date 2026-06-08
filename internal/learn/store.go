package learn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PipelineStats struct {
	Name           string    `json:"name"`
	TotalCalls     int       `json:"total_calls"`
	SuccessCount   int       `json:"success_count"`
	FailCount      int       `json:"fail_count"`
	TotalConfidence float64  `json:"total_confidence"`
	LastUsed       time.Time `json:"last_used"`
}

type ConfidenceBucket struct {
	Low     float64 `json:"low"`
	High    float64 `json:"high"`
	Count   int     `json:"count"`
	Success int     `json:"success"`
}

type KeywordStat struct {
	Pipeline string `json:"pipeline"`
	Count    int    `json:"count"`
	Success  int    `json:"success"`
}

type FeedbackEntry struct {
	Input           string    `json:"input"`
	PredictedEvent  string    `json:"predicted_event"`
	Confidence      float64   `json:"confidence"`
	PipelineUsed    string    `json:"pipeline_used"`
	WasConversation bool      `json:"was_conversation"`
	Success         bool      `json:"success"`
	Rated           int       `json:"rated"`
	ErrorMsg        string    `json:"error_msg,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

type LearningData struct {
	Version           int                            `json:"version"`
	PipelineStats     map[string]*PipelineStats      `json:"pipeline_stats"`
	Calibration       map[string][]ConfidenceBucket   `json:"calibration"`
	KeywordPipelines  map[string]map[string]*KeywordStat `json:"keyword_pipelines"`
	FeedbackLog       []FeedbackEntry                `json:"feedback_log"`
	LastUpdated       time.Time                      `json:"last_updated"`
}

const maxFeedbackLog = 500

type Engine struct {
	mu   sync.Mutex
	data LearningData
	path string
}

func NewEngine(dataDir string) *Engine {
	dir := filepath.Join(dataDir, "learn")
	os.MkdirAll(dir, 0755)
	p := filepath.Join(dir, "learning.json")

	e := &Engine{path: p}
	e.data = e.load()
	if e.data.Version == 0 {
		e.data.Version = 1
		e.data.PipelineStats = make(map[string]*PipelineStats)
		e.data.Calibration = make(map[string][]ConfidenceBucket)
	}
	if e.data.KeywordPipelines == nil {
		e.data.KeywordPipelines = make(map[string]map[string]*KeywordStat)
	}
	return e
}

func (e *Engine) load() LearningData {
	data, err := os.ReadFile(e.path)
	if err != nil {
		return LearningData{
			Version:          1,
			PipelineStats:    make(map[string]*PipelineStats),
			Calibration:      make(map[string][]ConfidenceBucket),
			KeywordPipelines: make(map[string]map[string]*KeywordStat),
		}
	}
	var ld LearningData
	if json.Unmarshal(data, &ld) != nil {
		return LearningData{
			Version:          1,
			PipelineStats:    make(map[string]*PipelineStats),
			Calibration:      make(map[string][]ConfidenceBucket),
			KeywordPipelines: make(map[string]map[string]*KeywordStat),
		}
	}
	if ld.PipelineStats == nil {
		ld.PipelineStats = make(map[string]*PipelineStats)
	}
	if ld.Calibration == nil {
		ld.Calibration = make(map[string][]ConfidenceBucket)
	}
	if ld.KeywordPipelines == nil {
		ld.KeywordPipelines = make(map[string]map[string]*KeywordStat)
	}
	return ld
}

func (e *Engine) save() {
	e.data.LastUpdated = time.Now()
	data, _ := json.MarshalIndent(e.data, "", "  ")
	os.WriteFile(e.path, data, 0644)
}

func (e *Engine) Stats() map[string]*PipelineStats {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make(map[string]*PipelineStats, len(e.data.PipelineStats))
	for k, v := range e.data.PipelineStats {
		cp := *v
		out[k] = &cp
	}
	return out
}

func (e *Engine) CalibrationData() map[string][]ConfidenceBucket {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make(map[string][]ConfidenceBucket, len(e.data.Calibration))
	for k, buckets := range e.data.Calibration {
		cb := make([]ConfidenceBucket, len(buckets))
		copy(cb, buckets)
		out[k] = cb
	}
	return out
}

func (e *Engine) FeedbackLog() []FeedbackEntry {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]FeedbackEntry, len(e.data.FeedbackLog))
	copy(out, e.data.FeedbackLog)
	return out
}
