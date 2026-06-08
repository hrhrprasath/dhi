# DHI Harness — Design & Architecture Analysis

## 1. Architecture Summary

The system implements a **Cognitive Sandwich** pattern:

```
Human Input → [Local LLM: Intent→JSON] → [Go Pipelines: Deterministic Exec] → [LLM: Raw→Summary] → TUI Output
                ▲ TOP BREAD                    ▲ MEAT                         ▲ BOTTOM BREAD
```

**Current stack**: Local Qwen 1.5B → Go pipelines (bash scripts + internal handlers) → Same Qwen 1.5B

---

## 2. Design Patterns Used

| Pattern | Location | Assessment |
|---------|----------|------------|
| **Cognitive Sandwich** | AI at boundaries (classify → execute → summarize) | Core innovation — sound concept |
| **Event-Driven** | Go channels + Bubble Tea Msg types | Correct decoupling for TUI |
| **Task System** | Unified `internal/task` package — one-shot + scheduled | Replaces CQRS queues, simpler |
| **Pipeline / Chain of Responsibility** | YAML-defined actions + internal handlers | Flexible, extensible, testable |
| **Strategy** | Action types (bash, http, exec, internal, open) | Polymorphic execution from config |
| **Observer** | `logger.Subscribe()` | Decouples logging from consumers |
| **TimeParser Interface** | Swappable NL time parsers | Future-proof for better NLP |

---

## 3. Architecture Evolution

```
User ──► taskMgr.AddOnce(rawInput) ──► processTasks() ──► Route() ──► Execute ──► TUI
               │                            │
         Persist before LLM           Classify + dispatch
```

**Key design decisions:**

| Before | Now | Reason |
|--------|-----|--------|
| Input queue AFTER LLM | Task system stores raw input BEFORE classification | Crash recovery; retries preserve original input |
| Output queue + display + persist | Task stores result; TUI reads from event channel | No dual-consumer inconsistency |
| 7 varied pipelines | 8 focused pipelines (stocks, search, browse, music, todo, system, schedule...) | Simpler, more focused |
| curl subprocess only | bash/http/exec/open/internal + interpolation | Same pragmatic approach, all in stdlib |

---

## 4. Current Architecture Decisions

### 4.1 Task System

Input is stored before any LLM classification — crash-safe by design:

```
User ──► taskMgr.AddOnce(rawInput) ──► task persisted (pending)
              │
              ▼
         processTasks() (200ms poll)
              │
        ┌─────┴─────┐
        ▼           ▼
   classify      (known intent = bypass)
        │
        ▼
   execute
        │
        ▼
   mark completed / failed
```

Key benefits:
- Raw input is persisted before any LLM call
- Crash recovery: pending tasks resume on startup
- Scheduled tasks share the same execution path
- Retries don't need re-classification (but currently do — future optimization)

### 4.2 Single LLM for Both Layers

Both top bread (classify) and bottom bread (summarize) use the same local Qwen model via Ollama. Acceptable for a local-first system. The sandwich is two thin slices of the same bread, but it works.

### 4.3 No LLM Bypass

Every command passes through Ollama for classification. There's no:
- Regex fast path for trivial commands ("quit", "help")
- Cached intent lookup
- Keyword matching before LLM

This is a known limitation. The 200ms poll + local model keeps latency acceptable (~500ms–2s per classify).

### 4.4 Multi-Schedule Design

The schedule pipeline accepts a `schedules` JSON array param for multi-step scheduling from a single user input. The LLM must output:
```json
{
  "event": "EXECUTE_SCHEDULE",
  "params": {
    "action": "add",
    "schedules": "[{\"when\": \"daily at 9am\", \"command\": \"play song X\"}, ...]"
  }
}
```

This pushes composability to the LLM rather than building pipeline chaining in Go. Simple and pragmatic for the local model's capability.

---

## 5. Key Strengths

### 5.1 Clean TUI Architecture

Each component has a single file with clear responsibility:
- `types.go` — all data structures
- `model.go` — core event loop
- `view.go` — pure rendering
- `update.go` — side effects and processing

The Bubble Tea model (Model → Update → View cycle) enforces this separation naturally.

### 5.2 Deterministic Pipeline Execution

Pipeline actions are pure Go: bash, http, exec, open, internal handlers. No AI decision-making during execution. Results are predictable, debuggable, and fast.

### 5.3 Robust Task Persistence

Tasks survive crashes and restarts:
- `AddOnce()` → immediately persisted before LLM call
- `AddScheduled()` → persisted with next run time
- Looper activates on startup AND every 60s
- 3-retry limit prevents infinite loops

---

## 6. Known Gaps

Notable items:

| Gap | Impact | Priority |
|-----|--------|----------|
| No log word-wrap | Long log lines overflow panel | Low (1 line fix) |
| No NVIDIA placeholder | Missing from metrics panel | Low (cosmetic) |
| No scheduled task list in TUI | Can't see upcoming schedules | Medium |
| No tests | Regression risk | Low (deferred) |
| conversation_continue is stub | Topic persistence not fully implemented | Low |
| LLM bypass for known commands | Every command hits LLM unnecessarily | Medium |

---

## 7. Risk Assessment

```
Cognitive Sandwich concept:       8/10  (genuinely innovative architectural insight)
Design coherence:                  7/10  (task system fixes queue-before-LLM flaw)
Implementation fidelity:           7/10  (matches design doc, some gaps)
Code quality:                      7/10  (clean Go, modular, no unsafe)
Resilience:                        5/10  (persistence good, no circuit breaker, single point on LLM)
Production readiness:              4/10  (no auth, no graceful shutdown of goroutines)
```

### Summary

Current state:
- ✅ Task system persists raw input before LLM (crash-safe)
- ✅ 8 focused pipelines (stocks, search, browse, music, todo, system, schedule...)
- ✅ Goroutine + channel concurrency model
- ✅ Bubble Tea v2 TUI
- ✅ Self-learning feedback loop (learn.Engine with keyword-pipeline tracking)
- ⚠️ Single LLM for both layers (acceptable tradeoff for local-first)
- ⚠️ No tests yet (deferred)
- ❌ No LLM fast-path for trivial commands (acceptable for now)
