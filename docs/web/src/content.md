# Install {#install}

<p class="section-subtitle">Get DHI running on your machine in minutes</p>

### Prerequisites

DHI works on **macOS** and **Linux** and requires **Go 1.26+**, **Ollama** with `qwen2.5:1.5b` (default), plus `rg`, `jq`, and `curl`. `ddgr` + `agent-browser` power web_search/fetch_url; `s-search` powers site_search.

### Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/hrhrprasath/dhi/main/install.sh | sh
```

### Or install via Go

```bash
go install github.com/hrhrprasath/dhi/cmd/dhi@latest
```

### Build from source

```bash
git clone https://github.com/hrhrprasath/dhi.git
cd dhi && go build -o build/dhi ./cmd/dhi
./build/dhi
```

### First run

On first launch, DHI auto-populates **~/.dhi/** with default config, pipeline definitions, and scripts. Ensure Ollama is running with your chosen model — DHI checks health on startup (<span class="tag tag-green">● READY</span>).


# Why DHI? {#why}

<p class="section-subtitle">A fundamentally different approach to AI integration</p>

::: {.card-grid}

::: {.card}
<div class="icon">01</div>

### Local Light Models

Built for Qwen 1B/3B via Ollama — runs entirely on your machine. No API costs, no data leaving your laptop, sub-second classification on a Mac Mini M4. Privacy-first by default.
:::

::: {.card}
<div class="icon">02</div>

### Cognitive Sandwich

Three-layer architecture: **Intent** (LLM classifies input → strict JSON) → **Execute** (deterministic Go pipelines, zero AI) → **Synthesize** (LLM formats response). AI is used only where it adds value.
:::

::: {.card}
<div class="icon">03</div>

### Agent vs Harness vs MCP

**Agents** are autonomous — AI decides what to do and does it. **MCP** is a protocol for tool exposure. **DHI** is a harness: AI classifies intent, but deterministic Go code executes — *you stay in control*.
:::

::: {.card}
<div class="icon">04</div>

### Code in Control

The AI suggests and classifies; it *never* executes actions directly. All pipelines are pure Go code or auditable bash scripts. Predictable, debuggable, testable. **You** decide what runs.
:::

::: {.card}
<div class="icon">05</div>

### Extensible by Design

Write a bash script once, add 4 lines to `pipelines.yaml`, and it's a new action. No AI retraining, no framework changes. Pipelines can be internal (Go) or external (YAML-defined with script references).
:::

::: {.card}
<div class="icon">06</div>

### Self-Learning &amp; Feedback

Weighted pipeline associations (<span class="tag tag-accent">weight 0.8</span>) with keyword learning and confidence calibration. Correct/incorrect feedback loops improve classification over time without cloud retraining.
:::

::: {.card}
<div class="icon">07</div>

### Topics vs Sessions

**Topics** are like opencode conversations or ChatGPT threads — they persist history, can be named, searched, forked, and resumed across restarts. **Sessions** are ephemeral interaction flows within a topic — like a single opencode task or a ChatGPT response turn. Topics live in `~/.dhi/data/topics/`; sessions exist only in memory.
:::

:::

### How It Works

<div class="diagram">
<span class="box">User Input</span> <span class="arrow">→</span>
<span class="box">Task Queue</span> <span class="arrow">→</span>
<span class="box">LLM Classify<span class="label">Qwen via Ollama</span></span> <span class="arrow">→</span>
<span class="box box-green">Execute Pipeline<span class="label">Deterministic Go / Bash</span></span> <span class="arrow">→</span>
<span class="box">LLM Synthesize<span class="label">Summary for user</span></span> <span class="arrow">→</span>
<span class="box box-dim">Output</span>
</div>

**Head-to-head comparison**

<div style="overflow-x:auto; margin-top: 16px;">
<table class="compare-table">
<thead>
<tr>
  <th>Dimension</th>
  <th>DHI Harness</th>
  <th>AI Agent</th>
  <th>MCP</th>
</tr>
</thead>
<tbody>
<tr>
  <td style="color:var(--text-bright);">Who executes?</td>
  <td><span class="check">●</span> Deterministic Go code</td>
  <td><span class="cross">●</span> AI decides &amp; executes</td>
  <td><span class="cross">●</span> Protocol, no execution</td>
</tr>
<tr>
  <td style="color:var(--text-bright);">AI scope</td>
  <td>Classify + synthesize only</td>
  <td>Full autonomy</td>
  <td>Tool definition</td>
</tr>
<tr>
  <td style="color:var(--text-bright);">Extensibility</td>
  <td><span class="check">●</span> YAML + scripts</td>
  <td><span class="cross">●</span> Depends on framework</td>
  <td><span class="check">●</span> Server-based</td>
</tr>
<tr>
  <td style="color:var(--text-bright);">Debuggability</td>
  <td><span class="check">●</span> Full (Go tests + logs)</td>
  <td><span class="cross">●</span> Non-deterministic</td>
  <td><span class="cross">●</span> Depends on server</td>
</tr>
<tr>
  <td style="color:var(--text-bright);">Local-first</td>
  <td><span class="check">●</span> 100% local, 1B-3B models</td>
  <td><span class="cross">●</span> Usually cloud</td>
  <td><span class="cross">●</span> Protocol-agnostic</td>
</tr>
</tbody>
</table>
</div>


# How to Use? {#usage}

<p class="section-subtitle">Navigate the terminal UI, run commands, and manage your AI harness</p>

### Basic Flow

Type a command in the input bar → DHI classifies your intent via local LLM → executes the appropriate pipeline → displays the result. All without leaving the terminal.

### Getting Help

Press `/help` (or `/?` / `:h`) then `Enter` to open the help popup — it lists every slash command and keybinding.

### Tab Navigation

Press **Tab** to cycle focus through the panels:

```
Input  →  Conversation  →  Topics  →  Input  (loop)
```

When the **log popup** is open:

```
Input  →  Conversation  →  Log  →  Input  (loop)
```

Focused panels are highlighted with an orange border. Use `↑` / `↓` to scroll within the active panel.

### Slash Commands

Type these in the input bar and press `Enter`:

<div style="overflow-x:auto;">
<table class="compare-table">
<thead>
<tr><th>Command</th><th>Description</th></tr>
</thead>
<tbody>
<tr><td><code>/help</code>, <code>/?</code>, <code>:h</code></td><td>Open the help popup</td></tr>
<tr><td><code>/todo</code></td><td>Open todo list (active / done tabs)</td></tr>
<tr><td><code>/schedule</code></td><td>Open scheduled tasks (active / completed tabs)</td></tr>
<tr><td><code>/rss</code></td><td>Open RSS reader</td></tr>
<tr><td><code>/rss add &lt;url&gt;</code></td><td>Add a new RSS feed source</td></tr>
<tr><td><code>/topic</code>, <code>/topics</code></td><td>Browse and switch conversation topics</td></tr>
<tr><td><code>/model</code>, <code>/models</code></td><td>Browse and switch AI models</td></tr>
<tr><td><code>/log</code>, <code>/logs</code>, <code>/l</code></td><td>Open log viewer</td></tr>
<tr><td><code>/new</code></td><td>Create a new conversation topic</td></tr>
<tr><td><code>/fork</code></td><td>Fork the current topic into a new one</td></tr>
<tr><td><code>/resume</code></td><td>Switch back to the previous topic</td></tr>
<tr><td><code>/good</code></td><td>Rate the last response positively (trains the model)</td></tr>
<tr><td><code>/bad</code></td><td>Rate the last response negatively</td></tr>
<tr><td><code>/quit</code>, <code>/q</code>, <code>:q</code></td><td>Exit DHI</td></tr>
</tbody>
</table>
</div>

### Auto-Completion

Type `/` followed by a few characters — matching commands appear below the input. Press **Tab** to cycle through suggestions, **Enter** to confirm.

### RSS Reader

`/rss` opens the built-in feed reader. Add sources with `/rss add <feed-url>` — they are stored as OPML in `~/.dhi/data/rss/feed.opml` (override with `DHI_RSS_OPML_PATH`), so you can import an existing reader export by replacing that file. Feeds refresh in the background while DHI runs.

Three tabs (`←` / `→` to switch):

| Tab | Contents |
|-----|----------|
| AI CURATED | LLM summary of the latest entries — press `s` to refresh |
| RAW | Every entry across all feeds |
| BOOKMARKS | Entries you saved with `B` |

Keys inside the reader: `↑` / `↓` navigate, `Enter` opens the entry URL in your browser, `B` toggles a bookmark, `R` removes a feed source, `s` refreshes the AI summary (or fetches the entry into a topic), `Esc` closes.

YouTube noise is filtered via `~/.dhi/config.yaml`:

```yaml
rss:
  skip_shorts: true
