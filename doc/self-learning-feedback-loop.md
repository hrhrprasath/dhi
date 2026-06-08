# Self-Learning Feedback Loop Design

## Problem
- Pipeline weights are static — defined once in `config/pipelines.yaml` and `internal/pipeline/manager.go`, never adjusted
- Confidence scores are purely LLM-generated — never calibrated against actual outcomes
- No tracking of whether a pipeline selection was correct
- User corrections ("no, I meant search the web") are lost
- Routing mistakes repeat because the system never learns **which words** trigger which pipelines

---

## Solution Overview

A `internal/learn/` package that:
1. Tracks pipeline execution outcomes (success/failure)
2. Tracks **word→pipeline** associations from real user input
3. Adjusts pipeline weights based on historical accuracy including user ratings
4. Calibrates confidence scores using real success rates
5. Supports explicit user rating (`/good`, `/bad`)
6. Injects learned signals + keyword associations into the LLM prompt
7. Persists all learning data in `.data/learn/learning.json`

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    learn.Engine                              │
│                                                             │
│  ┌─────────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ PipelineStats    │  │ Calibration  │  │ KeywordStats  │  │
│  │ (success/fail    │  │ (confidence  │  │ (word→pipeline│  │
│  │  counts per      │  │  → accuracy  │  │  success/fail │  │
│  │  pipeline)       │  │  buckets)    │  │  counts)      │  │
│  └────────┬─────────┘  └──────┬───────┘  └───────┬───────┘  │
│           │                   │                   │          │
│           └─────────┬─────────┘───────────────────┘          │
│                     │                                        │
│           .data/learn/learning.json                           │
└──────────────────────────────────────────────────────────────┘
```

---

## Data Model

### PipelineStats (per-pipeline)
```json
{
  "name": "search_browser",
  "total_calls": 49,
  "success_count": 45,
  "fail_count": 4,
  "total_confidence": 35.28,
  "last_used": "2026-06-09T..."
}
```

### ConfidenceBucket (per-pipeline, per-confidence-range)
```json
{
  "low": 0.8,
  "high": 0.9,
  "count": 120,
  "success": 95
}
```

### KeywordStat (per-word, per-pipeline)
```json
{
  "pipeline": "browse",
  "count": 5,
  "success": 4
}
```

### FeedbackEntry (per-interaction)
```json
{
  "input": "open the documentation link",
  "predicted_event": "EXECUTE_SYSTEM",
  "confidence": 0.9,
  "pipeline_used": "system",
  "was_conversation": false,
  "success": true,
  "rated": -1,
  "error_msg": "",
  "timestamp": "2026-06-09T..."
}
```

### LearningData (root)
```json
{
  "version": 1,
  "pipeline_stats": { ... },
  "calibration": { ... },
  "keyword_pipelines": {
    "open": { "browse": {"pipeline":"browse","count":5,"success":4}, "system": {"pipeline":"system","count":2,"success":1} },
    "link": { "browse": {"pipeline":"browse","count":5,"success":4} },
    "documentation": { "search_browser": {"pipeline":"search_browser","count":5,"success":3} }
  },
  "feedback_log": [ ... ],
  "last_updated": "2026-06-09T..."
}
```

---

## Weight Adjustment Algorithm

`adjusted_weight = base_weight × success_rate_multiplier × recency_boost`

1. **SuccessRateMultiplier** = `pipeline_success_rate / avg_success_rate` (clamped [0.5, 1.5])
   - Pipelines performing above average get boosted
   - Pipelines below average get penalized
   - `TotalCalls` includes both automatic outcomes AND user ratings (`/good`, `/bad`)

2. **RecencyBoost** = `min(1.0 + (recent_calls_20 / 20) × 0.2, 1.2)`
   - Pipelines with recent activity get slight boost (max +20%)

3. **Final** = `baseWeight × multiplier`, clamped to [0.1, 1.0]

---

## Word→Pipeline Learning (NOT hardcoded)

The system does **not** hardcode stop words, keyword extraction rules, or NLP logic. Instead:

1. Every user input is split into individual words (whitespace, ≥3 chars, deduplicated)
2. Each word is associated with the pipeline that handled it
3. Success/failure is tracked per word-pipeline pair
4. The LLM sees these associations in the prompt and learns from them

**No stop word list. No hardcoded extraction rules. The LLM understands the relationships.**

### How ratings affect keyword stats

| Event | Word stats update | Pipeline stats update |
|-------|------------------|----------------------|
| Pipeline succeeds (auto) | word→pipeline success++ | TotalCalls++, SuccessCount++ |
| Pipeline fails (auto) | word→pipeline fail++ | TotalCalls++, FailCount++ |
| Prediction mismatch | word→predicted fail++ | Predicted pipeline: TotalCalls++, FailCount++ |
| `/good` rating | word→pipeline success++ | TotalCalls++, SuccessCount++ |
| `/bad` rating | word→pipeline fail++ | TotalCalls++, FailCount++ |

---

## Confidence Calibration

1. Bucket LLM confidence into bands: [0.5-0.6, 0.6-0.7, 0.7-0.8, 0.8-0.9, 0.9-1.0]
2. For each bucket, compute: `actual_success_rate = bucket_successes / bucket_count`
3. `calibrated_confidence = min(raw_confidence × (actual_rate / expected_rate), 1.0)`
4. Applied programmatically before the 0.8 threshold check in `processTasks()`

---

## Feedback Sources

### Automatic (implicit)
| Signal | Detection | Effect |
|--------|-----------|--------|
| Pipeline succeeds | Non-empty output, no error | +positive |
| Pipeline fails | Empty/bad output or error | +negative |
| Pipeline partially fails | Output contains "Script failed" | +negative |
| Prediction mismatch | Predicted pipeline ≠ used pipeline | -negative on predicted, +positive on used |

### Explicit (user-driven)
| Command | Effect |
|---------|--------|
| `/good` | +positive rating on last interaction |
| `/bad` | +negative rating on last interaction |

---

## Prompt Integration

### Pipeline schema with learned weights:
```
event: EXECUTE_SEARCH_BROWSER
description: Search the web using a browser
weight: 0.3 | learned: 0.43 | accuracy: 92% (49) | trend: ↑
keywords: open, link, url, website
```

### Calibration block:
```
CALIBRATION DATA:
- EXECUTE_SEARCH_BROWSER: 92% accuracy (45/49 calls) | trend: ↑
- EXECUTE_SEARCH_INLINE: 78% accuracy (112/144 calls) | trend: →
- EXECUTE_STOCKS: 95% accuracy (38/40 calls) | trend: ↑
```

### Keyword associations block:
```
KEYWORD-PIPELINE INSIGHTS:
PIPELINE HINTS (from config):
  BROWSE: open, link, url, website, page
