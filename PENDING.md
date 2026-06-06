# Pending Items

Current state of the DHI Harness Go migration. Checkmarks track completion.

---

## TUI / Rendering

- [ ] **Log word-wrap** — `m.logView.SoftWrap = true` (one-liner in `tui/model.go:NewModel`)
- [ ] **NVIDIA placeholder** — Show `○ NVIDIA Coming Soon` in the system metrics panel (Rust had it)
- [ ] **Scrollbar in log panel** — Rust drew a scrollbar when focused; Go's viewport doesn't render one natively
- [ ] **Scheduled task list in TUI** — TASKS panel only shows counts (pending/waiting); no list of upcoming schedules with their fire times
- [ ] **Timer display for schedules** — Show countdown / "in 4h23m" next to each waiting task in the TUI

## Task / Schedule System

- [ ] **Scheduled task history** — Completed scheduled tasks are invisible; no way to see past executions in TUI
- [ ] **LLM multi-schedule prompt tuning** — The `schedules` JSON array param needs the classify prompt to show an example so the LLM learns to output it for multi-step requests like "play song at 9am and news at 9:30"
- [ ] **Schedule update partial** — `schedule update` currently requires at least `schedule_id`; updating only `when` or only `command` works but isn't well-tested
- [ ] **Schedule delete confirmation** — No visual feedback beyond the log message
- [ ] **Task persistence validation** — `tasks.json` accumulates completed tasks forever; consider a TTL/cleanup mechanism or max-limit truncation

## Pipeline Handlers

- [ ] **`conversation_continue`** — Go handler is a stub returning a formatted info string. Rust version read/wrote `.data/conversation/context.json` for topic-specific context persistence
- [ ] **`memory_grep` related-term extraction** — Rust had 3-level search (direct matches → extracted related terms → timeline synthesis). Go only does depth-based recursive file search
- [ ] **`search_inline` result quality** — Currently fetches first 3 DuckDuckGo results in parallel; sometimes returns low-quality snippets. Consider increasing to 5 results or adding a relevance filter

## Prompt / LLM

- [ ] **Schedule pipeline extract_hint coverage** — The `when` param `extract_hint` may not be expressive enough for the 1B/3B model to reliably extract scheduling intent from phrases like "remind me to X at Y"
- [ ] **Multi-schedule example in ClassifyPrompt** — Need an example showing `"schedules"` JSON array usage so the LLM learns to emit it
- [ ] **Confidence threshold tuning** — Hardcoded at 0.8; some edge cases may need per-pipeline or adaptive thresholds

## Testing

- [ ] **No Go tests** — Zero `_test.go` files. Rust had 4+ tests (prompt output, pipeline execution, name assertions). Deferred by preference.

---

## Recently Completed (for reference)

- [x] Task system replacing queue — `internal/task/` package with CRUD, persistence, 60s looper
- [x] Schedule pipeline — `EXECUTE_SCHEDULE` with add/list/delete/update + multi-schedule via `schedules` JSON array
- [x] `TimeParser` interface + `DefaultParser` — stdlib-only natural language time parser (handles "in 5 min", "daily at 9am", "tomorrow at 5pm", etc.)
- [x] Confidence-based routing — actions below 0.8 confidence fall back to conversation
- [x] Pipeline weight hints — each pipeline has a `weight` shown to the LLM for disambiguation
- [x] Orb animation — random spiking flow replaces old moon spinner
- [x] All pipeline definitions moved to YAML (external) or Go defaults (internal: todo, system, schedule)
- [x] TUI split into `types.go`, `model.go`, `view.go`, `update.go`