```

### Inline Commands

Input without a `/` prefix is sent to the LLM classifier. DHI automatically detects your intent and runs the matching pipeline:

> <span style="color:var(--dim);">"search for latest AI news"</span> &rarr; runs <span class="tag tag-accent">web_search</span>
> <span style="color:var(--dim);">"add buy milk to my todo"</span> &rarr; runs <span class="tag tag-accent">todo</span>
> <span style="color:var(--dim);">"schedule meeting at 3pm"</span> &rarr; runs <span class="tag tag-accent">schedule</span>

### Popup Keybindings

Open any popup then use:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate items |
| `←` / `→` | Switch tabs (todo, schedule, RSS) |
| `Enter` | Confirm / open item URL |
| `R` / `r` | Remove selected item |
| `D` / `d` | Toggle done / undone |
| `B` / `b` | Toggle bookmark (RSS) |
| `Esc` | Close popup |

---

# The Name {#name}

<p class="section-subtitle">The story behind the name</p>

**DHI** stands for **D**eterministic **H**arness **I**ntelligence — a Terminal AI OS that keeps you in control. The acronym spells **DHI** in ASCII art on startup, and the name reflects the project's philosophy: deterministic execution, harness pattern, and intelligence augmentation rather than replacement.

DHI is your command center: it listens, suggests, and executes *your* commands with precision.

::: {.name-cards}

::: {.name-card}
### Deterministic

Every action is defined in Go or bash — predictable, testable, auditable. No AI hallucination in execution.
:::

::: {.name-card}
### Harness

A structured framework that wires intent to action. You wear the harness; the AI is your tool, not your replacement.
:::

::: {.name-card}
### Intelligence

Local LLMs provide classification, conversation, and synthesis — working *for* you, not autonomously.
:::

:::


# Features {#features}

<p class="section-subtitle">Built-in pipelines that ship with every install</p>

<div class="feature-grid">

<div class="feature-item">
<h4>Web Search</h4>
<p>ddgr + agent-browser with auto-summarization via LLM</p>
</div>

<div class="feature-item">
<h4>Site Search</h4>
<p>Site-scoped search opened in your browser via s-search</p>
</div>

<div class="feature-item">
<h4>Fetch URL</h4>
<p>Extract page content via agent-browser with LLM summarization</p>
</div>

<div class="feature-item">
<h4>Open URL</h4>
<p>Open any link in the default browser, with unsafe-URL rejection</p>
</div>

<div class="feature-item">
<h4>Todo</h4>
<p>Full CRUD manager with persistent JSON storage</p>
</div>

<div class="feature-item">
<h4>Schedule</h4>
<p>Natural language time parsing + cron-based recurring tasks</p>
</div>

<div class="feature-item">
<h4>RSS</h4>
<p>OPML feed reader with bookmarks, background refresh, and AI-curated summaries</p>
</div>

<div class="feature-item">
<h4>Stocks</h4>
<p>Yahoo Finance lookup via curl/jq pipeline</p>
</div>

<div class="feature-item">
<h4>Music</h4>
<p>Spotify search + playback (macOS via AppleScript, Linux via xdg-open)</p>
</div>

<div class="feature-item">
<h4>System</h4>
<p>Live CPU/RAM/GPU metrics with progress bars</p>
</div>

<div class="feature-item">
<h4>Conversations</h4>
<p>Topic-managed chat with auto-summarization context</p>
</div>

</div>


# Customize {#customize}

<p class="section-subtitle">Extend DHI without touching Go code</p>

### Pipelines &mdash; `~/.dhi/pipelines.yaml`

Define new actions by adding to the YAML. Each pipeline has a name, action type, and optional weight for self-learning.

```yaml
# ~/.dhi/pipelines.yaml
pipelines:
  my_command:
    action: bash
    command: scripts/my_script {arg}
    weight: 0.6
    summarize: true
