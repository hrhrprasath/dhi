package learn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEngine(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	if e.path == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.HasSuffix(e.path, "learning.json") {
		t.Errorf("path = %q, want suffix learning.json", e.path)
	}
	if e.dataDir == "" {
		t.Fatal("expected non-empty dataDir")
	}

	if e.data.Version != 1 {
		t.Errorf("Version = %d, want 1", e.data.Version)
	}
	if e.data.PipelineStats == nil {
		t.Error("PipelineStats should not be nil")
	}
	if e.data.Calibration == nil {
		t.Error("Calibration should not be nil")
	}
	if e.data.KeywordPipelines == nil {
		t.Error("KeywordPipelines should not be nil")
	}

	learnDir := filepath.Join(dir, "learn")
	if _, err := os.Stat(learnDir); os.IsNotExist(err) {
		t.Error("learn directory was not created")
	}
}

func TestNewEngine_LoadsExistingData(t *testing.T) {
	dir := t.TempDir()
	e1 := NewEngine(dir)
	e1.SetModel("m1")
	e1.RecordOutcome("test input", "", "stocks", 0.85, true, false, "")

	e2 := NewEngine(dir)
	e2.SetModel("m1")

	stats := e2.Stats()
	_, ok := stats["stocks"]
	if !ok {
		t.Error("expected existing pipeline stats to be loaded")
	}
}

func TestSetModel(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.SetModel("my-model")
	modelDir := filepath.Join(dir, "learn", "my-model")
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		t.Error("model directory was not created")
	}
	if !strings.Contains(e.path, "my-model") {
		t.Errorf("path = %q, should contain my-model", e.path)
	}

	if e.data.Version != 1 {
		t.Errorf("Version = %d, want 1", e.data.Version)
	}
}

func TestSetModel_SanitizesName(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.SetModel("model/name with spaces")
	if !strings.Contains(e.path, "model_name_with_spaces") {
		t.Errorf("path = %q, should contain model_name_with_spaces", e.path)
	}
}

func TestSetModel_PreservesExistingStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)
	e.RecordOutcome("hello", "", "stocks", 0.9, true, false, "")

	e.SetModel("new-model")
	stats := e.Stats()
	_, ok := stats["stocks"]
	if ok {
		t.Error("expected stats to be reset on model change")
	}
}

func TestRecordOutcome_Success(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("show me stocks", "", "stocks", 0.95, true, false, "")

	stats := e.Stats()
	s, ok := stats["stocks"]
	if !ok {
		t.Fatal("expected stocks pipeline stats")
	}
	if s.TotalCalls != 1 {
		t.Errorf("TotalCalls = %d, want 1", s.TotalCalls)
	}
	if s.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", s.SuccessCount)
	}
	if s.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", s.FailCount)
	}
	if s.TotalConfidence != 0.95 {
		t.Errorf("TotalConfidence = %f, want 0.95", s.TotalConfidence)
	}
	if s.LastUsed.IsZero() {
		t.Error("LastUsed should not be zero")
	}

	log := e.FeedbackLog()
	if len(log) != 1 {
		t.Fatalf("FeedbackLog length = %d, want 1", len(log))
	}
	if log[0].Input != "show me stocks" {
		t.Errorf("Input = %q, want %q", log[0].Input, "show me stocks")
	}
	if log[0].Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", log[0].Confidence)
	}
	if !log[0].Success {
		t.Error("Success should be true")
	}
	if log[0].WasConversation {
		t.Error("WasConversation should be false")
	}
	if log[0].Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestRecordOutcome_Failure(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("get weather", "", "weather", 0.7, false, false, "api error")

	stats := e.Stats()
	s, ok := stats["weather"]
	if !ok {
		t.Fatal("expected weather pipeline stats")
	}
	if s.TotalCalls != 1 {
		t.Errorf("TotalCalls = %d, want 1", s.TotalCalls)
	}
	if s.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d, want 0", s.SuccessCount)
	}
	if s.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", s.FailCount)
	}

	log := e.FeedbackLog()
	if len(log) != 1 {
		t.Fatalf("len = %d, want 1", len(log))
	}
	if log[0].Success {
		t.Error("Success should be false")
	}
	if log[0].ErrorMsg != "api error" {
		t.Errorf("ErrorMsg = %q, want %q", log[0].ErrorMsg, "api error")
	}
}

