# DHI Harness

[![CI](https://github.com/hrhrprasath/dhi/actions/workflows/go.yml/badge.svg)](https://github.com/hrhrprasath/dhi/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/hrhrprasath/dhi)](https://github.com/hrhrprasath/dhi/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-blue.svg)](go.mod)

Terminal AI OS — Cognitive Sandwich pattern: LLM-classified intent → deterministic Go pipelines → LLM-synthesized response.

Works on **macOS** and **Linux**. Windows is not supported (bash pipelines, `open`/`xdg-open` branching).

## Requirements

* Go 1.26+
* Ollama running locally with `qwen2.5:1.5b` (default, lightweight classify/chat/synthesis)
* `ripgrep` (`rg`), `jq`, `curl` — required
* `ddgr` + `agent-browser` — required for `web_search` / `fetch_url`
* `s-search` (`s`) — required for `site_search` only
* `xdg-utils` (`xdg-open`) — Linux only, for `open_url` / Spotify links

## Install

```bash
# 1. Clone
git clone https://github.com/hrhrprasath/dhi.git
cd dhi

# 2. Install system deps + Ollama model (macOS via Homebrew, Linux via apt/dnf/pacman/apk or Linuxbrew)
./install.sh
# Skip the ~1GB model download: ./install.sh --no-model

# 3. Run
./build/dhi
# or: go run ./cmd/dhi
```

From release tarball (darwin/linux, amd64/arm64):

```bash
tar xzf dhi_*_Linux_x86_64.tar.gz
./dhi
```

From source with Go toolchain:

```bash
go install github.com/hrhrprasath/dhi/cmd/dhi@latest
```

## Configuration

First run auto-creates `~/.dhi/` from embedded defaults:

```
~/.dhi/
  config.yaml      # AI provider, model, RSS, accent color
  pipelines.yaml   # external (bash) pipeline definitions
  scripts/         # stock, web_search, fetch_url, open_url, spotify
  data/            # tasks, todos, topics, settings, learn, rss, logs
```

Default `~/.dhi/config.yaml` (Ollama only):

```yaml
ai:
  provider: ollama
  base_url: http://localhost:11434/v1
  api_key: ""
  model: qwen2.5:1.5b
  max_tokens: 4096
  temperature: 0.7
```

### Other providers

Any OpenAI-compatible endpoint works — just edit the `ai:` block in `~/.dhi/config.yaml`. API keys use `${VAR}` environment expansion (e.g. `export GROQ_API_KEY=...`). Never commit keys.

```yaml
# Groq
ai:
  provider: groq
  base_url: https://api.groq.com/openai/v1
  api_key: ${GROQ_API_KEY}
  model: llama-3.1-70b-versatile

# OpenRouter
ai:
  provider: openrouter
  base_url: https://openrouter.ai/api/v1
  api_key: ${OPENROUTER_API_KEY}
  model: anthropic/claude-sonnet-4

# OpenAI
ai:
  provider: openai
  base_url: https://api.openai.com/v1
  api_key: ${OPENAI_API_KEY}
  model: gpt-4o-mini

# Custom (vLLM, NIM, local server)
ai:
  provider: custom
  base_url: http://localhost:8000/v1
  api_key: ${CUSTOM_API_KEY}
  model: your-model-name
```

Switch provider/model at runtime via the in-app settings panel or by editing `~/.dhi/config.yaml`.

## Project Structure

```
├── cmd/dhi/main.go              # Entry point, wires dependencies
├── internal/
│   ├── ai/                      # OpenAI-compatible client (Ollama default, others via config)
│   ├── config/                  # YAML config loading + ${ENV} expansion
│   ├── dhroot/                  # ~/.dhi paths + embedded defaults (config, pipelines, scripts)
│   ├── intent/                  # LLM router: classify + chat
│   ├── learn/                   # Self-learning feedback loop (weights, calibration, keywords)
│   ├── pipeline/                # Pipeline definitions, executor, internal handlers
│   ├── prompt/                  # Classify/conversation/synthesis templates
│   ├── rss/                     # RSS manager
│   ├── services/                # Logger, system metrics
│   ├── settings/                # Model list + selection
│   ├── summarizer/              # LLM summarization of pipeline output
│   ├── task/                    # Task CRUD, persistence, scheduled-task looper
│   ├── todo/                    # Todo CRUD
│   └── topic/                   # Topic tracking
├── tui/                         # Bubble Tea v2 TUI
└── docs/                        # Design docs + user guide + GH Pages site
```

## Pipelines

| Pipeline | Type | Description |
|----------|------|-------------|
| stocks | bash (`scripts/stock`) | Stock price lookup via Yahoo Finance |
| web_search | bash (`scripts/web_search`) | DuckDuckGo via `ddgr` + page fetch via `agent-browser` |
| site_search | bash (`s --provider`) | Site-specific search, opens browser (needs `s-search`) |
| fetch_url | bash (`scripts/fetch_url`) | Fetch URL content via `agent-browser` |
| open_url | bash (`scripts/open_url`) | Open URL (`open` on macOS, `xdg-open` on Linux) |
| music | bash (`scripts/spotify`) | Spotify search (`open`/`xdg-open`) |
| todo | internal | Todo/reminder CRUD |
| system | internal | CPU/RAM/GPU metrics |
| schedule | internal | Scheduled task CRUD (60s looper) |

## Usage

### Slash commands

| Command | Description |
|---------|-------------|
| `/help`, `/?`, `:h` | Open the help popup |
| `/todo` | Open todo list (active / done tabs) |
| `/schedule` | Open scheduled tasks (active / completed tabs) |
| `/rss` | Open RSS reader |
| `/rss add <url>` | Add a new RSS feed source |
| `/topic`, `/topics` | Browse and switch conversation topics |
| `/model`, `/models` | Browse and switch AI models |
| `/log`, `/logs`, `/l` | Open log viewer |
| `/new` | Create a new conversation topic |
| `/fork` | Fork the current topic into a new one |
| `/resume` | Switch back to the previous topic |
| `/good` | Rate the last response positively (trains pipeline weights) |
| `/bad` | Rate the last response negatively |
| `/quit`, `/q`, `:q` | Exit DHI |

Anything without a `/` prefix is sent to the LLM classifier, which picks the matching pipeline automatically.

### Todo & schedule

`/todo` manages reminders with persistent JSON storage. `/schedule` accepts natural language times (`schedule meeting at 3pm`) and supports recurring tasks — a 60s looper activates due tasks.

### RSS reader

`/rss` opens the feed reader (AI CURATED / RAW / BOOKMARKS tabs). Sources live as OPML in `~/.dhi/data/rss/feed.opml` (override with `DHI_RSS_OPML_PATH`). `B` bookmarks, `R` removes a source, `s` refreshes the AI summary. `rss.skip_shorts` in `config.yaml` filters YouTube Shorts.

### Topics & models

Topics persist conversation history across restarts (`/new`, `/fork`, `/resume`, `/topic` to switch). `/model` lists Ollama models and switches the active one for classify/chat/synthesis.

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

## Troubleshooting

* `agent-browser not found` → macOS: `brew install agent-browser && agent-browser install`; Linux: same via Linuxbrew or `npm i -g agent-browser && agent-browser install`.
* `ddgr not found` → macOS: `brew install ddgr`; Linux: `sudo apt install ddgr` (Debian/Ubuntu), `sudo dnf install ddgr` (Fedora), `sudo pacman -S ddgr` (Arch).
* `s: command not found` → `site_search` only; macOS/Linux: `brew install s-search`.
* Ollama not running → `ollama serve &` then `ollama pull qwen2.5:1.5b`.
* Wrong model → check `~/.dhi/config.yaml` `ai.model`, or switch in-app.
* Logs → `~/.dhi/data/dhi.log`, in-app `/log` popup.

## Development

```bash
make build    # go build -o build/dhi ./cmd/dhi
make test     # go test ./... -count=1
make cover    # go test with coverage.out
make lint     # golangci-lint run
make vet      # go vet ./...
make tidy     # go mod tidy
make fmt      # go fmt ./...
```

Releases: push a `v*` tag, CI builds darwin/linux amd64/arm64 via GoReleaser.

## License

MIT — see [LICENSE](LICENSE).
