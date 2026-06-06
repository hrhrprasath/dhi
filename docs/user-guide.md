# DHI Harness User Guide

DHI (Deterministic Harness Intelligence) is a terminal-based AI operating system that routes user input through an intent parser (Ollama local LLM), executes actions via pipelines, and summarizes results.

---

## Getting Started

### Requirements
- **Go 1.26+**
- **Ollama** running locally with a model (e.g., `qwen2.5:1.5b`)

### Running
```bash
go build ./cmd/dhi && ./dhi
# or
go run ./cmd/dhi
```

### Controls

| Key | Action |
|-----|--------|
| `Enter` | Submit command |
| `Tab` | Cycle focus: LOGS → INPUT → CONVERSATION |
| `↑/↓` | Scroll active panel |
| `PgUp/PgDown` | Page scroll active panel |
| `Ctrl+C` / `Ctrl+D` | Quit |
| `quit` / `exit` / `:q` | Quit via command |

---

## Pipeline Configuration

Pipelines are defined in `config/pipelines.yaml` (external) or hardcoded in Go (internal).

### External Pipelines (YAML)

```yaml
pipelines:
  - name: stocks
    description: Stock queries with news
    weight: 0.8
    params:
      - name: company
        required: true
        extract_hint: "Extract company name, stock symbol, or index"
    action:
      type: bash
      script: "scripts/stock {company}"
```

### Pipeline Weight

Each pipeline has a `weight` (0.3–0.9) shown to the LLM as a priority hint. Higher = more likely to be selected when multiple pipelines match.

| Weight | Pipeline |
|--------|----------|
| 0.9 | todo |
| 0.8 | stocks, schedule |
| 0.7 | search_inline |
| 0.6 | system |
| 0.5 | music |
| 0.4 | browse |
| 0.3 | search_browser |

---

## Available Pipelines

### 1. Stocks
Fetch stock prices and news.
```
check apple stock
what is nifty doing
AAPL share price
```

### 2. Search Inline
Search the web and display results in-terminal.
```
search latest AI news
find recipe for pasta
what is quantum computing
```

### 3. Search Browser
Open a search in the default browser (only when explicitly asked).
```
open youtube in browser
search wikipedia for go language
```

### 4. Browse
Read a webpage's content inline.
```
read https://example.com/article
open page https://news.ycombinator.com
```

### 5. Music
Search Spotify for a song/artist/podcast.
```
play shape of you
search spotify for jazz
play lofi beats
```

### 6. System
System information and observability.
```
show system status
what's my uptime
check kernel version
```

### 7. Todo
CRUD for personal task list.
```
add buy milk to my todos
list my todos
done 123456
```

### 8. Schedule
Schedule commands for later execution. Supports single and multi-schedule.
```
remind me to check AAPL at 5pm
schedule play song at 9am daily and news at 9:30am
list schedules
delete schedule abc123
```

---

## Intent Flow

```
1. Type input → taskMgr creates a one-shot task (pending)
2. processTasks() picks it up (200ms poll)
3. Ollama classifies → EXECUTE_XXX event with confidence score
4. If confidence >= 0.8 → execute pipeline
5. If confidence < 0.8 → fall back to conversation (Chat)
6. Result displayed in CONVERSATION panel
```

### Confidence & Fallback

The LLM outputs a `confidence` score with every classification:
- `> 0.9`: Very clear match
- `>= 0.8`: Execute pipeline
- `< 0.8`: Treat as conversation (Chat)
- If uncertain, the LLM should default to low confidence

---

## Scheduling

### One-shot schedule
```
remind me to check crypto in 30 minutes
```
Creates a task that fires once after 30 minutes.

### Recurring schedule
```
schedule play lofi at 8am daily
```
Creates a recurring daily task. After execution, it re-schedules for the next day.

### Multi-schedule (one command, multiple schedules)
```
play song at 9am daily and give me news summary at 9:30am
```
Creates two separate schedules from a single input.

### Managing schedules
```
list schedules          # View all waiting tasks
delete schedule abc123  # Delete by ID (prefix match)
update schedule abc123 when="daily at 10am"
update schedule abc123 command="check nifty"
```

---

## Data Storage

All data is stored in `.data/`:

```
.data/
├── dhi.log                 # Application logs
├── tasks/tasks.json        # All tasks (pending + completed + scheduled)
├── todos/todos.json        # User todo list
├── conversation/summary.json  # Conversation history
├── intent/<timestamp>.md   # Classify prompt dumps
└── <pipeline>/<timestamp>.md  # Pipeline output persistence
```

---

## Troubleshooting

### Ollama not responding
- Ensure Ollama is running: `ollama list`
- Check the model is downloaded: `ollama pull qwen2.5:1.5b`

### Pipeline not found
- Check `config/pipelines.yaml` syntax
- Ensure pipeline name matches the event prefix (`EXECUTE_{NAME}`)

### Action fails
- Check action type is supported (bash, http, exec, open, internal)
- Verify the script file exists in `scripts/`
- Check script permissions (`chmod +x scripts/*`)

---

## Development

### Building
```bash
go build ./...
```

### Linting
```bash
go fmt ./...
go vet ./...
```

### Architecture
See `docs/design.md` for detailed architecture documentation.