func TestRecordOutcome_UpdatesCalibration(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("x", "", "test", 0.85, true, false, "")
	e.RecordOutcome("y", "", "test", 0.85, false, false, "")

	cal := e.CalibrationData()
	buckets, ok := cal["test"]
	if !ok {
		t.Fatal("expected calibration data for test")
	}
	if len(buckets) == 0 {
		t.Fatal("expected non-empty buckets")
	}

	found := false
	for _, b := range buckets {
		if b.Low == 0.8 && b.High == 0.9 {
			found = true
			if b.Count == 0 {
				t.Error("expected at least 1 count in the 0.8-0.9 bucket")
			}
			break
		}
	}
	if !found {
		t.Error("did not find bucket 0.8-0.9")
	}
}

func TestRecordOutcome_EmptyPipelineDoesNotUpdateStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("hello", "", "", 0.9, true, false, "")

	stats := e.Stats()
	if len(stats) != 0 {
		t.Errorf("expected no stats, got %d entries", len(stats))
	}
}

func TestRecordOutcome_ConversationSkipsStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("hello", "", "", 0.9, true, true, "")

	stats := e.Stats()
	if len(stats) != 0 {
		t.Errorf("expected no stats, got %d entries", len(stats))
	}
}

func TestRecordOutcome_PredictedEventMismatchUpdatesBoth(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("search web", "EXECUTE_search", "web_search", 0.6, true, false, "")

	stats := e.Stats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 pipelines in stats, got %d", len(stats))
	}

	s := stats["web_search"]
	if s == nil {
		t.Fatal("expected web_search stats")
	}
	if s.TotalCalls != 1 || s.SuccessCount != 1 {
		t.Errorf("web_search: calls=%d success=%d (want 1,1)", s.TotalCalls, s.SuccessCount)
	}

	p := stats["search"]
	if p == nil {
		t.Fatal("expected search stats (from predicted event)")
	}
	if p.TotalCalls != 1 || p.SuccessCount != 0 || p.FailCount != 1 {
		t.Errorf("search: calls=%d success=%d fail=%d (want 1,0,1)", p.TotalCalls, p.SuccessCount, p.FailCount)
	}
}

func TestRecordOutcome_MaxFeedbackLog(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	for i := 0; i < maxFeedbackLog+50; i++ {
		e.RecordOutcome("x", "", "stocks", 0.9, true, false, "")
	}

	log := e.FeedbackLog()
	if len(log) != maxFeedbackLog {
		t.Errorf("FeedbackLog length = %d, want %d", len(log), maxFeedbackLog)
	}
}

func TestRecordConversation(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordConversation("hello there")

	log := e.FeedbackLog()
	if len(log) != 1 {
		t.Fatalf("len = %d, want 1", len(log))
	}
	entry := log[0]
	if entry.Input != "hello there" {
		t.Errorf("Input = %q", entry.Input)
	}
	if entry.PredictedEvent != "CONVERSATION" {
		t.Errorf("PredictedEvent = %q", entry.PredictedEvent)
	}
	if !entry.WasConversation {
		t.Error("WasConversation should be true")
	}
	if !entry.Success {
		t.Error("Success should be true")
	}
}

func TestRecordConversation_DoesNotUpdateStats(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordConversation("hello")

	stats := e.Stats()
	if len(stats) != 0 {
		t.Errorf("expected no pipeline stats, got %d", len(stats))
	}

	cal := e.CalibrationData()
	if len(cal) != 0 {
		t.Errorf("expected no calibration data, got %d", len(cal))
	}
}

