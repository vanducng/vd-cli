# codex-workflow

Deterministic multi-agent workflow orchestrator for Codex, exposed as an MCP server. It provides the **`run_workflow`** tool to Codex and Claude.

## Design (ADR-0002, refined)

A **deterministic sequencer**, not an Agents-SDK manager: `run_workflow` dispatches each step to Codex's own `codex` MCP tool (`codex mcp-server`), so all model work runs through your **Codex login** — no second API key, no extra LLM bill. Steps run sequentially; consecutive steps sharing a `parallel_group` run concurrently (capped by `[agents].max_threads`).

## Tool

```jsonc
run_workflow({
  "steps": [
    { "id": "scan",   "prompt": "List changed files vs main." },
    { "id": "review", "prompt": "Review for bugs.", "agent": "code-reviewer", "parallel_group": "g1" }
  ]
})
// → { "results": [ { "id", "status": "ok"|"error", "output" }, ... ] }
```

- `agent` (optional): a `~/.codex/agents/<name>.toml` role — its `developer_instructions` are injected as a prompt override (#26363 workaround).
- v1 = sequential + one parallel group + per-step results. No in-spec loops/conditionals (script those outside or chain calls).

## Run / test

Managed by `vd`: `vd mcp install codex-workflow` registers it into Codex + Claude.

```bash
uv sync
uv run pytest -q                 # unit tests (no codex needed)
CODEX_WORKFLOW_LIVE=1 uv run pytest -q   # + live smoke (needs codex login)
uv run codex-workflow-server     # run the MCP server directly (stdio)
```

Tested via `uv`/`pytest`, independent of the vd-cli Go test suite.

## Install (local)

Prereqs: [`uv`](https://docs.astral.sh/uv/), and `codex login` (the orchestrator drives `codex mcp-server` through your Codex login — no extra API key).

```bash
cd ~/vd-cli
go build -o ~/.local/bin/vd ./cmd/vd     # build the vd binary (prefix `env -u GOROOT` if GOROOT is mise-pinned)
vd mcp install codex-workflow            # Codex (~/.codex/config.toml) + Claude (project ./.mcp.json)
vd mcp install codex-workflow --scope user   # …or Claude global (~/.claude.json)
vd mcp doctor                            # sanity: command on PATH + env preflight
# restart Codex / Claude Code to load the new MCP server
```

Scope affects only the **Claude** target (`project` → `./.mcp.json`, `user` → `~/.claude.json`); **Codex** always uses its single `~/.codex/config.toml`. Manage with `vd mcp list|enable|disable|doctor`. The writer is surgical + backed up + re-parsed (auto-restore on bad output); no secrets are written (env is inherited at launch).

## Use it

After restart, the **`run_workflow`** tool is available in Codex and Claude.

Natural language — the agent calls the tool:
> "Use run_workflow to review each file in `internal/` in parallel and summarize."

Direct call:
```jsonc
run_workflow({
  "steps": [
    { "id": "scan",  "prompt": "List Go files changed vs main." },
    { "id": "rev-a", "prompt": "Review internal/extension for bugs.",    "agent": "code-reviewer", "parallel_group": "g1" },
    { "id": "rev-b", "prompt": "Review internal/claudeconfig for bugs.", "agent": "code-reviewer", "parallel_group": "g1" }
  ]
})
// → { "results": [ { "id", "status": "ok"|"error", "output" }, ... ] }
```

- `agent` (optional) = a `~/.codex/agents/<name>.toml` role (`code-reviewer`, `planner`, `tester`, …) — its `developer_instructions` are injected (#26363 workaround).
- Sequential by default; same `parallel_group` runs concurrently (cap = `[agents].max_threads`).
