# hivebot

### open source bots that (almost) serve you

hivebots automate your menial work via AI models and build you microapps to manage the outcome, so that you have a false sense of control.

<p align="center">
  <img src="https://raw.githubusercontent.com/Northlatch-Labs-LLC/hivebot/main/assets/hero.png" alt="hivebot onboarding — Your AI team, visible and working." width="720" />
</p>

[![npm](https://img.shields.io/npm/v/hivebot?color=A87B4F)](https://www.npmjs.com/package/hivebot)
[![Discord](https://img.shields.io/badge/Discord-Join%20Community-5865F2?logo=discord&logoColor=white)](https://discord.gg/gjSySC3PzV)
[![License: MIT](https://img.shields.io/badge/license-MIT-A87B4F)](https://github.com/Northlatch-Labs-LLC/hivebot/blob/main/LICENSE)

One command. One shared office. CEO, PM, engineers, designer, CMO, CRO — all visible, arguing, claiming tasks, and shipping work instead of disappearing behind an API. And this one works.

[▶ Watch the demo](https://github.com/user-attachments/assets/312256d8-718a-44d1-8b42-fcf5019708d3)

## Get Started

**Prerequisites:** one bot CLI — [Claude Code](https://docs.anthropic.com/en/docs/claude-code) by default, or [Codex CLI](https://github.com/openai/codex) when you pass `--provider codex`. [tmux](https://github.com/tmux/tmux/wiki/Installing) is only required for `--tui` mode.

```bash
npx hivebot
```

That's it. The browser opens automatically and you're in the office.

Prefer a global install?

```bash
npm install -g hivebot && hivebot
```

Supported platforms: macOS, Linux, and Windows 10+ on x64 or arm64. The native binary is lazy-downloaded from [GitHub releases](https://github.com/Northlatch-Labs-LLC/hivebot/releases) on first run and cached under `node_modules/hivebot/bin/`.

> **Stability:** pre-1.0. `main` moves daily. Pin to a release tag, not `main`.

## Options

| Flag | What it does |
|------|-------------|
| `--memory-backend <name>` | Pick the organizational memory backend (`markdown`, `memanto`, `gbrain`, `none`) |
| `--tui` | Use the tmux TUI instead of the web UI |
| `--no-open` | Don't auto-open the browser |
| `--pack <name>` | Pick a bot pack (`starter`, `founding-team`, `coding-team`, `lead-gen-agency`, `revops`) |
| `--opus-ceo` | Upgrade CEO from Sonnet to Opus |
| `--provider <name>` | LLM provider override (`claude-code`, `codex`, `opencode`, `hermes-agent`, `openclaw-http`, `ollama`) |
| `--collab` | Start in collaborative mode — all bots see all messages (this is the default) |
| `--unsafe` | Bypass bot permission checks (local dev only) |
| `--web-port <n>` | Change the web UI port (default 7891) |

## Memory: Notebooks and the Wiki

Every bot gets its own **notebook**. The team shares a **wiki**. New installs get the wiki as a local git repo of markdown articles. Existing GBrain workspaces keep their knowledge-graph backend untouched.

**Backends for the wiki:**

- `markdown` (the "team wiki" tile in onboarding) is the default for new installs since v0.0.6. It stores typed facts, entity briefs, cited lookup answers, and lintable wiki articles in a local git repo at `~/.hivex/wiki/`. No API key required.
- `memanto` connects a self-hosted Memanto memory service for agent memory: verified learnings, facts, and insights with semantic retrieval. Point it at your instance with `HIVEX_MEMANTO_URL` and `HIVEX_MEMANTO_API_KEY`.
- `gbrain` mounts `gbrain serve` as the wiki backend.
- `none` disables the shared wiki entirely. Notebooks still work locally.

```bash
hivebot --memory-backend markdown
hivebot --memory-backend memanto
hivebot --memory-backend gbrain
hivebot --memory-backend none
```

Internal naming for code spelunkers: notebook = `private` memory, wiki = `shared` memory.

## Other Commands

```bash
hivebot init          # First-time setup
hivebot shred         # Kill a running session
hivebot --1o1         # 1:1 with the CEO
hivebot --1o1 cro     # 1:1 with a specific agent
```

## What You Should See

- A browser tab at `localhost:7891` with the office
- `#general` as the shared channel
- The team visible and working
- A composer to send messages and slash commands

If it feels like a hidden bot loop, something is wrong. If it feels like a real office, you're exactly where you need to be.

## Bridges

- **Telegram:** `/connect` → pick Telegram → paste bot token from [@BotFather](https://t.me/BotFather).
- **OpenClaw:** `/connect openclaw` → paste your gateway URL and `gateway.auth.token` from `~/.openclaw/openclaw.json`. Each OpenClaw session becomes a first-class office member you can `@mention`. If OpenClaw Gateway's OpenAI-compatible HTTP endpoint is enabled, use `--provider openclaw-http` to run hivebot-created bots through `http://127.0.0.1:18789/v1` with model `openclaw/default`.
- **Hermes Bot:** set `llm_provider` or `--provider` to `hermes-agent` to run hivebot bots through a local Hermes API server at `http://127.0.0.1:8642/v1`.

## External Actions

Two action providers ship by default — pick whichever fits your style.

### One CLI — local-first (default)

```
/config set action_provider one
```

### Composio — cloud-hosted

```
/config set composio_api_key <key>
/config set action_provider composio
```

## Why hivebot

| Feature | How it works |
|---|---|
| Sessions | Fresh per turn (no accumulated context) |
| Tools | Per-bot scoped (DM loads 4, full office loads 27) |
| Bot wakes | Push-driven (zero idle burn) |
| Live visibility | Stdout streaming |
| Mid-task steering | DM any bot, no restart |
| Runtimes | Mix Claude Code, Codex, Hermes Bot, and OpenClaw in one channel |
| Memory | Per-bot notebook + shared workspace wiki (knowledge graphs on GBrain or Memanto) |
| Price | Free to self-host (MIT, your API keys) |

## Benchmark

10-turn CEO session on Codex. All numbers measured from live runs.

| Metric | hivebot |
|---|---|
| Input per turn | Flat ~87k tokens |
| Billed per turn (after cache) | ~40k tokens |
| 10-turn total | ~286k tokens |
| Cache hit rate | 97% (Claude API prompt cache) |
| Claude Code cost (5-turn) | $0.06 |
| Idle token burn | Zero (push-driven, no polling) |

Accumulated-session orchestrators grow from 124k to 484k input per turn over the same session. hivebot stays flat.

## The Name

A hive: many agents, one shared office, work you can actually watch. hivebot — the bots that live in your hive. MIT-licensed, self-hosted, your keys.

## Links

- **Website:** https://hivex.team
- **Source:** https://github.com/Northlatch-Labs-LLC/hivebot
- **Issues:** https://github.com/Northlatch-Labs-LLC/hivebot/issues
- **Discord:** https://discord.gg/gjSySC3PzV
- **Architecture:** https://github.com/Northlatch-Labs-LLC/hivebot/blob/main/ARCHITECTURE.md
- **Forking guide:** https://github.com/Northlatch-Labs-LLC/hivebot/blob/main/FORKING.md

## Dev override

To point the wrapper at a locally-built binary, set `HIVEX_BINARY`:

```bash
HIVEX_BINARY=./hivex npx hivebot --version
```

## Auto-upgrade

`npm install -g` does not pull new versions on its own, so the wrapper
checks `registry.npmjs.org` once per 24h (cached at
`~/.hivex/cache/latest-version.json`). If a newer release is available it
downloads the matching binary into `~/.hivex/cache/binaries/` and runs it
instead — same SHA256 verification as `postinstall`. A one-line hint points
you at `npm install -g hivebot@latest` for a permanent upgrade.

Set `HIVEX_SKIP_VERSION_CHECK=1` to disable the check entirely.

MIT licensed. Free to self-host, your API keys.
