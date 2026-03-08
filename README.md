# GoGogot — Lightweight OpenClaw Written in Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/aspasskiy/GoGogot?style=flat-square)](https://go.dev)
[![License](https://img.shields.io/github/license/aspasskiy/GoGogot?style=flat-square)](LICENSE)
[![Stars](https://img.shields.io/github/stars/aspasskiy/GoGogot?style=flat-square)](https://github.com/aspasskiy/GoGogot/stargazers)
[![Lines of code](https://img.shields.io/badge/lines-~4%2C500-blue?style=flat-square)](#)
[![Docker](https://img.shields.io/docker/pulls/octagonlab/gogogot?style=flat-square)](#deployment)

A **lightweight, extensible, and secure** open-source AI agent that lives on your server. It runs shell commands, edits files, browses the web, manages persistent memory, and schedules tasks — a self-hosted alternative to OpenClaw (Claude Code) in ~4,500 lines of Go. The entire agent is a single Go binary under 15 MB that idles at ~10 MB RAM and deploys with one `docker run` command. No frameworks, no plugins, no magic.

### Philosophy

The core philosophy of GoGogot is built around being **lightweight, extensible, and secure**:

- **Lightweight & Containerized**: A single Go binary running inside a Docker container. No heavy frameworks, no complex orchestration. Just a simple eval loop with good tools and smart prompts that consistently outperforms complex frameworks.
- **Secure**: You are fully in control. API keys never leave your server, and the agent runs isolated in a container.
- **Single Model & Cost-Efficiency**: Driven by a single LLM of your choice. Switch between Anthropic and 200+ models via OpenRouter with one env var — use affordable models for routine tasks, frontier models when you need them.
- **Extensible**: Clean Go interfaces make it trivial to add new LLM providers, transports, or custom tools.

### How It Works

The entire agent is a `for` loop. No framework, no state machine, no orchestration layer — just call the LLM, execute any tool calls it returns, feed the results back, and repeat until the model has nothing left to do.

Here is a simplified version of the actual [`Run`](agent/run.go) method with logging and bookkeeping stripped away:

```go
func (a *Agent) Run(ctx context.Context, input []ContentBlock) error {
    a.messages = append(a.messages, userMessage(input))

    for {
        resp, err := a.llm.Call(ctx, a.messages, a.tools)
        if err != nil {
            return err
        }
        a.messages = append(a.messages, resp)

        if len(resp.ToolCalls) == 0 {
            break // model is done — send text to user
        }

        results := a.executeTools(resp.ToolCalls)
        a.messages = append(a.messages, results)
    }
    return nil
}
```

That's it. Everything else — memory, scheduling, compaction, identity — is just tools the LLM can call inside this loop.

## Use Cases

> 📰 **Daily Digest**
> *"Find top 5 AI news from today, summarize each in 2 sentences, send me every morning at 9:00"*

> 📊 **Report Generation**
> *"Download sales data from this URL, calculate totals by region, generate a PDF report"*

> 📁 **File Processing**
> *"Take these 12 screenshots, merge them into a single PDF, and send the file back"*

> 🔍 **Market Research**
> *"Search the web for pricing of competitors X, Y, Z and make a comparison table"*

> 🖥️ **Server Monitoring**
> *"Check disk and memory usage every hour, alert me in Telegram if anything exceeds 80%"*

> 🗃️ **Data Extraction**
> *"Fetch this webpage, extract all email addresses and phone numbers into a CSV"*

> ⚙️ **Routine Automation**
> *"Every Friday at 18:00, pull this week's git commits and send me a changelog summary"*


## You Are In Control

- **Your keys stay on your server.** API keys are passed as environment variables at deploy time and never leave the container. There is no cloud account, no SaaS dashboard, no telemetry, no phoning home.
- **You pick the model.** 7 built-in models from budget to frontier — switch with one env var. Bring any OpenAI-compatible or Anthropic-compatible endpoint by adding a JSON entry to `models.json`.
- **You extend the code.** Clean Go interfaces (`Backend`, `Transport`, `Tool`) — add a new LLM provider, a Discord transport, or a custom tool by implementing a single interface. No plugin registry, no framework lock-in.


## Choosing a Model

Two providers are connected out of the box: **Anthropic** (Claude) and **[OpenRouter](https://openrouter.ai)** (access to 200+ models). Set the model via env var or CLI flag:

```bash
GOGOGOT_MODEL=deepseek      # env var
./gogogot --model=gemini     # CLI flag (overrides env)
```

If `GOGOGOT_MODEL` is not set, the first available provider is used.

### Built-in Models

| ID         | Model             | Provider   | Context | Vision |
| ---------- | ----------------- | ---------- | ------- | ------ |
| `claude`   | Claude Sonnet 4.6 | Anthropic  | 1M      | Yes    |
| `deepseek` | DeepSeek V3.2     | OpenRouter | 164K    | No     |
| `gemini`   | Gemini 3 Pro      | OpenRouter | 1M      | Yes    |
| `minimax`  | MiniMax M2.5      | OpenRouter | 1M      | No     |
| `qwen`     | Qwen3.5 397B A17B | OpenRouter | 262K    | Yes    |
| `llama`    | Llama 4 Maverick  | OpenRouter | 1M      | Yes    |
| `kimi`     | Kimi K2.5         | OpenRouter | 262K    | Yes    |

For independent benchmarks and model comparisons see [PinchBench](https://pinchbench.com/).

### Adding Custom Models

Models are defined in `models.json`. Defaults are compiled into the binary, but you can override them by placing your own `models.json` in the data directory (`~/.gogogot/models.json`):

```json
[
  {
    "id": "mythomax",
    "label": "MythoMax 13B (OpenRouter)",
    "model": "gryphe/mythomax-l2-13b",
    "base_url": "https://openrouter.ai/api/v1",
    "api_key_env": "OPENROUTER_API_KEY",
    "format": "openai",
    "context_window": 4096
  }
]
```

Copy an entry, change 3 fields, restart. No recompilation needed. The `api_key_env` field references the environment variable name — keys are passed via environment variables at runtime, the config is safe to commit.

## Extensible by Design

No plugin system, no registry, no framework. Just Go interfaces and a JSON config.

### Adding a model (no code changes)

Any OpenAI-compatible or Anthropic-compatible API works out of the box. Add an entry to `models.json`, set the `format` field, restart:

- `"format": "openai"` — OpenRouter, Together, Fireworks, any OpenAI-compatible endpoint
- `"format": "anthropic"` — Anthropic direct API or compatible proxies

See [Adding Custom Models](#adding-custom-models) above for the JSON schema.

### Adding a new LLM backend (code)

If you need a non-OpenAI, non-Anthropic wire format, implement the `Backend` interface — one method:

```go
type Backend interface {
    Call(ctx context.Context, model string, systemPrompt string,
        messages []types.Message, tools []types.ToolDef, maxTokens int,
    ) (*types.Response, error)
}
```

Then register it in `llm/client.go`:

```go
case "myformat":
    backend = mypkg.NewBackend(p.BaseURL, p.APIKey)
```

Ships with: **Anthropic** (native SDK) and **OpenAI-compatible** (OpenRouter, etc.).

### Adding a new transport (code)

Implement `Transport` — 3 methods:

```go
type Transport interface {
    Name() string
    Run(ctx context.Context, handler Handler) error
    SendText(ctx context.Context, channelID string, text string) error
}
```

Optionally implement `FileSender`, `TypingNotifier`, `StatusUpdater` for richer UX (file uploads, typing indicators, editable status messages).

Then add a case in `cmd/main.go`:

```go
case "discord":
    return discord.New(cfg.DiscordToken)
```

Ships with: **Telegram**. Want Discord, Slack, or Matrix? Implement 3 methods and plug it in.

## Features

- **Telegram** — multi-chat, attachments (images, documents), typing indicators
- **System access** — bash, read/write/edit files, system info
- **Web** — search (Brave), fetch pages, HTTP requests, download files
- **Identity** — persistent `soul.md` (agent personality) and `user.md` (owner profile), auto-evolving through conversations
- **Memory** — persistent markdown files the agent reads and writes itself
- **Skills** — reusable procedural knowledge the agent creates and consults itself
- **Task planning** — session-scoped checklist for multi-step work
- **Scheduling** — cron-based self-scheduling, persisted across restarts
- **Compaction** — automatic context compression when approaching token limits
- **Multi-model** — 7 built-in models (Claude Sonnet 4.6, DeepSeek V3.2, Gemini 3 Pro, MiniMax M2.5, Qwen3.5, Llama 4, Kimi K2.5), add your own via `models.json`
- **Observability** — structured event system (LLM calls, tool executions, errors)

## Tools

27 built-in tools across 10 categories: shell, files, web, identity, memory, skills, system info, scheduling, transport, and task planning.

The LLM sees the full tool list and picks the right one for the job.

## Installation & Quick Start

### Prerequisites

- Get a `TELEGRAM_BOT_TOKEN` by creating a new bot via [@BotFather](https://t.me/BotFather) on Telegram.
- Find your `TELEGRAM_OWNER_ID` (your personal Telegram user ID) using a bot like [@userinfobot](https://t.me/userinfobot). **This is critical for security** — it ensures only you can communicate with your agent.

### Docker Deployment

No git clone needed — the image is published on Docker Hub:

```bash
docker run -d --restart unless-stopped \
  --name gogogot \
  -e TELEGRAM_BOT_TOKEN=... \
  -e TELEGRAM_OWNER_ID=... \
  -e OPENROUTER_API_KEY=... \
  -e GOGOGOT_MODEL=deepseek \
  -v ./data:/data \
  -v ./work:/work \
  octagonlab/gogogot:latest
```

That's it. The image supports `linux/amd64` and `linux/arm64`.

The Docker image ships with a full Ubuntu environment: bash, git, Python, Node.js, ripgrep, sqlite, postgresql-client, and more.

<details>
<summary>Alternative: Docker Compose</summary>

```bash
curl -O https://raw.githubusercontent.com/aspasskiy/GoGogot/main/deploy/docker-compose.yml

# Create .env with your keys
cat > .env <<EOF
TELEGRAM_BOT_TOKEN=...
TELEGRAM_OWNER_ID=...
OPENROUTER_API_KEY=...
GOGOGOT_MODEL=deepseek
EOF

docker compose up -d
```

</details>

### Local Development

To run the agent locally without Docker (requires Go 1.25+):

```bash
go run cmd/main.go
```

## Minimum Dependencies

- **Go 1.25** — compiled, concurrent, zero-dependency runtime
- [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) — native Claude API
- [openai-go](https://github.com/openai/openai-go) — OpenRouter / OpenAI-compatible providers
- [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram transport
- [robfig/cron](https://github.com/robfig/cron) — scheduler
- [goldmark](https://github.com/yuin/goldmark) — markdown parsing
- [goquery](https://github.com/PuerkitoBio/goquery) — HTML parsing for web tools
- [zerolog](https://github.com/rs/zerolog) — structured logging

## License

MIT