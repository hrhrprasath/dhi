# Topic Management System

## Problem

The old system stored all conversation history in a single file (`.data/conversation/summary.json`). Every interaction was
appended to the same unbounded context — there was no concept of separate conversations, no way to reset context, and no way
to revisit past exchanges without scrolling the TUI.

## Solution: Topic-Based Architecture

Replace monolithic history with named, switchable topics. Each topic is an isolated conversation with its own turns,
title, and timestamps.

```
User
  │
  ├── /new          ─→  create empty topic, switch to it
  ├── /fork         ─→  copy current topic's turns into new topic, switch
  ├── /resume       ─→  switch to the previously-active topic
  ├── /topics       ─→  list all topics in the conversation panel
  ├── /topic <N>    ─→  switch to topic by number or id
  └── <message>     ─→  append turn to current topic
```

## File Layout

```
.data/topics/
├── topics.json              # Registry
├── topic_a1b2c3d4.json      # Canonical topic data
├── topic_a1b2c3d4.md        # Human-readable mirror
└── ...
```

### `topics.json` — Registry

```json
{
  "topics": [
    {
      "id": "topic_a1b2c3d4",
      "title": "Stock research",
      "created_at": "2026-06-09T10:00:00Z",
      "updated_at": "2026-06-09T10:15:00Z"
    },
    {
      "id": "topic_e5f6g7h8",
      "title": "Weather chat",
      "created_at": "2026-06-09T11:00:00Z",
      "updated_at": "2026-06-09T11:30:00Z"
    }
  ],
  "current": "topic_e5f6g7h8",
  "previous": "topic_a1b2c3d4"
}
```

| Field     | Purpose                                          |
|-----------|--------------------------------------------------|
| `current` | ID of the topic the user is actively in           |
| `previous`| ID of the topic to return to via `/resume`        |
| `topics`  | Ordered list of all topics (newest-first)         |

### `<id>.json` — Canonical Data

```json
{
  "id": "topic_a1b2c3d4",
  "title": "Stock research",
  "created_at": "2026-06-09T10:00:00Z",
  "updated_at": "2026-06-09T10:15:00Z",
  "turns": [
    {
      "input": "What is AAPL at?",
      "response": "AAPL is trading at $198.42, up 1.2% today.",
      "time": "2026-06-09T10:05:00Z"
    },
    {
      "input": "What about GOOGL?",
      "response": "GOOGL is at $175.80, down 0.3%.",
      "time": "2026-06-09T10:10:00Z"
    }
  ]
}
```

This is the **canonical** store. The TUI parses it into `[]conversationEntry` for instant rendering on topic switch.
Every `AppendTurn` call writes to this file.

### `<id>.md` — Human-Readable Mirror

Generated automatically from the JSON. Useful for external review, `rg` searches, and reference outside the TUI.

```markdown
# Topic: Stock research
Created: 2026-06-09T10:00:00Z
Updated: 2026-06-09T10:15:00Z

## User: What is AAPL at?
## DHI: AAPL is trading at $198.42, up 1.2% today.

---

## User: What about GOOGL?
## DHI: GOOGL is at $175.80, down 0.3%.
```

## Commands

| Command | Action |
|---|---|
| `/new` | Save current topic, create a new empty topic, set it as current, clear TUI conversation panel |
| `/fork` | Save current topic, create a new topic with an identical copy of all turns, switch to it |
| `/resume` | Swap `current` ↔ `previous` in the registry, reload the previous topic's turns into the TUI |
| `/topics` | Display a numbered list of all topics in the conversation panel |
| `/topic <N\|id>` | Switch to topic by its list number (from `/topics`) or by its full ID |

## Topic Lifecycle

### Startup

1. `topic.NewManager(".data")` creates the `topics/` directory if missing.
2. Loads `topics.json` — if absent, creates a default "Topic 1".
3. **Migration:** If `.data/conversation/summary.json` exists (old format), its contents are imported as the first topic,
   then the old file is renamed to `.data/conversation/summary.json.migrated`.
