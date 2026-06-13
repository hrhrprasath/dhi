# AGENTS.md - DHI Harness Development Guide

This file provides guidance for AI agents working on the DHI Harness codebase.

## 1. Project Overview

- **Name:** dhi_harness
- **Language:** Go (1.26)
- **Type:** Terminal UI (TUI) using Bubble Tea v2 + Lip Gloss v2
- **Runtime:** Go runtime with Bubble Tea event loop
- **Target:** Mac Mini M4 16GB Unified Memory

### Background & Architecture

DHI is a Terminal AI OS inspired by Her (movie), Batman AI, and Jarvis/Ironman. It uses the **Cognitive Sandwich** pattern:
- **Top Bread (Intent):** Local 1B/3B model (Qwen via Ollama) translates human input → strict JSON with confidence score
- **Meat (Execution):** Pure Go pipelines (event-driven, zero AI tool calls) hit APIs/scrapers via bash scripts or internal handlers
- **Bottom Bread (Synthesis):** Same local model (Qwen via Ollama) converts raw data → human summary

**Pipelines:** stocks, search_inline, search_browser, browse, music (external YAML-defined), todo, system, schedule (internal Go-defined)

**Task System:** Universal unit of work replacing input/output queues. Tasks are one-shot (execute immediately) or scheduled (cron-based). A 60s looper scans for due scheduled tasks.

---

## 2. Build, Run, and Test Commands

### Building
```bash
go build ./...               # Build all packages
go build -o build/dhi ./cmd/dhi  # Build binary to build/ dir
```

### Running
```bash
go run ./cmd/dhi        # Run from source
go build ./cmd/dhi && ./dhi  # Build + run binary
```

### Testing
```bash
go test ./...                    # Run all tests
go test ./internal/pipeline      # Test specific package
go test -v ./...                 # Verbose output
```

### Linting and Formatting
```bash
go fmt ./...         # Format all Go files
go vet ./...         # Static analysis
golangci-lint run    # Full lint (if configured)
```

---

## 3. Code Style Guidelines

### 3.1 Imports

Organize imports in groups with blank lines between:
1. Standard library (`fmt`, `strings`, `time`, etc.)
2. External packages (`charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `gopkg.in/yaml.v3`)
3. Internal packages (`dhi_harness/internal/...`)

```go
import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"dhi_harness/internal/config"
	"dhi_harness/internal/intent"
)
```

### 3.2 Formatting

- Use `go fmt` (tab indentation, 8-char tabs)
- Maximum line length: 100 characters (enforced manually)
- Use trailing commas in multi-line struct/function literals

### 3.3 Naming Conventions

- **camelCase**: variables, function parameters, unexported fields
- **PascalCase**: exported types, functions, struct fields
- **UPPER_CASE**: constants (iota-based const blocks)
- Unexported package-level functions/types don't need `_` prefix

### 3.4 Error Handling

- Return `error` from functions (not panics)
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Log errors with `logger.Error(component, msg)` — never log secrets

### 3.5 TUI Patterns (Bubble Tea v2)

- Model struct holds all state; `Update()` dispatches on message types
- View returns `tea.View{Content, AltScreen}` — use AltScreen for full terminal
- `tea.WindowSizeMsg` is the single source of layout dimensions
- Use `tea.Batch()` to return multiple commands
- Use `tea.Tick()` for periodic re-renders (never `time.Sleep` in Update)
- Viewports (`viewport.Model`) for scrollable panels
- Textinput for command input
- Progress bars for CPU/RAM/GPU metrics

### 3.6 Task System

All user inputs create a `task.Task` via `taskMgr.AddOnce()`. The `processTasks` loop (200ms poll) picks up pending tasks, classifies via Ollama, and executes. Scheduled tasks are created with `taskMgr.AddScheduled()` and checked by a 60s looper.

```go
// Task statuses
StatusPending    // ready to classify+execute
StatusProcessing // being processed
StatusCompleted  // done
StatusFailed     // error, retries exhausted
StatusWaiting    // scheduled, waiting for time
```

### 3.7 Comments

- Do NOT add comments unless explicitly requested
- Code should be self-documenting

### 3.8 Safety and Security

- Never log secrets, keys, or credentials
- Use `.env` files for sensitive configuration (never commit)
- All bash scripts handle interpolation via the executor (params mapped from LLM JSON)

---

## 4. Key Files

- `cmd/dhi/main.go` — Application entry point, wires all dependencies
- `tui/types.go` — Model struct, message types, styles
- `tui/model.go` — NewModel, Init, Update, eventPoller, helpers
- `tui/view.go` — View, render* methods, orb animation, content refresh
- `tui/update.go` — Key handling, background loops, processTasks, pipeline/conversation handlers
- `internal/intent/router.go` — Ollama classify + chat, ParsedIntent with confidence
- `internal/pipeline/executor.go` — FindAndExecute, action dispatch (bash, internal, http, exec, open)
- `internal/pipeline/handlers.go` — Internal handlers (todo, schedule)
- `internal/pipeline/manager.go` — Pipeline definitions, YAML loading, defaults
- `internal/task/manager.go` — Task CRUD, persistence, 60s looper
- `internal/task/time.go` — TimeParser interface + DefaultParser (natural language → time)
- `internal/prompt/templates.go` — Classify/Conversation/Synthesis prompts, BuildSchema
- `config/pipelines.yaml` — External pipeline definitions (stocks, search*, browse, music)
- `PENDING.md` — Remaining work items

---

## 5. Architecture Overview

```
User Input → taskMgr.AddOnce() → processTasks() (200ms poll)
  → Route() via Ollama classify
    → EXECUTE_* → handlePipeline → FindAndExecute → output
    → CONVERSATION → handleConversation → Chat via Ollama
    → confidence < 0.8 → fallback to conversation

Startup: taskMgr.CheckSchedule() — pick up due scheduled tasks
Looper (60s): CheckSchedule() — activate waiting tasks when ScheduledFor <= now
Recurring tasks: after execution, compute next ScheduledFor, set back to waiting
```

## 6. Testing Strategy

- Write unit tests for pure functions (parser, prompt output, time parsing)
- Use `_test.go` files in the same package
- Temporary directories for filesystem tests
- Test task manager CRUD + persistence

## 7. Performance

- Minimize allocations in hot paths (render loop, tick handler)
- Viewport content is recomputed only when state changes (not on every render)
- Orb animation uses fast math primitives (no allocations per cell)
- Consumer loop polls at 200ms — avoids busy-waiting
- Avoid blocking in Update (use goroutines for LLM calls, file IO)