```

### Action Types

Each pipeline action has a type that determines how it executes:

| Type | What it does | Key fields |
|------|-------------|------------|
| `bash` | Runs a shell script | `script`, `command` |
| `http` | Makes an HTTP request via curl | `url`, `method`, `auth` |
| `exec` | Runs a system command directly | `command`, `args` |
| `internal` | Calls a built-in Go handler | `handler` |

Examples of each type:

```yaml
pipelines:
  greet:
    action: bash
    script: scripts/greet {name}

  check_api:
    action: http
    url: https://api.example.com/status
    method: GET
    summarize: true

  deploy:
    action: exec
    command: /usr/local/bin/deploy
    args: ["--env", "staging"]

  todo:
    action: internal
    handler: todo
```

### Scripts &mdash; `~/.dhi/scripts/`

Bash scripts are referenced by pipeline actions. Drop a new script in `~/.dhi/scripts/` and wire it in `pipelines.yaml`.

```bash
#!/usr/bin/env bash
# ~/.dhi/scripts/my_script
QUERY="$1"
curl -s "https://api.example.com/search?q=$QUERY" | jq '.'
```

### Config &mdash; `~/.dhi/config.yaml`

Switch AI providers, models, or adjust generation parameters. Ollama is the default; any OpenAI-compatible endpoint works via the `ai:` block (keys use `${VAR}` environment expansion):

```yaml
# ~/.dhi/config.yaml
ai:
  provider: ollama
  base_url: http://localhost:11434/v1
  model: qwen2.5:1.5b
  max_tokens: 4096
  temperature: 0.7
```

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
```
