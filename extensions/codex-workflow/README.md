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