4. `LoadTopicTurns(currentTopic.ID)` reads the JSON and populates `m.conversations` for the TUI.

### Turn Flow

Every user input → response cycle:
1. The response is appended to `m.conversations` (in-memory TUI display).
2. `topicMgr.AppendTurn(id, input, response)` writes the new turn to `<id>.json` and `<id>.md`.
3. The prompt sent to Ollama is built from `topicMgr.FormatForPrompt()` which reads the last N turns.

### Topic Switch Flow

1. All turns are already persisted (written on each turn).
2. Update the topic registry (current/previous pointers).
3. Call `topicMgr.LoadTopicTurns(newID)` to populate the TUI.
4. `m.convView.GotoBottom()` repositions the viewport.

## Package: `internal/topic/`

### Types

```go
type Turn struct {
    Input    string `json:"input"`
    Response string `json:"response"`
    Time     string `json:"time"`
}

type Topic struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}

type TopicList struct {
    Topics  []Topic `json:"topics"`
    Current string  `json:"current"`
    Previos string  `json:"previous"`
}

type TopicData struct {
    Topic
    Turns []Turn `json:"turns"`
}

type Manager struct {
    dataDir string
    list    *TopicList
    mu      sync.Mutex
}
```

### Key Methods

| Method | Description |
|---|---|
| `NewManager(dataDir) *Manager` | Init, migrate old data, ensure dirs |
| `CurrentTopic() *Topic` | Topic matching `list.Current` |
| `CreateTopic(title) *Topic` | New ID, append to list, set current+previous |
| `ForkTopic(title) *Topic` | Copy current topic's turns into new topic |
| `ResumeTopic() *Topic` | Swap current↔previous, return previous topic |
| `SwitchToTopic(id) bool` | Set current to given id, old current becomes previous |
| `AppendTurn(id, input, response) error` | Append to `<id>.json` + regenerate `.md` |
| `LoadTopicTurns(id) []Turn` | Parse `<id>.json` |
| `FormatForPrompt() string` | Format last 5 turns for LLM prompt |
| `ListTopics() []Topic` | Return copy of topic list |

### Auto-Title

When a topic is created via `/new`, its title is set to `"Topic N"`. After the first user message + response,
the title auto-updates to the first 60 characters of the user's input.

## TUI Changes

### `tui/types.go`
- Replace `convMgr *conversation.Manager` → `topicMgr *topic.Manager`
- Keep `conversations []conversationEntry` for rendering

### `tui/update.go`
- Slash completes `/new`, `/fork`, `/resume`, `/topics`, `/topic`
- `/new` handler: `topicMgr.CreateTopic()` → clear conversations → update view
- `/fork` handler: `topicMgr.ForkTopic()` → load turns → update view
- `/resume` handler: `topicMgr.ResumeTopic()` → load turns → update view
- `/topics`: Build numbered list → display in conversation panel
- `/topic <N>`: Parse number → lookup in list → `topicMgr.SwitchToTopic()` → load turns
- `processTasks`: `topicMgr.FormatForPrompt()` replaces old conv manager call
- `handleConversation/Pipeline`: `topicMgr.AppendTurn()` replaces `convMgr.UpdateSummary()`

### `tui/view.go`
- `renderConvBlock()` header changes from `" CONVERSATION "` to `" TOPIC: <title> "`

### `cmd/dhi/main.go`
- Wire `topic.NewManager(".data")` instead of `conversation.NewManager(".data")`

## Migration Path

On first launch with the new code:
1. Check if `.data/conversation/summary.json` exists.
2. If yes, read its `Turns` field, create a topic named after the first input, write the `.json` and `.md` files.
3. Rename the old file to `summary.json.migrated` so it is not imported again.
4. Proceed with normal startup.
