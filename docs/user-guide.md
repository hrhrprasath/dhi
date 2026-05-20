# DHI Harness User Guide

DHI (Deterministic Harness Intelligence) is a terminal-based AI operating system that routes user input through an intent parser (Ollama local LLM), executes actions via configurable pipelines, and summarizes results.

---

## Getting Started

### Running the Application

```bash
# Debug build
cargo run

# Release build
cargo run --release
```

### Controls

| Key | Action |
|-----|--------|
| `Enter` | Submit command |
| `Ctrl+C` | Quit application |
| `Tab` | Switch between Logs and Command panels |
| `↑/↓` | Scroll panel |

---

## Pipeline Configuration

Pipelines are defined in `config/pipelines.yaml`. Each pipeline defines:
- **name**: Unique identifier
- **description**: Keywords for LLM routing (what it handles)
- **params**: Parameters to extract from user input
- **action**: What to execute
- **summarizer**: Prompt for LLM to summarize output

### Pipeline Fields

| Field | Description |
|-------|-------------|
| `name` | Pipeline identifier |
| `description` | Keywords/phrases for routing (e.g., "stock, portfolio, nifty") |
| `params` | List of parameters |
| `params[].name` | Parameter name |
| `params[].required` | Whether parameter is required |
| `params[].default` | Default value if not provided |
| `action.type` | Action type (http, exec, bash, open, internal) |
| `action.*` | Action-specific fields |
| `summarizer.prompt` | Prompt for summarization LLM |

---

## Action Types

### 1. HTTP Request
```yaml
action:
  type: http
  url: "https://api.example.com/data"
  method: GET
  auth:
    header: "Authorization"
    value: "Bearer {api_key}"
```

| Field | Description |
|-------|-------------|
| `url` | Endpoint URL (use `{param}` for interpolation) |
| `method` | HTTP method (GET, POST, etc.) |
| `auth` | Optional authentication header |

### 2. Execute Command (CLI)
```yaml
action:
  type: exec
  command: "spotify-cli"
  args: ["search", "{query}"]
```

| Field | Description |
|-------|-------------|
| `command` | Command to execute |
| `args` | Arguments (use `{param}` for interpolation) |

### 3. Bash Script
```yaml
action:
  type: bash
  script: "ls -la {directory}"
```

| Field | Description |
|-------|-------------|
| `script` | Bash script to execute |

### 4. Open URL/App
```yaml
action:
  type: open
  target: "https://example.com"
```
- If target starts with `http://` or `https://` → opens in default browser
- Otherwise → opens as macOS application

---

## Built-in Pipelines

| Pipeline | Description | Example Input |
|----------|-------------|---------------|
| `portfolio` | Stock/portfolio queries | "check apple stock" |
| `rss` | RSS feed/news | "fetch techcrunch rss" |
| `search` | Web search | "search latest AI news" |
| `system` | System metrics | "show system status" |
| `expense` | Expense tracking | "spent 500 on food" |
| `health` | Health tracking | "log workout 30min" |
| `music` | Music search | "play songname" |
| `open_url` | Open URL | "open https://github.com" |
| `open_app` | Open application | "open spotify" |
| `exec_cmd` | Run CLI command | "run ls -la" |
| `bash_script` | Run bash | "bash echo hello" |
| `http_api` | API with auth | "fetch api data" |

---

## Intent Flow

```
User Input → Ollama (Intent Router) → Parse JSON → Pipeline Lookup → Execute Action → Summarize Output
```

1. **Input**: User enters command
2. **Intent Router**: Ollama parses input → `EXECUTE_PIPELINE`, `target`, `params`
3. **Pipeline Lookup**: Map event to pipeline name
4. **Execute**: Run action with interpolated params
5. **Summarize**: Use summarizer prompt for output

### Fallback Behavior
- If event is `UNKNOWN` → use search pipeline
- If required params missing → use search pipeline with original input

---

## Data Storage

- Data stored in `./.data/` directory
- Intent prompts saved to `.data/intent/<timestamp>.md`
- Queue items in `.data/queue/`

---

## Commands

| Command | Action |
|---------|--------|
| `quit` / `exit` | Exit application |
| Any other text | Route through intent parser |

---

## Troubleshooting

### Ollama not responding
- Ensure Ollama is running: `ollama list`
- Check model: `qwen2.5:1.5b`

### Pipeline not found
- Check `config/pipelines.yaml` syntax
- Ensure pipeline name matches event (EXECUTE_{NAME})

### Action fails
- Check action type is supported
- Verify URL/command exists

---

## Development

### Building
```bash
cargo build
cargo build --release
```

### Testing
```bash
cargo test
cargo test <test_name>
```

### Linting
```bash
cargo fmt
cargo clippy
```

---

## Architecture

See `docs/design.md` for detailed architecture documentation.