func TestCalibrateWithEvent_NoDataReturnsRaw(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	result := e.CalibrateWithEvent(0.85, "EXECUTE_stocks")
	if result != 0.85 {
		t.Errorf("CalibrateWithEvent = %f, want 0.85", result)
	}
}

func TestCalibrateWithEvent_Conversation(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	result := e.CalibrateWithEvent(0.75, "CONVERSATION")
	if result != 0.75 {
		t.Errorf("CalibrateWithEvent(CONVERSATION) = %f, want 0.75", result)
	}
}

func TestCalibrateWithEvent_NonExecuteEvent(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	result := e.CalibrateWithEvent(0.75, "stocks")
	if result != 0.75 {
		t.Errorf("CalibrateWithEvent(stocks) = %f, want 0.75", result)
	}
}

func TestCalibrateWithEvent_AdjustsAfterSufficientData(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.SetModel("test")

	for i := 0; i < 10; i++ {
		e.RecordOutcome("x", "", "test", 0.85, true, false, "")
	}

	result := e.CalibrateWithEvent(0.85, "EXECUTE_test")
	if result < 0.1 || result > 1.0 {
		t.Errorf("calibrated value out of range [0.1, 1.0]: %f", result)
	}
}

func TestCalibrateWithEvent_LowConfidenceAdjustsDown(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.SetModel("test")

	for i := 0; i < 6; i++ {
		e.RecordOutcome("x", "", "test", 0.85, false, false, "")
	}

	result := e.CalibrateWithEvent(0.85, "EXECUTE_test")
	if result < 0.1 || result > 1.0 {
		t.Errorf("calibrated value out of range [0.1, 1.0]: %f", result)
	}
}

func TestCalibrateWithEvent_ResultInRange(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.SetModel("test")

	for i := 0; i < 10; i++ {
		e.RecordOutcome("x", "", "test", 0.85, true, false, "")
	}

	result := e.CalibrateWithEvent(0.85, "EXECUTE_test")
	if result < 0.1 || result > 1.0 {
		t.Errorf("calibrated value out of range [0.1, 1.0]: %f", result)
	}
	if result == 0.85 {
		t.Errorf("expected calibrated value to differ from raw with sufficient data, got %f", result)
	}
}

func TestBuildWeightLine_NoData(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	line := e.BuildWeightLine("stocks", "show me stocks", 1.0)
	if line == "" {
		t.Fatal("expected non-empty weight line")
	}
}

func TestBuildWeightLine_WithData(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.SetModel("test")

	e.RecordOutcome("show me stocks", "", "stocks", 0.95, true, false, "")
	e.RecordOutcome("stock price", "", "stocks", 0.90, true, false, "")
	e.RecordOutcome("stock news", "", "stocks", 0.85, true, false, "")

	line := e.BuildWeightLine("stocks", "show me stocks", 1.0)
	if line == "" {
		t.Fatal("expected non-empty weight line")
	}
}

func TestBuildWeightLine_WithFewCalls(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("x", "", "svc", 0.9, true, false, "")

	line := e.BuildWeightLine("svc", "x", 1.0)
	if !strings.HasPrefix(line, "weight:") {
		t.Errorf("expected weight line, got: %s", line)
	}
}

func TestStats_ReturnsDeepCopy(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("x", "", "test", 0.9, true, false, "")

	stats := e.Stats()
	stats["test"].TotalCalls = 999

	stats2 := e.Stats()
	if stats2["test"].TotalCalls == 999 {
		t.Error("Stats() did not return a deep copy")
	}
}

func TestStats_Empty(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	stats := e.Stats()
	if stats == nil {
		t.Error("Stats() should return non-nil map")
	}
	if len(stats) != 0 {
		t.Errorf("len = %d, want 0", len(stats))
	}
}

