# DHI OS: Terminal-Based Deterministic Harness — Design Document

## 1. High-Level Architecture Overview

DHI is a lightweight, high-performance, command-line AI operating system for Apple Silicon (Mac Mini M4, 16GB Unified Memory). It uses the **Cognitive Sandwich** pattern: AI at the boundaries (intent classification and summarization), pure Go deterministic execution in the middle.

### Core Stack
- **Frontend:** Terminal UI (TUI) via Bubble Tea v2 + Lip Gloss v2
- **Backend:** Go (1.26) — standard library, gopsutil v3 for system metrics
- **State Management:** JSON file persistence (`.data/` directory)
- **Cognitive Layer:**
  - Top Bread (Intent): Qwen 1B/3B via local Ollama — translates human input → strict JSON with confidence score
  - Bottom Bread (Synthesis): Same local Ollama model — converts raw pipeline output → human summary

### Design Principles
- **Zero LLM tool calls** — AI only translates state; pipelines are pure Go
- **Event-driven** — goroutines communicate over channels; Bubble Tea event loop drives updates
- **Deterministic execution** — pipelines run predictable Go code or shell scripts
- **Crash recovery** — task queue persists to JSON; pending tasks resume on startup

---

## 2. Visual Architecture

```
User ──► TUI (Bubble Tea)
              │
              ▼
        taskMgr.AddOnce()  ──►  processTasks()  (200ms poll)
              │                              │
              ▼                              ▼
        ┌──── Ollama API ────┐        taskMgr.CheckSchedule()  (60s looper)
        │  classify → JSON   │               │
        └────────┬───────────┘               ▼
                 │                    Activate due waiting tasks
          ┌──────┴──────┐
          ▼              ▼
   EXECUTE_*       CONVERSATION
          │              │
          ▼              ▼
   FindAndExecute    Chat()
          │              │
          ▼              ▼
   Pipeline output ──► convMsg → TUI display
          │
          ▼
   (optional) Summarizer → human summary
```

### Pipeline Execution Flow

```
EXECUTE_STOCKS → FindAndExecute()
  → resolve params → validate → executeAction
    → bash: "scripts/stock {company}"
    → output → (optional) summarize → reply

EXECUTE_SCHEDULE → handleSchedule()
  → parse "when" via TimeParser
  → AddScheduled(command, scheduledFor, interval)
  → looper fires when due → re-enters classify+execute flow
```

### Task Lifecycle

```
User input
  → AddOnce(rawInput) → status: pending
  → processTasks() picks up → status: processing
  → classify → execute
  → on success → status: completed
  → on error → retry (max 3) → status: failed

Scheduled task
  → AddScheduled(rawInput, time, interval) → status: waiting
  → looper: ScheduledFor <= now → status: pending
  → processTasks() picks up → same flow as above
  → if recurring → compute next ScheduledFor → back to waiting
```

---

## 3. TUI Layout

The TUI is split into a left panel (fixed 28 chars wide) and a right panel (remaining width, min 40 chars).

### Left Panel (28 wide)

```
┌──────────────────────────────┐
│        SYSTEM METRICS        │  ← CPU/RAM/GPU progress bars + Ollama status
│ CPU: ████████░░░░ 42%        │
│ RAM: ██████████░░ 60%        │
│ GPU: ██░░░░░░░░░░ 12%        │
│ ● Ollama READY               │
├──────────────────────────────┤
│           TASKS              │  ← pending/waiting task counts
│ Pending: 2                   │
│ Waiting: 5                   │
├──────────────────────────────┤
│         PIPELINES            │  ← pipeline status from Viewport
│ todo         IDLE            │
│ stocks       IDLE            │
│ schedule     RUNNING         │
├──────────────────────────────┤
│           DHI OS             │  ← host info, version, uptime
│ v0.1.0                       │
│ Host: mac-mini               │
│ Kernel: 24.3.0               │
│ Up: 142m                     │
├──────────────────────────────┤
│                              │  ← orb animation (random spiking flow)
│    ░▒▓█▓▒░ ░▒▓              │     bottom-to-top intensity
│   ░▒▓███▓▒░                 │     warm amber glow
│   ░▒▓███▓▒░                 │
│   ░▒▓█▓▒░                   │
│  ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁   │
└──────────────────────────────┘
```

### Right Panel (variable width)

```
┌────────────────────────────────────────────┐
│                  LOGS                      │  ← scrollable, 20% height
│ [ROUTER][INFO] User: check AAPL            │
│ [CONSUMER][INFO] Classifying...             │
│ [DHI][INFO] AAPL is trading at...          │
├────────────────────────────────────────────┤
│              CONVERSATION                   │  ← scrollable, 80% height
│ You: what is the stock price of AAPL?      │
│                                            │
│ DHI: Apple (AAPL) is currently trading...  │
│ ──────────────────────────────────────     │
├────────────────────────────────────────────┤
│ > Type a command... (Tab to focus) ● ONLINE│  ← text input + connection status
└────────────────────────────────────────────┘
```

### Focus Cycling

Tab cycles through the three right-panel panels: LOGS → INPUT → CONVERSATION → LOGS.
Focused panel gets accent-colored border (orange `#FFA500`).
Up/Down/PgUp/PgDown scroll when LOGS or CONVERSATION are focused.

---

