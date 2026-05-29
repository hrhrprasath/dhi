# DHI Harness — Design & Architecture Analysis

## 1. Architecture Summary

The system implements a **Cognitive Sandwich** pattern:

```
Human Input → [Local LLM: Intent→JSON] → [Rust Pipelines: Deterministic Exec] → [LLM: Raw→Summary] → TUI Output
                ▲ TOP BREAD                    ▲ MEAT                         ▲ BOTTOM BREAD
```

**Claimed stack**: Local Qwen 1.5B → Pure Rust tokio pipelines → NVIDIA NIM 405B  
**Actual stack**: Local Qwen 1.5B → Curl/bash subprocesses/file IO → Same Qwen 1.5B

---

## 2. Design Patterns Used

| Pattern | Location | Assessment |
|---------|----------|------------|
| **Cognitive Sandwich** | AI at boundaries (classify → execute → summarize) | Core innovation — sound concept |
| **Event-Driven** | `tokio::mpsc` + `SystemEvent` enum | Correct choice for decoupling |
| **CQRS** | Separate Input/Output queues | Good pattern, flawed placement |
| **Pipeline / Chain of Responsibility** | YAML-defined `action → summarizer` | Flexible, extensible, testable |
| **Message Bus** | Central mpsc channel routing events | Clean async decoupling from TUI |
| **Template Method** | `validate → interpolate → execute → summarize` | Consistent execution flow |
| **Strategy** | Action types (http, bash, exec, internal, open) | Polymorphic execution from config |
| **Observer** | `broadcast::channel` for Logger | Decouples logging from consumers |

---

## 3. Architecture from Excalidraw Diagram

The excalidraw (`docs/design.excalidraw`) reveals the intended architecture flow:

```
                    ┌──────────────┐
                    │  Ollama API  │◄──── (also used for summary)
                    └──────┬───────┘
                           │ classify
                           ▼
User ──► Intent Router ──► Input Queue ──► [RSS, History, Portfolio, Tracker]
                           │                   │
                      ┌────┘                   │
                      ▼                        ▼
              Pre-flite                  Output Queue ──► Display (TUI)
              (scheduled                     │
               + retry)                      ▼
                                        Persist .md
                                        ./data/<pipeline>/
```

**Key observations from the diagram:**

1. **Single Ollama API** — one box serves both classification AND summary. The design document promises separate models for each, but the diagram shows a single API. The design itself is ambiguous.

2. **Input Queue is AFTER the Intent Router** — the queue stores already-classified intents, not raw user input. This means: (a) the LLM blocks before anything is queued, (b) crashes during classification lose the input, (c) pre-flite/retry items must re-enter through the LLM.

3. **Output Queue has dual consumers** — display and persist, with no ordering or transaction guarantee.

4. **Pre-flite feeds into the Input Queue** — but the Input Queue is after the LLM, meaning retried items need re-classification.

---

## 4. Implementation Gaps (TODOs)

### 4.1 Critical Gaps

- [ ] **Bottom bread uses wrong model** — `src/summarizer/` calls local Qwen 1.5B (default model), not NVIDIA NIM 405B as designed. The diagram shows a single Ollama API for both roles.
- [ ] **No real API integrations** — All 7 pipelines (RSS, Mail, Portfolio, Ledger, Health, Observability, Search) are YAML stubs or mock data. Zero connect to external services.
- [ ] **`unsafe impl Send + Sync` for `AppState`** — `src/main.rs:74-75`. Bypasses compiler safety. Contains `sysinfo::System` behind `Mutex` which may carry non-Send state.
- [ ] **Shell injection via param interpolation** — `src/pipeline/executor.rs:207-213`. Pipeline params from LLM spliced into bash scripts with zero sanitization.
- [ ] **Input queue not persisted** — `src/queue/mod.rs:75`. `input_queue: Vec<InputQueueItem>` is in-memory only. The design preserves this for crash recovery but it's the opposite of what's implemented.
- [ ] **HTTP via `curl` subprocess** — `src/pipeline/executor.rs:289-311`. `reqwest` is in Cargo.toml but unused.

### 4.2 Architecture Gaps

- [ ] **Pipeline execution blocks async runtime** — `src/pipeline/executor.rs:146-151`. `block_in_place` + nested `block_on`. All curl/bash/exec is synchronous inside tokio tasks.
- [ ] **No graceful shutdown** — `src/main.rs:422-430`. Background tokio tasks dropped on quit. In-flight LLM requests abandoned.
- [ ] **No timeout on LLM calls** — `src/intent.rs:54-88` and `src/summarizer/mod.rs:36-66`. No timeout on Ollama calls.
- [ ] **No backpressure on event channel** — `src/main.rs:156`. `mpsc::channel::<SystemEvent>(100)`. Fast ticks + slow pipeline = silent drops.
- [ ] **Output queue file grows unbounded** — `src/queue/mod.rs:141-145`. Every execution appends to `output.json`. No rotation, TTL, or size limit.
- [ ] **No offline/fallback mode** — `src/main.rs:288-313`. If Ollama is down, the system is paralyzed. No regex keyword matching as fallback.
- [ ] **No pipeline composition** — Pipelines are single-step only. No chain: `RSS → extract → search → summarize`.
- [ ] **No circuit breaker for LLM** — Single point of failure on every command's critical path.

### 4.3 Design Contradictions (Design vs Reality)