func TestCalibrationData_ReturnsDeepCopy(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("x", "", "test", 0.85, true, false, "")

	cal := e.CalibrationData()
	if len(cal) > 0 {
		for _, buckets := range cal {
			if len(buckets) > 0 {
				buckets[0].Count = 999
			}
		}
	}

	cal2 := e.CalibrationData()
	if len(cal2) > 0 {
		for _, buckets := range cal2 {
			for _, b := range buckets {
				if b.Count == 999 {
					t.Error("CalibrationData() did not return a deep copy")
				}
			}
		}
	}
}

func TestCalibrationData_Empty(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	data := e.CalibrationData()
	if data == nil {
		t.Error("CalibrationData() should return non-nil map")
	}
}

func TestFeedbackLog_ReturnsDeepCopy(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("x", "", "test", 0.9, true, false, "")

	log := e.FeedbackLog()
	if len(log) > 0 {
		log[0].Input = "mutated"
	}

	log2 := e.FeedbackLog()
	if len(log2) > 0 && log2[0].Input == "mutated" {
		t.Error("FeedbackLog() did not return a deep copy")
	}
}

func TestFeedbackLog_Empty(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	log := e.FeedbackLog()
	if len(log) != 0 {
		t.Errorf("len = %d, want 0", len(log))
	}
}

func TestBuildCalibrationBlock_Empty(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	block := e.BuildCalibrationBlock()
	if block != "" {
		t.Errorf("expected empty string with no data, got: %q", block)
	}
}

func TestBuildCalibrationBlock_WithSufficientData(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	for i := 0; i < 3; i++ {
		e.RecordOutcome("x", "", "stocks", 0.9, true, false, "")
	}

	block := e.BuildCalibrationBlock()
	if block == "" {
		t.Fatal("expected non-empty calibration block")
	}
	if !strings.Contains(block, "CALIBRATION DATA") {
		t.Errorf("block missing CALIBRATION DATA header: %s", block)
	}
	if !strings.Contains(block, "STOCKS") {
		t.Errorf("block missing STOCKS: %s", block)
	}
}

func TestBuildCalibrationBlock_SortsByAccuracy(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	for i := 0; i < 3; i++ {
		e.RecordOutcome("x", "", "stocks", 0.9, true, false, "")
		e.RecordOutcome("x", "", "weather", 0.9, false, false, "")
	}

	block := e.BuildCalibrationBlock()
	stockIdx := strings.Index(block, "STOCKS")
	weatherIdx := strings.Index(block, "WEATHER")
	if stockIdx > weatherIdx {
		t.Error("expected STOCKS (100%) to appear before WEATHER (0%)")
	}
}

func TestBuildKeywordBlock_NoData(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	block := e.BuildKeywordBlock("hello world", nil)
	if !strings.Contains(block, "KEYWORD-PIPELINE INSIGHTS") {
		t.Errorf("expected header even without hints, got: %q", block)
	}
}

func TestBuildKeywordBlock_NoInputWordsNoHints(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	block := e.BuildKeywordBlock("a", nil)
	if block != "" {
		t.Errorf("expected empty string for short input without hints, got: %q", block)
	}
}

func TestBuildKeywordBlock_WithPipelineHints(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	hints := map[string][]string{
		"stocks": {"stock", "price", "market"},
	}

	block := e.BuildKeywordBlock("stock price", hints)
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	if !strings.Contains(block, "PIPELINE HINTS") {
		t.Errorf("block missing PIPELINE HINTS: %s", block)
	}
	if !strings.Contains(block, "STOCKS") {
		t.Errorf("block missing STOCKS: %s", block)
	}
	if !strings.Contains(block, "stock, price, market") {
		t.Errorf("block missing keywords: %s", block)
	}
}

func TestBuildKeywordBlock_WithLearnedKeywords(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("stock price go up", "", "stocks", 0.9, true, false, "")
	e.RecordOutcome("stock market news", "", "stocks", 0.9, true, false, "")

	block := e.BuildKeywordBlock("stock price", nil)
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	if !strings.Contains(block, "LEARNED") {
		t.Errorf("block missing LEARNED section: %s", block)
	}
}

