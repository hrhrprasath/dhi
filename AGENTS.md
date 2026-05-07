# AGENTS.md - Aura Harness Development Guide

This file provides guidance for AI agents working on the Aura Harness codebase.

## 1. Project Overview

- **Name:** aura_harness
- **Language:** Rust (2021 edition)
- **Type:** Terminal UI (TUI) using ratatui + crossterm
- **Runtime:** tokio async runtime
- **Target:** Mac Mini M4 16GB Unified Memory

### Background & Architecture

Aura is a Terminal AI OS inspired by Her (movie), Batman AI, and Jarvis/Ironman. It uses the **Cognitive Sandwich** pattern:
- **Top Bread (Intent):** Local 1B/3B model (Qwen via Ollama) translates human input → strict JSON
- **Meat (Execution):** Pure Rust pipelines (event-driven, zero AI tool calls) hit APIs/scrapers
- **Bottom Bread (Synthesis):** NVIDIA NIM API (Llama 405B) converts raw data → human summary

**7 Pipelines (Daemons):** RSS, Mail/Calendar, Portfolio/Trading, Expense Ledger, Health, System Observability, Deep Research

---

## 2. Build, Run, and Test Commands

### Building
```bash
cargo build          # Debug build
cargo build --release  # Release build
```

### Running
```bash
cargo run          # Run debug build
cargo run --release  # Run release build
```

### Testing
```bash
cargo test                    # Run all tests
cargo test <test_name>        # Run specific test (exact match)
cargo test -- test_name       # Run tests matching pattern
cargo test -- --nocapture     # Show print output
cargo test -- --test-threads=1  # Run tests sequentially
```

### Linting and Formatting
```bash
cargo fmt        # Format code (run before committing)
cargo clippy     # Lint code
cargo clippy -- -D warnings  # Treat warnings as errors
cargo check      # Check compilation without building
```

---

## 3. Code Style Guidelines

### 3.1 Imports

Organize imports in groups with blank lines between:
1. Standard library (`std`, `io`, etc.)
2. External crates (`tokio`, `ratatui`, `crossterm`, `anyhow`)
3. Local modules

Use `Event as CEvent` alias to avoid name conflicts.

```rust
use anyhow::Result;
use crossterm::{
    event::{self, Event as CEvent, KeyCode},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    widgets::{Block, Borders, Paragraph},
    Terminal,
};
use std::{io, time::{Duration, Instant}};
use tokio::sync::{mpsc, Mutex};
```

### 3.2 Formatting

- Maximum line length: 100 characters
- Use 4 spaces for indentation
- Use trailing commas in multi-line function calls

### 3.3 Types

- Use explicit type annotations for function parameters and return types
- Use `Arc<Mutex<T>>` for shared state across async tasks

```rust
struct AppState {
    should_quit: bool,
    input_buffer: String,
    logs: Vec<String>,
    active_pipelines: Vec<(String, String, String)>,
}

pub enum SystemEvent {
    Input(crossterm::event::KeyEvent),
    Tick,
    SystemLog(String),
    PipelineUpdate { name: String, status: String },
}
```

### 3.4 Naming Conventions

- **Snake_case**: variables, functions, struct fields
- **PascalCase**: types, enum variants
- **SCREAMING_SNAKE_CASE**: static constants, config values

### 3.5 Error Handling

- Use `anyhow::Result` for application code
- Use `?` operator for error propagation
- Include context using `anyhow!` macro

### 3.6 Async Patterns

- Use `tokio::main` macro for async main
- Use `mpsc::channel` for message passing
- Use `Arc<Mutex<T>>` for shared state and clone tx before spawning

### 3.7 TUI Patterns (ratatui)

- Use `CrosstermBackend` for terminal backend
- Create render functions taking `&mut ratatui::Frame` and `&AppState`
- Use `Block::default().borders(Borders::ALL).title(" TITLE ")`

### 3.8 Comments

- Do NOT add comments unless explicitly requested
- Code should be self-documenting

### 3.9 Safety and Security

- Never log secrets, keys, or credentials
- Use `.env` files for sensitive configuration (never commit)
- Validate user input before processing

---

## 4. Key Files

- `src/main.rs` - Main application entry point
- `Cargo.toml` - Project dependencies
- `docs/design.md` - Full architecture specification

---

## 5. Testing Strategy

- Write unit tests for pure functions
- Test async code using `#[tokio::test]`
- Use temporary directories for filesystem tests
- Mock external services (APIs, databases)

---

## 6. Performance

- Minimize allocations in hot paths
- Use `tokio::select!` for concurrent operations
- Avoid blocking in async context (use `tokio::task::spawn_blocking`)
- Profile before optimizing