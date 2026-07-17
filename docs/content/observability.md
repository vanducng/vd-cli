---
title: "vd obs — observability"
---

`vd obs` reads the transcripts Claude Code and Codex already write on disk and turns them into sessions, token usage, and API-equivalent cost — in the terminal, in `vd web`, or over a local HTTP API. Read-only; it never touches your agent files. No collector, no daemon, no OTel backend to run.

## Commands

- `vd obs sessions [--agent claude-code|codex] [--project <p>] [--since 7d] [--json]` — one list across both agents: title, model, turns, tokens, estimated cost.
- `vd obs show <id-or-prefix> [--turns N] [--json]` — a session turn by turn: prompts, tool calls, hook timings, subagent rollups.
- `vd obs usage [--daily|--monthly] [--agent …] [--since 3d] [--json]` — tokens and estimated cost grouped by day or month, per model.

Costs are labeled **API-equivalent** — computed from token counts, not a subscription bill. An unpriced model renders `?`, never `$0.00`. Add rates in `~/.vd/obs/prices.json` to override the built-in table.

## Web portal

`vd web` serves the same data as a browser portal alongside the existing skills/hooks/doctor views:

- **Sessions** — filterable table (agent, since, project) with server-side paging.
- **Session transcript** (`/obs/sessions/:id`) — chat bubbles, tool blocks (errors expanded), a hook timeline that flags any PreToolUse hook over its 100ms budget, and subagent rollups. Deep-linkable.
- **Usage** — cost-over-time chart stacked by model, with a per-model breakdown and an explicit warning for any unpriced model.

## How it fits together

One Go core, three surfaces. Parsing and cost live in `obs.Service`; the CLI calls it in-process and the HTTP API serializes it, so the terminal and the portal always agree — they read the same numbers off the same cache.

```text
~/.claude/projects/*.jsonl   ~/.codex/sessions/*.jsonl
            │  read-only              │
            └──────────┬──────────────┘
                       ▼
        internal/obs/ingest  (parse, dedupe, resume)
                       ▼
        internal/obs/store   (modernc sqlite, ~/.vd/obs/obs.sqlite)
        internal/obs/pricing (model → cost, unknown = "?")
                       ▼
        internal/obs/service.go   obs.Service  ── the seam
              ┌────────────────────┴────────────────────┐
              ▼                                          ▼
     internal/cli/obs.go                internal/ui/web/obs_handlers.go
     vd obs sessions|show|usage         GET /api/obs/sessions|…/{id}|usage
                                                         ▼
                                            web/  React portal (SPA)
```

The cache is derived — every row rebuilds from the JSONL — so it self-heals if corrupt and drops-and-rebuilds on a schema change. Token accounting is validated against [ccusage](https://github.com/ryoppippi/ccusage) to within ~1% for same-universe models.

Design rationale and the layer map live in [ADR 0003 — goreact layering](https://github.com/vanducng/vd-cli/blob/main/docs/decisions/0003-goreact-layering.md).