## 4. Pipeline System

### Definitions

Pipelines are either:
- **External** — defined in `config/pipelines.yaml` with bash/http/exec actions
- **Internal** — hardcoded Go handlers (todo, system, schedule)

### External Pipelines (YAML)

| Pipeline | Action | Summarize | Ignore Context |
|----------|--------|-----------|----------------|
| `stocks` | `scripts/stock {company}` | no | — |
| `web_search` | `scripts/web_search {query}` | yes | yes |
| `site_search` | `s --provider {site} {query}` | no | — |
| `fetch_url` | `scripts/fetch_url {url}` | yes | no |
| `music` | `scripts/spotify {query}` | no | — |

### Internal Pipelines (Go Code)

| Pipeline | Handler | Description |
|----------|---------|-------------|
| `todo` | `handleTodo` | CRUD for user todo list (add/list/complete) |
| `system` | `systemMetrics` | Kernel/hostname/uptime info |
| `schedule` | `handleSchedule` | Create/list/delete/update scheduled tasks |

### Action Types

| Type | Method | Use Case |
|------|--------|----------|
| `bash` | `bash -c "{script}"` | Shell scripts, APIs via curl |
| `http` | `curl -s -X {method} {url}` | REST API calls |
| `exec` | `exec.Command(name, args...)` | Local CLI tools |
| `open` | `open {target}` | Open URLs/apps on macOS |
| `internal` | Go handler function | Todo, system, schedule |

### Confidence-Based Routing

The LLM outputs a `confidence` field (0.0–1.0) with each classification:
- `>= 0.8` → execute the classified event
- `< 0.8` → fall back to conversation (Chat)
- Pipelines also have a `weight` (0.3–0.9) as a priority hint for the LLM

---

## 5. Task System

Replaces the old input/output queue architecture. Tasks are persisted before any LLM call — crash-safe by design.

### Task Statuses

| Status | Meaning |
|--------|---------|
| `pending` | Ready to be classified and executed |
| `processing` | Currently being classified or executed |
| `completed` | Successfully finished |
| `failed` | Error (max 3 retries) |
| `waiting` | Scheduled, waiting for `ScheduledFor <= now` |

### Task Manager

- **Persistence:** `.data/tasks/tasks.json`
- **One-shot tasks:** `AddOnce(rawInput)` — created on user input, executed immediately
- **Scheduled tasks:** `AddScheduled(rawInput, time, intervalSec)` — created by `EXECUTE_SCHEDULE` handler
- **Looper:** `StartLooper(60s)` — goroutine that checks for due waiting tasks
- **Startup:** `CheckSchedule()` — activates any waiting tasks that are past their time

### Time Parsing

The `TimeParser` interface handles natural language time expressions:
- `in 5 minutes` — relative duration
- `at 5pm` — absolute time (today or tomorrow if past)
- `tomorrow at 9am` — next day at time
- `daily at 9am` — recurring daily
- `every 30 minutes` — recurring interval
- `@every 5m` — shorthand interval

---

## 6. Data Persistence

```
.data/
├── dhi.log               # Logger output
├── conversation/
│   └── summary.json      # Full conversation turns (up to 50)
├── todos/
│   └── todos.json        # User todo list
├── tasks/
│   └── tasks.json        # All tasks (pending, completed, scheduled)
├── intent/
│   └── <timestamp>.md    # Raw classify prompt dumps
└── <pipeline>/
    └── <timestamp>.md    # Pipeline output persistence
```

---

## 7. Key File Map

| Layer | File | Role |
|-------|------|------|
| Entry | `cmd/dhi/main.go` | Dependency wiring, program start |
| TUI | `tui/types.go` | Model, messages, styles |
| TUI | `tui/model.go` | Init, Update, helpers |
| TUI | `tui/view.go` | All rendering + orb animation |
| TUI | `tui/update.go` | Key handling, loops, task processing |
| Intent | `internal/intent/router.go` | Ollama classify + chat |
| Prompt | `internal/prompt/templates.go` | Classify/conversation/synthesis prompts |
| Pipeline | `internal/pipeline/manager.go` | YAML loading, pipeline definitions |
| Pipeline | `internal/pipeline/executor.go` | Action dispatch, param interpolation |
| Pipeline | `internal/pipeline/handlers.go` | Internal handlers (todo, system, schedule) |
| Task | `internal/task/manager.go` | Task CRUD, persistence, looper |
| Task | `internal/task/time.go` | Natural language time parsing |
| Config | `config/pipelines.yaml` | External pipeline definitions |

---

## 8. Implementation Phases (Completed)

- **Phase 1: TUI & Boilerplate** — Bubble Tea event loop, split-pane layout, viewports, textinput, Tab focus cycling, resize handling
- **Phase 2: Intent Layer** — Ollama REST classify + chat, ParsedIntent with confidence, guardrailed prompts
- **Phase 3: Core Pipelines** — YAML-defined pipelines, bash/http/exec/open/internal actions, param interpolation
- **Phase 4: Task System** — Replace input/output queues with unified task manager, scheduled tasks, 60s looper
- **Phase 5: Internal Handlers** — todo (CRUD), system_metrics, schedule (CRUD + multi-schedule)
- **Phase 6: TUI Polish** — Word wrapping, markdown stripping, orb animation, focused border styling
