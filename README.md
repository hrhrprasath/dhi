# DHI Harness

Terminal AI OS — Cognitive Sandwich pattern: LLM-classified intent → deterministic Go pipelines → LLM-synthesized response.

## Quick Start

```bash
# Requirements: Go 1.26+, Ollama running locally with a model (e.g. qwen2.5:1.5b)

go build -o build/dhi ./cmd/dhi && ./build/dhi
```

## Project Structure

```
├── cmd/dhi/main.go        # Entry point, wires dependencies
├── internal/
│   ├── config/            # Config loading (YAML)
│   ├── intent/            # LLM router: classify + chat via Ollama
│   ├── learn/             # Self-learning feedback loop (weights, calibration, keywords)
│   ├── pipeline/          # Pipeline definitions, executor, internal handlers
│   ├── prompt/            # LLM prompt templates
│   ├── services/          # Logger, system metrics
│   ├── summarizer/        # LLM summarization of pipeline output
│   ├── task/              # Task CRUD, persistence, scheduled task looper
│   └── todo/              # Todo CRUD
├── tui/                   # Bubble Tea TUI (types, model, view, update)
├── config/pipelines.yaml  # Pipeline definitions (stocks, web_search, site_search, fetch_url, music)
└── docs/                  # Design docs, architecture analysis, user guide
```

## Pipelines

| Pipeline | Type | Weight | Description |
|----------|------|--------|-------------|
| stocks | bash | 0.8 | Stock price lookup |
| web_search | bash | 0.7 | Web search inline with summaries |
| site_search | bash | 0.4 | Site-specific browser search |
| fetch_url | bash | 0.4 | Fetch URL content inline |
| music | bash | 0.5 | Spotify control |
| todo | internal | 0.9 | Todo/reminder CRUD |
| system | internal | 0.6 | System metrics |
| schedule | internal | 0.8 | Scheduled task CRUD |

## Controls

| Key | Action |
|-----|--------|
| `Enter` | Submit command |
| `Tab` | Cycle focus / autocomplete `/` commands |
| `↑/↓` | Scroll / recall last input (when empty) |
| `Ctrl+C` / `Ctrl+D` | Quit |
| `/good` | Rate last response positive |
| `/bad` | Rate last response negative |
| `/log` | Toggle log popup |
