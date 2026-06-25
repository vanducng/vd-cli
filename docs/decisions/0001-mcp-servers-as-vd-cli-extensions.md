# 0001 — MCP servers/services managed as vd-cli extensions

- **Status:** Accepted
- **Date:** 2026-06-25
- **Deciders:** vanducng

## Context

We want a single control plane (`vd`) to manage MCP servers and related local services across both agent runtimes — **Codex** (`~/.codex/config.toml` `[mcp_servers.*]`) and **Claude Code** (`mcpServers` in `~/.claude.json` for user scope, or `.mcp.json` for project scope). Today vd-cli installs **skills** and **hooks** but touches **zero** MCP configuration; MCP servers are registered by hand in two different files and formats, with no source of truth, no lifecycle, and no parity between the two runtimes.

The immediate driver is a Codex dynamic-workflow orchestrator (see [0002](0002-codex-dynamic-workflow-via-agents-sdk.md)), but the need is general: any number of MCP servers/services will want the same install/register/enable/disable/doctor treatment. MCP servers are **polyglot** (Python, Node, Go), so they cannot be compiled into the Go `vd` binary.

## Decision

**vd-cli is the _manager_ of MCP servers, not their _host_.** The servers are self-contained units described by a manifest; vd registers and lifecycles them.

1. **Home:** a top-level `extensions/` directory in the vd-cli repo. Each subdir is one self-contained MCP server/service:
   ```
   ~/vd-cli/extensions/<name>/
     extension.toml        # manifest (the contract)
     <server impl + its own runtime files: pyproject.toml, package.json, …>
   ```
   - Named **`extensions/`** (not `plugins/`, which collides with `codex plugin` / `claude plugin` terminology).
   - Lives in the **vd-cli repo** (not `~/skills`): these are runnable services tied to the control plane, versioned and released *with* `vd` — distinct from the declarative markdown capabilities in `~/skills`.

2. **Manifest** — `extension.toml` (TOML, matching Codex config + agent TOMLs):
   ```toml
   name = "codex-workflow"
   description = "…"
   transport = "stdio"            # stdio | http
   command = "uv"                 # stdio: command + args
   args = ["run", "server.py"]
   # url = "http://127.0.0.1:7878"  # http transport instead
   env = ["OPENAI_API_KEY"]       # env var names passed through (never values)
   targets = ["codex", "claude"]  # runtimes to register into
   scope = "project"              # default scope; overridable at install time
   startup_timeout_sec = 120
   enabled = true
   ```

3. **vd-cli changes:**
   - New `internal/extension` package: discover + parse `extensions/*/extension.toml`.
   - Extend `internal/claudeconfig` to write MCP registration into **both** Codex `config.toml [mcp_servers.<name>]` and Claude `mcpServers."<name>"`, idempotently, with a timestamped backup (same discipline as the hook installer).
   - New `vd mcp` command group: `list`, `install`, `enable`, `disable`, `doctor`.

4. **Scope:** registration defaults to **project** level (Codex project config / `.mcp.json`), with an explicit `--scope user|global` option to register at the user/global level (`~/.claude.json`, `~/.codex/config.toml`). Project-by-default keeps machine-global config clean and makes per-repo MCP sets reproducible.

## Consequences

- The `vd` binary stays lean — no embedded servers; polyglot is supported by construction.
- Extensions version and release on the same path as `vd`; one `vd mcp install` provisions both runtimes with parity.
- New responsibility: vd must write two config formats correctly and idempotently, and back them up before edits.
- Secrets are never stored in manifests **or written to the runtime config**. The manifest's `env = [names]` is used only for **preflight/doctor checks** (warn when a required var is unset); vd does **not** emit an `env`/`[mcp_servers.<name>.env]` table — the spawned stdio process inherits the parent environment at launch. (Codex's `[mcp_servers.<name>.env]` is a `KEY="value"` map; writing resolved values would leak secrets into a dotfiles-tracked config, so it is excluded by design.)
- Reversible: deleting an extension dir + `vd mcp disable` unregisters cleanly; no lock-in at this layer.

## Alternatives rejected

- **Embed MCP servers as Go packages in the binary** — impossible for polyglot servers; bloats `vd`; couples unrelated runtimes.
- **Name it `plugins/`** — collides with `codex plugin` and `claude plugin`; "extension" is unambiguous and covers non-MCP services too.
- **Host the servers in `~/skills/extensions/`** — splits platform (runnable services) from content (markdown capabilities). `~/skills` stays declarative; executable services belong with the control plane.
- **YAML manifest** — TOML matches the surrounding ecosystem (Codex `config.toml`, agent TOMLs); one fewer format in the head.
- **Register only Codex (or only Claude)** — parity across both runtimes is the whole point of a single manager.

## References

- Codex MCP config: `~/.codex/config.toml` `[mcp_servers.*]` · Claude MCP config: `~/.claude.json` `mcpServers`, project `.mcp.json`
- Consumer of this mechanism: [0002 — Codex dynamic-workflow via Agents SDK](0002-codex-dynamic-workflow-via-agents-sdk.md)