LEARNED (from feedback):
  "open" → BROWSE (80% success, 5 uses)
  "link" → BROWSE (80% success, 5 uses)
  "documentation" → SEARCH_BROWSER (60% success, 5 uses)
  "system" → SYSTEM (95% success, 40 uses)
```

---

## Data Flow

```
User Input → taskMgr.AddOnce()
  ↓
processTasks() (200ms poll)
  ↓
router.Route() with calibrated prompt + keyword associations
  ↓
Parse JSON → get raw confidence
  ↓
learn.Engine.CalibrateWithEvent(raw_confidence, event)
  ↓
adjusted_confidence >= 0.8?
  ├─ YES → handlePipeline()
  │         ├─ executor.FindAndExecute(...)
  │         └─ learn.Engine.RecordOutcome(input, event, pipeline, confidence, success, false, errMsg)
  │              ├─ updates PipelineStats (TotalCalls/SuccessCount/FailCount)
  │              ├─ updates Calibration buckets
  │              └─ updates KeywordPipelines (word → pipeline associations)
  └─ NO  → handleConversation()
            └─ learn.Engine.RecordConversation(input)

User rates → /good or /bad
  └─ learn.Engine.RecordRating(input, positive)
       ├─ updates PipelineStats (TotalCalls++ + SuccessCount++ or FailCount++)
       └─ updates KeywordPipelines (word → pipeline with success/fail)
```

---

## Optional `keywords` in `pipelines.yaml`

Pipelines can include explicit keyword hints to guide the LLM:

```yaml
pipelines:
  - name: browse
    weight: 0.4
    keywords: ["open", "link", "url", "website", "page"]
    action:
      type: bash
      script: scripts/browse {url}
      summarize: true

  - name: search_browser
    weight: 0.3
    keywords: ["search", "find", "lookup", "google", "web"]
    action:
      type: bash
      script: scripts/search_browser {provider} {query}

  - name: system
    weight: 0.6
    keywords: ["system", "cpu", "ram", "memory", "uptime", "kernel"]
    action:
      type: internal
      handler: system_metrics
```

These appear in the prompt under `PIPELINE HINTS (from config)` and are merged with learned data under `LEARNED (from feedback)`.

---

## Slash Commands

| Command | Action |
|---------|--------|
| `/good` | Rate last interaction positive |
| `/bad` | Rate last interaction negative |
| `/log` `/logs` `/l` | Toggle log popup |
| `/quit` `/q` | Quit |

Tab autocomplete cycles through matching commands when input starts with `/`.

---

## Files Created

| File | Lines | Purpose |
|------|-------|---------|
| `internal/learn/store.go` | ~140 | Engine struct, data model, JSON persistence |
| `internal/learn/stats.go` | ~240 | PipelineStats tracking, weight computation, outcome recording |
| `internal/learn/calibrate.go` | ~75 | Confidence calibration + bucket management |
| `internal/learn/prompt.go` | ~135 | Prompt-building helpers (weight lines, calibration block) |
| `internal/learn/keywords.go` | ~140 | Word extraction, word→pipeline tracking, keyword block builder |

## Files Modified

| File | Changes |
|------|---------|
| `internal/prompt/templates.go` | `BuildSchema(weightLines)` accepts learned weight lines; `ClassifyPrompt(..., calibrationBlock, keywordBlock, weightLines)` injects all learning data; `PipelineInfo.Keywords` field |
| `internal/pipeline/manager.go` | `Pipeline.Keywords` field for YAML `keywords:` |
| `internal/intent/router.go` | Added `WeightProvider` interface + `BuildKeywordBlock`; `Route()` builds weight lines, calibration block, keyword block; calibrates confidence after parse |
| `tui/types.go` | Added `FeedbackRecorder` interface; `feedback`, `lastInput`, `lastIntent`, `lastSubmitted`, `slashMatches`, `slashIndex` fields |
| `tui/model.go` | `NewModel` accepts `FeedbackRecorder` |
| `tui/update.go` | `/good` `/bad` handlers; `handlePipeline` records outcomes with word tracking; `handleConversation` records conversation; up-arrow repopulates last input; Tab autocomplete for `/` commands |
| `cmd/dhi/main.go` | Creates `learn.NewEngine`, sets as weight provider + feedback recorder |