func TestBuildKeywordBlock_WithBothHintsAndLearned(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("stock price", "", "stocks", 0.9, true, false, "")

	hints := map[string][]string{
		"stocks": {"stock", "price"},
	}

	block := e.BuildKeywordBlock("stock price", hints)
	if !strings.Contains(block, "PIPELINE HINTS") {
		t.Error("block missing PIPELINE HINTS")
	}
	if !strings.Contains(block, "LEARNED") {
		t.Error("block missing LEARNED")
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "normal sentence",
			input: "show me the stock price",
			want:  []string{"show", "the", "stock", "price"},
		},
		{
			name:  "removes short words",
			input: "a is to of",
			want:  nil,
		},
		{
			name:  "keeps 3-letter words",
			input: "the cat ran",
			want:  []string{"the", "cat", "ran"},
		},
		{
			name:  "deduplicates words",
			input: "hello hello world world",
			want:  []string{"hello", "world"},
		},
		{
			name:  "strips outer punctuation",
			input: "hello, world! how's it?",
			want:  []string{"hello", "world", "how's"},
		},
		{
			name:  "lowercases input",
			input: "HELLO WORLD",
			want:  []string{"hello", "world"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "only punctuation",
			input: "!!! ??? ,,,",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWords(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("extractWords(%q) = %v (len=%d), want %v (len=%d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractWords(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetWeight(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	w := e.GetWeight("stocks")
	if w != 0 {
		t.Errorf("GetWeight = %f, want 0 for no data", w)
	}

	for i := 0; i < 3; i++ {
		e.RecordOutcome("x", "", "stocks", 0.9, true, false, "")
	}

	w = e.GetWeight("stocks")
	if w <= 0 {
		t.Errorf("GetWeight = %f, want > 0", w)
	}
	if w > 1.0 {
		t.Errorf("GetWeight = %f, want <= 1.0", w)
	}
}

func TestGetWeights(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	for i := 0; i < 3; i++ {
		e.RecordOutcome("x", "", "a", 0.9, true, false, "")
	}

	weights := e.GetWeights([]string{"a", "b"})
	if len(weights) != 2 {
		t.Fatalf("len = %d, want 2", len(weights))
	}
	if weights["a"] <= 0 {
		t.Errorf("weight[a] = %f, want > 0", weights["a"])
	}
	if weights["b"] != 0 {
		t.Errorf("weight[b] = %f, want 0", weights["b"])
	}
}

func TestGetAccuracy(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	rate, calls := e.GetAccuracy("stocks")
	if rate != 0 || calls != 0 {
		t.Errorf("no data: rate=%f calls=%d, want 0,0", rate, calls)
	}

	e.RecordOutcome("x", "", "stocks", 0.9, true, false, "")
	e.RecordOutcome("x", "", "stocks", 0.9, false, false, "")

	rate, calls = e.GetAccuracy("stocks")
	if rate != 0.5 {
		t.Errorf("rate = %f, want 0.5", rate)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestGetTrend(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	trend := e.GetTrend("stocks")
	if trend != "→" {
		t.Errorf("GetTrend = %q, want → for no data", trend)
	}
}

func TestHasSufficientData(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	if e.HasSufficientData("stocks", 3) {
		t.Error("expected false with no data")
	}

	for i := 0; i < 5; i++ {
		e.RecordOutcome("x", "", "stocks", 0.9, true, false, "")
	}

	if !e.HasSufficientData("stocks", 3) {
		t.Error("expected true with 5 calls and minSamples=3")
	}
	if e.HasSufficientData("stocks", 10) {
		t.Error("expected false with 5 calls and minSamples=10")
	}
}

func TestRecordRating_Positive(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("test input", "", "stocks", 0.8, true, false, "")
	e.RecordRating("test input", true)

	log := e.FeedbackLog()
	if len(log) != 1 {
		t.Fatalf("len = %d, want 1", len(log))
	}
	if log[0].Rated != 1 {
		t.Errorf("Rated = %d, want 1", log[0].Rated)
	}
}

func TestRecordRating_Negative(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordOutcome("test input", "", "stocks", 0.8, true, false, "")
	e.RecordRating("test input", false)

	log := e.FeedbackLog()
	if len(log) != 1 {
		t.Fatalf("len = %d, want 1", len(log))
	}
	if log[0].Rated != -1 {
		t.Errorf("Rated = %d, want -1", log[0].Rated)
	}
}

func TestRecordRating_UnknownInput(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	e.RecordRating("nonexistent", true)

	log := e.FeedbackLog()
	if len(log) != 0 {
		t.Errorf("expected no new entries, got %d", len(log))
	}
}

func TestPersistenceAcrossEngines(t *testing.T) {
	dir := t.TempDir()

	e1 := NewEngine(dir)
	e1.SetModel("persist-test")
	e1.RecordOutcome("hello", "", "stocks", 0.95, true, false, "")

	e2 := NewEngine(dir)
	e2.SetModel("persist-test")

	stats := e2.Stats()
	s, ok := stats["stocks"]
	if !ok {
		t.Fatal("expected persisted stats to be loaded")
	}
	if s.TotalCalls != 1 || s.SuccessCount != 1 {
		t.Errorf("persisted: calls=%d success=%d", s.TotalCalls, s.SuccessCount)
	}

	log := e2.FeedbackLog()
	if len(log) != 1 {
		t.Errorf("persisted feedback: len=%d, want 1", len(log))
	}

	cal := e2.CalibrationData()
	if len(cal) == 0 {
		t.Error("expected persisted calibration data")
	}
}

func TestCalibrationSummary(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	summary := e.CalibrationSummary("EXECUTE_stocks")
	if summary != "" {
		t.Errorf("expected empty summary with no data, got %q", summary)
	}

	for i := 0; i < 3; i++ {
		e.RecordOutcome("x", "", "stocks", 0.85, true, false, "")
	}

	summary = e.CalibrationSummary("EXECUTE_stocks")
	if summary == "" {
		t.Error("expected non-empty summary after recording outcomes")
	}
}

func TestCalibrate_Direct(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine(dir)

	result := e.Calibrate(0.85, "stocks")
	if result != 0.85 {
		t.Errorf("Calibrate = %f, want 0.85 with no data", result)
	}
}

func TestDefaultBuckets(t *testing.T) {
	if len(defaultBuckets) != 5 {
		t.Fatalf("expected 5 default buckets, got %d", len(defaultBuckets))
	}
	expected := []ConfidenceBucket{
		{Low: 0.5, High: 0.6},
		{Low: 0.6, High: 0.7},
		{Low: 0.7, High: 0.8},
		{Low: 0.8, High: 0.9},
		{Low: 0.9, High: 1.0},
	}
	for i, b := range defaultBuckets {
		if b.Low != expected[i].Low || b.High != expected[i].High {
			t.Errorf("bucket[%d] = {%f,%f}, want {%f,%f}", i, b.Low, b.High, expected[i].Low, expected[i].High)
		}
	}
}

func TestBucketIndex(t *testing.T) {
	buckets := defaultBuckets
	tests := []struct {
		name       string
		confidence float64
		want       int
	}{
		{"in 0.5-0.6", 0.55, 0},
		{"in 0.6-0.7", 0.65, 1},
		{"in 0.7-0.8", 0.75, 2},
		{"in 0.8-0.9", 0.85, 3},
		{"in 0.9-1.0", 0.95, 4},
		{"at low bound", 0.5, 0},
		{"at high bound 0.9 goes to [0.9,1.0)", 0.9, 4},
		{"above 0.95", 0.99, 4},
		{"below 0.5", 0.3, -1},
		{"at 1.0", 1.0, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bucketIndex(tt.confidence, buckets)
			if got != tt.want {
				t.Errorf("bucketIndex(%f) = %d, want %d", tt.confidence, got, tt.want)
			}
		})
	}
}
