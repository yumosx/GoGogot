![GoGogot](https://octagon-lab.sfo3.cdn.digitaloceanspaces.com/gogogot.jpg)

# GoGogot

[![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/github/license/aspasskiy/GoGogot)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/aspasskiy/GoGogot/build.yml?label=build)](https://github.com/aspasskiy/GoGogot/actions)
[![Stars](https://img.shields.io/github/stars/aspasskiy/GoGogot?style=flat)](https://github.com/aspasskiy/GoGogot/stargazers)
[![Lines of code](https://img.shields.io/badge/lines%20of%20code-~4500-blue)](#)
[![Docker](https://img.shields.io/badge/deploy-docker%20compose-2496ED?logo=docker&logoColor=white)](#deployment)

A personal AI agent that lives on your server. Talk to it on Telegram — it runs commands, edits files, browses the web, remembers things, and schedules tasks. ~4,500 lines of Go. No frameworks, no plugins, no magic.

```
You (Telegram) → GoGogot → bash, files, web, memory, scheduler → You
```

## Why GoGogot

|                    | GoGogot            | OpenClaw                                      |
| ------------------ | ------------------ | --------------------------------------------- |
| Language           | Go                 | TypeScript                                    |
| Codebase           | ~4,500 LOC         | ~430,000 LOC                                  |
| Dependencies       | 7                  | 800+ npm packages                             |
| Runtime            | Single binary      | 80+ MB Node.js                                |
| Architecture       | 1 loop + tools     | Gateway + plugins + channel router + registry |
| Deploy             | `docker compose up` | CLI wizard + daemon + Node >= 22             |
| Time to understand | An afternoon       | Good luck                                     |

Anthropic, Cursor, OpenClaw, and the OpenAI Agents SDK all converge on the same architecture: **a while loop with tools**. OpenClaw wraps it in 430K lines of TypeScript solving multi-tenant platform problems — channel routing, plugin registries, security models — that don't exist when you're the only user. GoGogot ships the loop and gets out of the way.

## You Are In Control

Everything is configured explicitly via environment variables. No cloud accounts, no SaaS dashboards, no telemetry.

| Variable | Purpose |
| --- | --- |
| `ANTHROPIC_API_KEY` | Claude (direct API) |
| `OPENROUTER_API_KEY` | MiniMax and other models via OpenRouter |
| `GOGOGOT_MODEL` | `claude` or `minimax` — you choose the model |
| `TELEGRAM_BOT_TOKEN` | Your Telegram bot |
| `TELEGRAM_OWNER_ID` | Only this user can talk to the bot |
| `BRAVE_API_KEY` | Web search (optional) |

Model is also selectable via CLI flag: `--model=minimax`. Your keys, your server, your data.

## Cost: MiniMax vs Claude

You pick the price/quality tradeoff. GoGogot supports both cheap and powerful models.

| Model | Input (per 1M tokens) | Output (per 1M tokens) |
| --- | --- | --- |
| MiniMax M2.5 (via OpenRouter) | $0.30 | $1.10 |
| Claude Opus 4.6 | $5.00 | $25.00 |
| Claude Opus 4 | $15.00 | $75.00 |

A typical agent session (~50K input, ~10K output):

| Model | Cost per session |
| --- | --- |
| MiniMax | ~$0.03 |
| Claude Opus 4.6 | ~$0.50 |
| Claude Opus 4 | ~$1.50 |

For routine tasks — daily digests, file management, web lookups — MiniMax is more than enough. Switch to Claude for complex reasoning when you need it. One env var.

## Extensible by Design

LLM providers and chat transports are clean Go interfaces. Adding a new one = implementing a few methods. No plugin system, no registry, no YAML config.

**LLM provider** — 3 methods:

```go
type LLM interface {
    Call(ctx context.Context, messages []Message, opts CallOptions) (*Response, error)
    ModelLabel() string
    ContextWindow() int
}
```

Ships with: **Anthropic** (native SDK) and **OpenAI-compatible** (OpenRouter — MiniMax, etc.)

**Transport** — 3 methods:

```go
type Transport interface {
    Name() string
    Run(ctx context.Context, handler Handler) error
    SendText(ctx context.Context, channelID string, text string) error
}
```

Optional capabilities via extra interfaces: `FileSender`, `TypingNotifier`, `StatusUpdater`.

Ships with: **Telegram**. Want Discord, Slack, or Matrix? Implement 3 methods and plug it in.

## Features

- **Telegram** — multi-chat, attachments (images, documents), typing indicators
- **System access** — bash, read/write/edit files, system info
- **Web** — search (Brave), fetch pages, HTTP requests, download files
- **Memory** — persistent markdown files the agent reads and writes itself
- **Scheduling** — cron-based self-scheduling, persisted across restarts
- **Compaction** — automatic context compression when approaching token limits
- **Multi-model** — Claude and MiniMax, manually chosen, not auto-routed
- **Observability** — structured event system (LLM calls, tool executions, errors)

## Architecture

```
┌──────────┐     ┌────────┐     ┌───────┐     ┌─────┐
│ Telegram │────▸│ Bridge │────▸│ Agent │────▸│ LLM │
└──────────┘     └────────┘     └───┬───┘     └─────┘
                                    │
                                    ▼
                               ┌─────────┐
                               │  Tools  │
                               └─────────┘
                          bash, files, web,
                        memory, scheduler, ...
```

The entire agent is one loop:

```go
for {
    guardrails.Check()
    maybeCompact()

    response := llm.Call(session.Messages())

    if no tool calls in response {
        break // done, send final text to user
    }

    for each tool call {
        result := tools.Execute(name, input)
        session.Append(result)
    }
}
```

The LLM decides everything — when to plan, when to ask the user, when to self-correct. These are prompt strategies, not code constructs. Adding a new behavior = editing the system prompt.

## Tools

| Tool | What it does |
| --- | --- |
| `bash` | Execute shell commands |
| `read_file` | Read file contents |
| `write_file` | Create or overwrite files |
| `edit_file` | Find-and-replace edits |
| `list_files` | List directory contents |
| `web_search` | Search the web (Brave) |
| `web_fetch` | Fetch and extract page content |
| `web_request` | Arbitrary HTTP requests |
| `web_download` | Download files by URL |
| `memory_list` | List memory files |
| `memory_read` | Read from memory |
| `memory_write` | Write to memory |
| `system_info` | OS, disk, memory info |
| `schedule_add` | Add a cron task |
| `schedule_list` | List scheduled tasks |
| `schedule_remove` | Remove a scheduled task |
| `send_file` | Send a file back to the user |

## Quick Start

```bash
git clone https://github.com/yourusername/sofie.git
cd sofie

# Configure
cp .env.example .env
# Edit .env: set TELEGRAM_BOT_TOKEN, TELEGRAM_OWNER_ID, API keys

# Run
docker compose -f deploy/docker-compose.yml up -d
```

The Docker image ships with a full Ubuntu environment: bash, git, Python, Node.js, ripgrep, sqlite, postgresql-client, and more.

## Philosophy

From the [orchestration spec](ORCHESTRATION_SPEC.md):

> **Simplicity** — one loop, good tools, smart prompt. Complexity is added only when metrics prove it helps.

> **LLM-first** — the LLM decides what to do, when to plan, when to critique its own work. The system executes, not orchestrates.

The previous spec had 10 orchestration patterns and 10 recipes as Go code structures. They were removed because a simple loop with good tools and prompts consistently outperforms complex orchestration frameworks. The only structural pattern retained is the **eval loop** — because external verification (tests, linter) requires actual code to check results and retry.

## Tech Stack

- **Go 1.25** — compiled, concurrent, zero-dependency runtime
- [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) — native Claude API
- [openai-go](https://github.com/openai/openai-go) — OpenRouter / OpenAI-compatible providers
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram transport
- [robfig/cron](https://github.com/robfig/cron) — scheduler
- [uber/fx](https://github.com/uber-go/fx) — dependency injection
- [goquery](https://github.com/PuerkitoBio/goquery) — HTML parsing for web tools

## License

MIT
