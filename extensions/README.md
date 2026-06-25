# extensions/

Self-contained MCP servers/services managed by vd. Each subdir is one extension with an `extension.toml` manifest. vd is the **manager** — it registers these into Codex (`~/.codex/config.toml` `[mcp_servers]`) and Claude (`~/.claude.json` / `.mcp.json`) and runs lifecycle (`vd mcp list|install|enable|disable|doctor`). vd does **not** host them — they run in their own runtime (Python/uv, Node, …). See [ADR-0001](../docs/decisions/0001-mcp-servers-as-vd-cli-extensions.md).

## extension.toml

```toml
name = "codex-workflow"
description = "…"
transport = "stdio"            # stdio | http
command = "uv"                 # stdio
args = ["run", "--directory", "{dir}", "codex-workflow-server"]  # {dir} → this extension's abs dir
# url = "http://127.0.0.1:7878"  # http transport instead
env = ["OPENAI_API_KEY"]       # var NAMES only — preflight/doctor; never written to config
targets = ["codex", "claude"]
scope = "project"              # default registration scope: project | user | global
startup_timeout_sec = 120
enabled = true
```

Secrets are never written to config: the `env` names drive preflight checks only; the spawned process inherits the parent environment.

## Extensions

- **codex-workflow** — deterministic multi-agent workflow orchestrator over `codex mcp-server` (the `run_workflow` tool). See [ADR-0002](../docs/decisions/0002-codex-dynamic-workflow-via-agents-sdk.md).
