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

## 2. Implementation Gaps (TODOs)

### Critical Gaps

- [ ] **Bottom bread uses wrong model** — `src/summarizer/` calls local Qwen 1.5B (default model), not NVIDIA NIM 405B as designed. Synthesis quality is identical to intent parsing.
- [ ] **No real API integrations** — All 7 pipelines (RSS, Mail, Portfolio, Ledger, Health, Observability, Search) are YAML stubs or mock data. Zero connect to external services.
- [ ] **`unsafe impl Send + Sync` for `AppState`** — `src/main.rs:74-75`. Bypasses compiler safety guarantees. Contains `sysinfo::System` behind `Mutex` which may carry non-Send state. Future refactors risk UB.
- [ ] **Shell injection via param interpolation** — `src/pipeline/executor.rs:207-213`. Pipeline params from LLM output are spliced into bash scripts/commands with zero sanitization.
- [ ] **Input queue not persisted** — `src/queue/mod.rs:75`. `input_queue: Vec<InputQueueItem>` is in-memory only. Crash = total loss of pending commands. Design doc promises crash recovery.
- [ ] **HTTP via `curl` subprocess** — `src/pipeline/executor.rs:289-311`. Spawns `curl` for every HTTP pipeline. `reqwest` is in Cargo.toml but unused. Slower, no connection pooling, macOS assumption.

### Architecture Gaps

- [ ] **Pipeline execution blocks async runtime** — `src/pipeline/executor.rs:146-151`. `block_in_place` + nested `block_on` for summarizer. All HTTP/bash/exec actions are synchronous inside tokio tasks. UI freezes during execution.
- [ ] **No graceful shutdown** — `src/main.rs:422-430`. Background tokio tasks (intent routing, pipeline execution) are dropped on quit. In-flight LLM requests abandoned; queue may be inconsistent.
- [ ] **No timeout on LLM calls** — `src/intent.rs:54-88` and `src/summarizer/mod.rs:36-66`. Both intent routing and synthesis calls to Ollama have no timeout. A hanging model blocks the pipeline indefinitely.
- [ ] **No backpressure on event channel** — `src/main.rs:156`. `mpsc::channel::<SystemEvent>(100)`. Fast ticks + slow pipeline = silent event drops.
- [ ] **Output queue file grows unbounded** — `src/queue/mod.rs:141-145`. Every execution appends to `output.json`. No rotation, TTL, or size limit.
- [ ] **No offline/fallback mode** — `src/main.rs:288-313`. If Ollama is down, the system is completely paralyzed. No keyword-based intent matching as fallback.

### State & Data Gaps

- [ ] **No SQLite** — Mandated in design doc. Everything uses file-based JSON instead. No indexing, no concurrent access safety, no querying.
- [ ] **Dual mutex strategy** — `StdMutex` for UI state locked in async context (`src/main.rs:239-420`). `TokioMutex` for queue/app state. StdMutex held across event handler can block the async thread.
- [ ] **Data duplication between `state.logs` and `LogState`** — `src/main.rs:44, 62-63`. Two copies of log data updated in different paths — will diverge under load.
- [ ] **No STT/TTS** — `Whisper` and `Piper` mentioned in design doc. Not started.
- [ ] **All user input logged to disk** — `src/intent.rs:123-133`. Every query written to `.data/intent/<timestamp>.md`. Privacy concern; thousands of tiny files accumulate.

### Pipeline-Specific Gaps

- [ ] **Pipeline status is hardcoded** — `src/main.rs:116-121`. Active pipelines display static labels ("Sleeping", "Executing"). No background daemons actually run.
- [ ] **No pipeline isolation** — All pipelines share the same process/tokio runtime. One crash or memory leak takes down everything.
- [ ] **Auth/credentials exposed in YAML** — `src/pipeline/mod.rs:51-55`. `HttpAuth` stores header/value in plaintext in config file. No secrets management.
- [ ] **No tests** — `tests/` directory is empty. `src/pipeline/executor.rs:399-436` has 4 tests that depend on external config file and produce mock output.

---

## 3. Design & Architecture Analysis

### 3.1 What Gets Right

**The Cognitive Sandwich concept is genuinely good.** Keeping AI strictly at the translation boundaries (human→structured, raw→summary) while executing with deterministic code is the correct architectural insight. It mirrors how production AI systems are evolving:
- OpenAI Structured Outputs / Anthropic Tool Use → parse intent into constrained JSON
- Deterministic backend code → execute without AI intervention
- AI for final formatting → synthesize for human consumption

**Event-driven architecture via tokio mpsc is sound.** The decoupling of input (crossterm thread) → event loop (async main) → background tasks (tokio spawn) is a clean pattern.

**YAML-defined pipelines are extensible.** Separating pipeline config from code allows adding new actions without recompilation.

### 3.2 The Cognitive Contradiction

The design documents a "thick sandwich":

```
Thin AI (1.5B intent parser)
Thick Meat (pure Rust, zero AI)
Thick AI (405B synthesizer)
```

But implements a "thin sandwich":

```
Thin AI (1.5B intent parser)
Thin Meat (curl, bash, blocking IO — also calls AI summarizer per-action)
Same Thin AI (1.5B synthesizer)
```

**The middle layer calls the AI on every pipeline action** (`executor.rs:145, 165, 181`), violating the "zero AI tool calls" principle. Even logging a $5 expense incurs an LLM round-trip.

### 3.3 Real vs Promised

| Feature | Promised | Reality |
|---------|----------|---------|
| NVIDIA NIM synthesis | Llama 405B | Qwen 1.5B (same as parser) |
| 7 daemons | 7 isolated background pipelines | 0 running daemons, YAML stubs |
| SQLite state | Indexed, queryable DB | Flat JSON files |
| STT/TTS | Voice in/out | Not implemented |
| Async pipelines | Non-blocking tokio | Synchronous curl/bash in block_on |
| Crash recovery | Queue replay | Input queue is in-memory only |

### 3.4 Risk Assessment

```
Cognitive Sandwich concept:       8/10  (genuinely innovative)
      Implementation fidelity:     2/10  (core promise unmet)
      Code quality:                6/10  (clean structure, unsafe shortcuts)
      Test coverage:               1/10  (4 tests, all mock)
      Production readiness:        1/10  (no auth, no isolation, no graceful shutdown)
```

### 3.5 Verdict

The architecture document describes what this **should** be. The codebase is what it **currently** is. The gap between them is the key risk — the design promises a system that doesn't exist yet, which can lead to misaligned expectations.

The Cognitive Sandwich pattern is worth pursuing. But the current implementation is best understood as a well-structured prototype, not the system described in `design.md`.
