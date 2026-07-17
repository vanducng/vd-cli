---
title: "vd obs — observability"
---

`vd obs` reads the transcripts Claude Code and Codex already write on disk and turns them into sessions, token usage, and API-equivalent cost — in the terminal, in `vd web`, or over a local HTTP API. Read-only; it never touches your agent files. No collector, no daemon, no OTel backend to run.

## Commands

- `vd obs sessions [--agent claude-code|codex] [--project <p>] [--since 7d] [--json]` — one list across both agents: title, model, turns, tokens, estimated cost. Codex rollouts carry no title of their own, so their title is derived from the first prompt (secret-redacted, truncated to 80 chars).
- `vd obs show <id-or-prefix> [--turns N] [--json]` — a session turn by turn: prompts, tool calls, hook timings, subagent rollups.
- `vd obs usage [--daily|--monthly] [--agent …] [--since 3d] [--json]` — tokens and estimated cost grouped by day or month, per model.
- `vd obs skills [--agent …] [--project <p>] [--since …] [--json]` — per-skill tool calls, error rate, corrections, aborts and tokens across both agents.
- `vd obs hooks [--agent …] [--project <p>] [--since …] [--json]` — hook fire counts, block rates and their share of same-turn tool errors (Claude-only).
- `vd obs health [--agent …] [--project <p>] [--since 7d] [--json]` — recurring tool-error clusters ranked by count, with a variant breakdown, fetchable evidence refs, co-occurring skill file paths and a machine-readable diagnosis. An **investigate signal, not a health verdict** — agents fail-probe routinely, so a count says "look here", never "this is broken". `--json` is the self-heal entry point: an agent picks a cluster, fetches evidence via `vd obs show <sessionid> --json`, edits the linked skill file, then re-checks with a tight post-fix `--since` window (the cluster `signature` is stable across runs, so it doubles as the tracking key).
- `vd obs sync [--full] [--agent …] [--since …]` — fold new or changed transcripts into the cache. `--full` drops the cache and re-reads everything (use it after upgrading past an ingest change so historical rollouts are re-parsed).

Costs are labeled **API-equivalent** — computed from token counts, not a subscription bill. An unpriced model renders `?`, never `$0.00`. Add rates in `~/.vd/obs/prices.json` to override the built-in table.

### Skill attribution

`vd obs skills` attributes work **per invocation**: a skill owns the turns from its invocation to the next invocation in the same session (or session end); the tool calls, errors, tokens and correction/abort signals inside that window count toward it. Activity before any invocation, or in a session that invoked no skill, lands in the `(none)` bucket. Counting by session broadcast instead overcounts several-fold, so the rollup never does. `CORR` (user push-backs) and `ABRT` (interrupt marker) are query-time correctness proxies — they flag candidates, not proven fault, and no raw prompt text ever enters an aggregate. Codex skill invocations come from the `$name` / `$vd:name` text convention, validated against the installed-skill registry so shell noise never counts.

## Web portal

`vd web` serves the same data as a browser portal alongside the existing skills/hooks/doctor views:

- **Sessions** — filterable table (agent, since, project) with server-side paging.
- **Session transcript** (`/obs/sessions/:id`) — chat bubbles, tool blocks (errors expanded), a hook timeline that flags any PreToolUse hook over its 100ms budget, and subagent rollups. Deep-linkable.
- **Usage** — cost-over-time chart stacked by model, with a per-model breakdown and an explicit warning for any unpriced model.
- **Skill health** (`/obs/skills`) — the per-invocation skills rollup, same columns as `vd obs skills`, with agent and since filters.

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
     vd obs sessions|show|usage|        GET /api/obs/sessions|…/{id}|usage|
     skills|hooks|sync                      skills|hooks
                                                         ▼
                                            web/  React portal (SPA)
```

The cache is derived — every row rebuilds from the JSONL — so it self-heals if corrupt and drops-and-rebuilds on a schema change. Token accounting is validated against [ccusage](https://github.com/ryoppippi/ccusage) to within ~1% for same-universe models.

Design rationale and the layer map live in [ADR 0003 — goreact layering](https://github.com/vanducng/vd-cli/blob/main/docs/decisions/0003-goreact-layering.md).