| Design Doc Says | Code/Diagram Shows | Impact |
|----------------|-------------------|--------|
| NVIDIA NIM 405B for synthesis | Same Qwen 1.5B for both layers | Sandwich is two thin slices of same bread |
| Input Queue for crash recovery | Input Queue is in-memory, after LLM | Crash = total loss of pending commands |
| Always persist to SQLite | File-based JSON only | No indexing, no concurrent access |
| Async non-blocking pipelines | Sync curl/bash with `block_in_place` | UI freezes during execution |
| 7 running daemons | 0 daemons, hardcoded status strings | Pipeline status in TUI is fake |
| STT with Whisper, TTS with Piper | Not implemented | Voice interface absent |
| Persistent queue for retry | Input queue is in-memory Vec | Retry doesn't survive restart |

### 4.4 State & Data Gaps

- [ ] **No SQLite** — Mandated in design doc. Everything uses file-based JSON.
- [ ] **Dual mutex strategy** — `StdMutex` for UI state locked in async context. `TokioMutex` for queue/app state. Deadlock risk.
- [ ] **Data duplication** — `state.logs` and `LogState` both store logs. `output_history` duplicates the queue. Will diverge under load.
- [ ] **No STT/TTS** — Whisper and Piper not started.
- [ ] **All user input logged to disk** — `src/intent.rs:123-133`. Every query written to `.data/intent/<timestamp>.md`. Privacy concern.
- [ ] **No auth/secrets management** — `HttpAuth` stores header/value in plaintext YAML. No env var interpolation.

### 4.5 Pipeline-Specific Gaps

- [ ] **Pipeline status is hardcoded** — `src/main.rs:116-121`. No pipeline daemon actually runs. Status is cosmetic.
- [ ] **No pipeline isolation** — All pipelines share the same process/runtime. One crash takes down everything.
- [ ] **No tests** — `tests/` directory is empty. 4 existing tests produce mock output.

---

## 5. Design Flaws (Architecture-Level Problems)

These are problems with the **design itself**, not the implementation:

### 5.1 Input Queue is After the LLM — Defeats Its Purpose

```
User ──► Intent Router (LLM) ──► Input Queue ──► Pipeline
                                    ▲
                              Pre-flite (retry)
```

The queue stores *already-classified intents* instead of *raw user input*. This means:
- The LLM blocks before anything can be queued
- Crash during LLM call = input is lost (queue never sees it)
- Pre-flite/retry items must re-enter through the LLM (pointless — they already have known intents)
- The queue can't absorb bursts because the bottleneck (LLM) is before it

**Correct design**: Queue raw input before the LLM, with a bypass lane for known intents:

```
User ──► Input Queue ──► Intent Router ──► Pipeline
             ▲                 ▲
             │            Pre-flite (bypasses LLM)
      Keyword match (fast path)
```

### 5.2 Single LLM Contradicts the Sandwich

The excalidraw shows one "Ollama API" box feeding both the Intent Router (top bread) AND receiving synthesis output (bottom bread). The design document explicitly promises different models (1.5B vs 405B), but the architecture diagram draws a single API. This is a design-level contradiction.

**The sandwich has two thin slices of the same bread.** If the design truly intends different models, the diagram must show two separate LLM connections with different capabilities and cost profiles.

### 5.3 No LLM Bypass for Trivial Commands

Every command passes through the LLM for intent classification. There's no:
- Regex-based fast path ("quit", "help", "status")
- Cached intent lookup for repeated commands
- Keyword matching before the LLM call

This means even "log $5 for food" takes 500ms-2s for the LLM round-trip before any action happens.

### 5.4 Output Queue Has No Transaction Guarantee

```
Output Queue ──► Display (TUI)
             ──► Persist .md file
```

Two independent consumers, no ordering, no atomicity. Crash between the two = inconsistent state.

### 5.5 Pre-flite (Retry/Scheduler) is Undefined

The diagram shows "Pre-flite (scheduled + retry)" → Input Queue, but:
- No scheduling mechanism
- No retry policy (backoff, max attempts, TTL)
- No deduplication
- Feeds into queue that's AFTER the LLM, forcing re-classification

### 5.6 No Pipeline Composition

Pipelines are single-step: `action → output`. No chaining, no branching. You cannot:
- Fetch RSS → extract URLs → search each
- Check portfolio → format → append to daily brief
- Chain pipeline outputs as pipeline inputs

### 5.7 TUI and Pipeline Runtime Share the Same Event Loop

The async runtime runs both `terminal.draw()` and pipeline tasks. Blocking pipeline actions can starve the UI render cycle.

### 5.8 No Mechanism for Pipelines to Report Status

`SystemEvent::PipelineUpdate` exists but nothing emits it. Pipeline status is a hardcoded Vec displayed in the TUI. There's no design for pipelines to push progress, errors, or completion back to the UI.

---

## 6. Risk Assessment

```
Cognitive Sandwich concept:       8/10  (genuinely innovative architectural insight)
      Design coherence:            4/10  (CQRS placement undermines queues, single LLM contradicts sandwich)
      Implementation fidelity:     2/10  (core promises unmet: NVIDIA NIM, SQLite, daemons, async)
      Code quality:                6/10  (clean structure, unsafe shortcuts, no tests)
      Resilience:                  2/10  (no fallback, no circuit breaker, no isolation)
      Production readiness:        1/10  (no auth, no graceful shutdown, shell injection risk)
```

### Summary

The Cognitive Sandwich concept is worth pursuing. The CQRS queue pattern is sound. But there are **two design-level contradictions** that need to be resolved before any implementation work:

1. **Queue placement** — move it before the LLM to absorb bursts and enable crash recovery
2. **Single LLM** — resolve the ambiguity: either commit to one model (and design accordingly) or implement two distinct LLM layers as the document promises

The excalidraw diagram is more honest than the design doc — it shows one Ollama API for both roles. The design doc should be updated to match either the diagram or the ambition.
