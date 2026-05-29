# Missing Features (Go Migration)

Tracked from the Rust → Go migration. Items here are known gaps
between `feat/go-migration` and the original Rust `main` branch.

## TUI / Rendering

- [ ] **Breathing animation** — Rust had a 10-frame diamond-pulse animation for the DHI
      header. Go uses a Moon spinner instead.
- [ ] **Log word-wrap** — Rust word-wraps log lines at panel width. Go's viewport
      component handles wrapping via `SoftWrap` field — needs `logView.SoftWrap = true`.
- [ ] **Scrollbar** — Rust's log panel draws a scrollbar when focused. Go's viewport
      doesn't render a scrollbar natively; would need custom rendering.
- [ ] **NVIDIA placeholder** — Rust showed `○ NVIDIA Coming Soon` in metrics. Not migrated.

## Handlers

- [ ] **`conversation_continue`** — Go handler is a stub that returns a formatted info
      string. Rust version reads/writes `.data/conversation/context.json` for
      topic-specific context persistence.
- [ ] **`memory_grep` related-term extraction** — Rust's version does 3-level search:
      Level 1 = direct matches, Level 2 = extracted related terms, Level 3 = timeline
      synthesis. Go's version only does depth-based recursive file search.

## Testing

- [ ] **No Go tests** — Rust had 4+ tests (prompt output inspection, pipeline
      execution, pipeline name assertions). Go has zero `_test.go` files.

## Resolved

- [x] **Logger file persistence** — Added `logger.SetFile(".data/dhi.log")` in main.go
- [x] **CPU/RAM metrics** — Added gopsutil v3, SystemService returns real data every 1s
- [x] **GPU status** — Shows chipset model from `system_profiler SPDisplaysDataType`
- [x] **Weather removed** — Purged from SystemService, Model, and TUI
- [x] **Logger subscriber** — `loggerForwardLoop()` subscribes and sends all log entries to TUI
- [x] **Tick re-issue** — `tickMsg` handler returns `tea.Tick()` so CPU/RAM refresh every second
- [x] **Initial health check** — Immediate `router.HealthCheck()` goroutine on startup
- [x] **textinput.Focus() cmd** — Stored and returned from Init()
- [x] **Viewport overflow** — Viewport widths set to `rw - 2` to match border inner width
- [x] **Initial log seed** — Viewport prepopulated so it's never empty
- [x] **Unused code removed** — `sync.Mutex`, `textColor`, `blueColor`, `weather` field
