# 0002 — Codex dynamic-workflow capability via OpenAI Agents SDK

- **Status:** Accepted
- **Date:** 2026-06-25
- **Deciders:** vanducng
- **Depends on:** [0001 — MCP servers as vd-cli extensions](0001-mcp-servers-as-vd-cli-extensions.md)

## Context

We want Codex to have a dynamic-workflow capability analogous to Claude Code's **Workflow** tool — deterministic orchestration of subagents (`agent()` / `parallel()` / `pipeline()`) with schema-validated structured output. Codex (0.142.0) already ships the primitives:

- **Native subagents** via `~/.codex/agents/*.toml` (we already have 14), but a live regression — [openai/codex#26363](https://github.com/openai/codex/issues/26363) since v0.137.0 — makes custom agent TOMLs **not selectable at spawn** (they fall back to generic threads).
- **`spawn_agents_on_csv`** — deterministic per-row fan-out.
- **`codex exec --json --output-schema`** — headless, schema-validated, resumable.
- **`codex mcp-server`** — exposes Codex as MCP (`codex`, `codex-reply` tools); pairs with the **OpenAI Agents SDK** for deterministic, traced orchestration.

The deciding constraint: prioritize **native + long-lived + decoupled from vd-cli core development**, accepting a higher runtime cost for a lower maintenance burden.

## Decision

Adopt the **OpenAI Agents SDK + `codex mcp-server`** path, packaged as an MCP server under the extensions mechanism:

- `~/vd-cli/extensions/codex-workflow/` — a **Python** MCP server (OpenAI Agents SDK) that **exposes a `run_workflow` tool** to Codex and Claude, and **internally** drives `codex mcp-server` (launched as its subprocess via `MCPServerStdio`) to run sequential handoffs, parallel fan-out, and gated pipelines.
- Hosted + registered via [ADR-0001](0001-mcp-servers-as-vd-cli-extensions.md): `vd mcp install` registers it into both runtimes.
- The heavy orchestration logic is **decoupled** from the Go `vd` core and **independently versioned**; vd only registers it. Officialness + the MCP standard give longevity; OpenAI Traces give audit trails.

Two supporting changes, independent of the orchestrator:

1. **Agent TOMLs become source-controlled + deployed.** Move the 14 `~/.codex/agents/*.toml` to `~/skills/agents/` and extend `internal/install/codex.go` so `vd install codex` deploys them to `~/.codex/agents/`. (Today they are untracked machine state.)
2. **`[agents]` config block** added to `~/.codex/config.toml` (dotfiles source): `max_threads = 4`, `max_depth = 1`, `job_max_runtime_seconds = 900`.

## Consequences

- **Accepted lock-in:** a Python runtime + `openai-agents` dependency + OpenAI-SDK coupling. This is a conscious tradeoff — chosen for official support, MCP longevity, and decoupling from vd-cli development, over the "no new runtime / no lock-in" of a custom Go driver.
- Immune to regression #26363: orchestration goes through `codex mcp-server`, not the in-session custom-agent spawner.
- Callable **inline** from a Codex (or Claude) turn as the `run_workflow` MCP tool — closer to Claude's inline `Workflow()` than a shell command would be.
- Token cost is real and linear; orchestration raises throughput per reviewer, not reviewer capacity — keep fan-out at 3–5 concurrent.
- A `codex-workflow` **skill** in `~/skills` teaches when/how to invoke it and the native in-chat fallbacks (`spawn_agents_on_csv`, `worktree + codex exec &`).

## Alternatives rejected

- **A — skill-only natural-language orchestration** — non-deterministic (model decides), and currently broken by #26363. Recreates the "orchestration-by-prompt" problem Claude's Workflow exists to replace. (Kept only as an in-chat *fallback* documented in the skill, not the substrate.)
- **B — custom `vd workflow` (Go) driving `codex exec --json --output-schema`** — deterministic and lock-in-free, but more code to maintain, runs out-of-chat, and couples orchestration to vd-cli releases. Rejected in favor of the official, decoupled path.
- **FableCodex** — a workflow-_discipline_ gate (goal ledger + findings), not an orchestrator. May be adopted separately as an optional discipline skill; does not satisfy this decision.

## References

- [Subagents — Codex](https://developers.openai.com/codex/subagents) · [Use Codex with the Agents SDK](https://developers.openai.com/codex/guides/agents-sdk) · [Non-interactive mode](https://developers.openai.com/codex/noninteractive)
- Regression: [openai/codex#26363](https://github.com/openai/codex/issues/26363)
- Internal: research + brainstorm briefs (`~/skills/.workbench/_global/scratch/reports/`, 2026-06-25)
