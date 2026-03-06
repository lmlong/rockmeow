# LingGuard — Copilot Instructions

LingGuard is an ultra-lightweight (~20 MB RAM) personal AI assistant written in Go 1.23. It runs as a single binary and bridges multiple LLM providers with messaging channels (Feishu, QQ) through a tool + skill system.

## Commands

```bash
make build            # Build for current platform
make dev              # Run without building
make test             # Run all tests (with race detection)
make test-coverage    # Generate coverage.html
make test-bench       # Benchmark tests
make fmt              # Format code
make lint             # golangci-lint static analysis
make package          # Package Linux + macOS
make package-all      # All platforms
make install          # Binary + config + systemd/launchd
```

Run a single test:
```bash
go test ./internal/agent/... -run TestFunctionName -v
go test ./pkg/memory/... -run TestAutoRecall -race
```

## Architecture

The request flow from a user message to a response is:

```
Channel (Feishu/QQ WebSocket)
  → Agent.ProcessMessageStream()
    → session.Manager  (lock, history)
    → memory.ContextBuilder  (auto-recall injection)
    → provider.Complete()  (LLM call with tools)
    → Tool execution loop  (max maxToolIterations, default 10)
    → stream.StreamCallback  (incremental card updates)
  → Auto-capture (async, deferred)
```

### Agent (`internal/agent/agent.go`)

`Agent` holds references to all subsystems. The tool-calling loop in `runLoopWithProvider()`:
1. Builds the system prompt from: skill summaries + user system prompt + soul definition + workspace + current time
2. Injects recalled memories if `autoRecall` is enabled
3. Calls `provider.Complete()` with current tool definitions
4. If no tool calls → returns content, defers auto-capture async
5. If tool calls → executes each, appends `role: tool` results, adds reflection prompt, loops
6. At max iterations → returns error; tool errors are formatted as `"Error: ..."` and fed back to the LLM (not propagated as Go errors)

### Providers (`internal/providers/`)

All providers are registered in `spec.go` as a `[]ProviderSpec` slice — this is the single source of truth for defaults (API base, default model, keywords, API key prefixes).

Provider matching priority in `registry.go`:
1. `provider/model` format (slash-separated)
2. Exact provider name match
3. Keyword match in `ProviderSpec.Keywords`
4. API key prefix (e.g., `sk-or-` → openrouter, `gsk_` → groq)
5. API base URL keyword match
6. Configured default provider

To **add a new provider**: add a `ProviderSpec` entry to `spec.go`; no other code changes needed unless the provider uses a non-OpenAI API format.

### Tools (`internal/tools/`)

Two categories:

| Type | `ShouldLoadByDefault()` | When Active |
|------|------------------------|-------------|
| Default | `true` | Every request (skill, memory, message, workspace, cron, taskboard) |
| Dynamic | `false` | Loaded per-session when a skill is invoked |

Dynamic tools are tracked per session in `Agent.sessionDynamicTools`. Tools unused for 10 consecutive requests are aged out automatically.

To **add a new tool**: implement the `Tool` interface, register in `registry.go`. Set `ShouldLoadByDefault()` based on whether it should always be available. For dangerous tools (shell), `IsDangerous()` should return `true`.

### Skills (`internal/skills/` + `skills/`)

Progressive loading strategy — the LLM only sees skill summaries in the system prompt. Full instructions and associated tools are loaded lazily when the LLM calls the built-in `skill` tool.

**`SKILL.md` format** (every skill directory requires this file):
```markdown
---
name: myskill
description: One-line description shown in the system prompt
metadata: {
  "nanobot": {
    "emoji": "🔧",
    "requires": {
      "bins": ["ffmpeg"],
      "env": ["MY_API_KEY"]
    }
  }
}
---

Full instruction markdown here...
```

Skills are marked unavailable if declared binary/env requirements are missing (checked at startup via `exec.LookPath` / `os.Getenv`). Optional `reference.md` and `examples.md` files in the same directory are concatenated into the loaded skill content.

To **add a new skill**: create `skills/<name>/SKILL.md`. If the skill needs dynamic tools (e.g., `aigc`, `web_search`), add the mapping to `skillToToolMapping` in `tools/skill.go`.

### Memory (`pkg/memory/`)

- **FileStore**: SQLite-backed (`~/.lingguard/memory/events.db`), always-on; stores full message history and sessions.
- **HybridStore** (optional): FileStore + vector DB (sqlite-vec); enables semantic search. Configured under `agents.memory.vector`.
- **Auto-recall**: At context-build time, hybrid search retrieves top-K memories (default 3) and injects them into the system prompt as `相关记忆`.
- **Auto-capture**: Deferred async at end of each conversation turn; uses regex trigger patterns to detect memorable content (preferences, facts, contacts). Includes prompt-injection detection to prevent poisoning the memory store.

### Channels (`internal/channels/`)

Both channels implement the same handler signatures:
```go
type StreamingMessageHandler func(ctx context.Context, from string, msg string, callback stream.StreamCallback) error
```

- **Feishu**: Long-connection WebSocket via Larksuite SDK; renders streaming responses as updating message cards.
- **QQ**: Custom opcode-based WebSocket protocol; heartbeat interval is server-driven from the `HELLO` packet.

## Key Conventions

### Error handling
- Sentinel errors as package-level vars: `var ErrSessionBusy = errors.New("...")`
- Always wrap with context: `fmt.Errorf("failed to load skill %q: %w", name, err)`
- Tool `Execute()` returns `(string, error)` — errors from tools are formatted as text and sent to the LLM, not surfaced to users
- Graceful degradation: vector search failure falls back to file store; missing multimodal provider falls back to main provider

### Structured logging
```go
logger.Info("processing message", "session", sessionID, "from", from)
logger.Error("tool execution failed", "tool", name, "error", err)
```
Always use structured key-value pairs, not `fmt.Sprintf` in log messages.

### Configuration
Config file is at `~/.lingguard/config.json` (overridden by `$LINGGUARD_CONFIG`). Provider defaults come from `spec.go` and are overridden per-key in config. Never hardcode API endpoints — add them to the provider spec.

### Type naming suffixes
`Config`, `Tool`, `Provider`, `Manager`, `Store`, `Registry`, `Handler` — used consistently across packages.

### Concurrency
- `sync.RWMutex` for all registries (tools, providers)
- Session locking uses `TryLockWithTimeout` (default 10 min); returns `ErrSessionBusy` when locked
- Auto-capture runs in a goroutine via `go func()` deferred from the main loop — don't block the response path
