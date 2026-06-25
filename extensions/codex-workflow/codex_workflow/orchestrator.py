"""Deterministic workflow sequencer over `codex mcp-server`.

No external LLM/SDK: each step is dispatched to Codex's own `codex` MCP tool,
so all model work runs through the user's Codex login. Steps run sequentially;
consecutive steps sharing a `parallel_group` run concurrently, capped by the
`[agents].max_threads` config value.
"""
from __future__ import annotations

import asyncio
import json
import logging
import os
import time
import tomllib
from pathlib import Path

from jsonschema import validate as _js_validate
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

_SCHEMA = json.loads((Path(__file__).resolve().parent.parent / "schema" / "run_workflow.v1.json").read_text())

# Agent-friendly, transparent log so the extension can be observed + improved.
# Convention: vd extensions log to ~/.vd/logs/<name>.log; surfaced by `vd mcp logs`.
_LOG_DIR = Path(os.environ.get("VD_LOG_DIR", Path.home() / ".vd" / "logs"))
log = logging.getLogger("codex-workflow")
if not log.handlers:
    try:
        _LOG_DIR.mkdir(parents=True, exist_ok=True)
        _h = logging.FileHandler(_LOG_DIR / "codex-workflow.log")
        _h.setFormatter(logging.Formatter("%(asctime)s %(levelname)-5s %(message)s"))
        log.addHandler(_h)
        log.setLevel(logging.INFO)
        log.propagate = False  # stdio MCP server: logs go ONLY to the file, never stdout/stderr
    except OSError:
        pass  # never let logging break the server


class _DropNotifValidationWarning(logging.Filter):
    """Drop the mcp SDK's root-logger 'Failed to validate notification' warning:
    Codex emits custom codex/event notifications the SDK can't validate against
    its newer task schema — informational + non-fatal, but noisy on every turn."""

    def filter(self, record: logging.LogRecord) -> bool:
        return "Failed to validate notification" not in record.getMessage()


logging.getLogger().addFilter(_DropNotifValidationWarning())


def _max_threads(default: int = 4) -> int:
    cfg = Path.home() / ".codex" / "config.toml"
    try:
        return int(tomllib.loads(cfg.read_text()).get("agents", {}).get("max_threads", default))
    except Exception:
        return default


def _toml_inline(v) -> str:
    """Minimal TOML inline serializer for the mcp_servers subset (str/int/bool/
    list/dict). Keys are quoted so server names with dashes are safe."""
    if isinstance(v, str):
        return '"' + v.replace("\\", "\\\\").replace('"', '\\"') + '"'
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (int, float)):
        return str(v)
    if isinstance(v, list):
        return "[" + ", ".join(_toml_inline(x) for x in v) + "]"
    if isinstance(v, dict):
        return "{" + ", ".join(f'"{k}" = {_toml_inline(x)}' for k, x in v.items()) + "}"
    return '""'


def _nested_mcp_override(requested: list[str] | None) -> str:
    """TOML value for the nested codex's `-c mcp_servers=...`.

    Default `{}` drops all downstream servers (lean + recursion-safe). When
    `requested` names servers, only those are passed through from the user's
    Codex config — `codex-workflow` is ALWAYS excluded so the orchestrator can
    never re-spawn itself.
    """
    if not requested:
        return "{}"
    try:
        servers = tomllib.loads((Path.home() / ".codex" / "config.toml").read_text()).get("mcp_servers", {})
    except Exception:
        servers = {}
    subset = {}
    for name in requested:
        if name == "codex-workflow":
            continue
        if name in servers:
            subset[name] = servers[name]
        else:
            log.warning("requested mcp_server %r not in ~/.codex/config.toml — skipped", name)
    return _toml_inline(subset)


def _agent_instructions(name: str) -> str | None:
    """developer_instructions of a ~/.codex/agents/<name>.toml role, if present.

    #26363 workaround: custom agents aren't selectable at spawn, so we inject
    the role's instructions as a prompt override.
    """
    p = Path.home() / ".codex" / "agents" / f"{name}.toml"
    try:
        return tomllib.loads(p.read_text()).get("developer_instructions")
    except Exception:
        return None


def _step_prompt(step: dict) -> str:
    prompt = step["prompt"]
    if agent := step.get("agent"):
        if instr := _agent_instructions(agent):
            return f"You are acting as the '{agent}' agent. Follow these instructions:\n\n{instr}\n\n---\n\nTask:\n{prompt}"
    return prompt


def _result_text(call_result) -> str:
    parts = []
    for item in getattr(call_result, "content", []) or []:
        text = getattr(item, "text", None)
        if text:
            parts.append(text)
    return "\n".join(parts).strip()


async def _run_step(session: ClientSession, step: dict) -> dict:
    sid = step["id"]
    t0 = time.monotonic()
    log.info("step start id=%s agent=%s", sid, step.get("agent") or "-")
    try:
        res = await session.call_tool(
            "codex",
            {"prompt": _step_prompt(step), "approval-policy": "never", "sandbox": "workspace-write"},
        )
        status = "error" if getattr(res, "isError", False) else "ok"
        out = _result_text(res) or ("tool error" if status == "error" else "")
        log.info("step done  id=%s status=%s dur=%.1fs out=%dch", sid, status, time.monotonic() - t0, len(out))
        return {"id": sid, "status": status, "output": out}
    except Exception as exc:  # surface terminal errors per-step, don't abort the run
        log.error("step fail  id=%s %s: %s", sid, type(exc).__name__, exc)
        return {"id": sid, "status": "error", "output": f"{type(exc).__name__}: {exc}"}


def _batches(steps: list[dict]) -> list[list[dict]]:
    """Group consecutive steps with the same non-empty parallel_group; everything
    else is its own sequential batch."""
    out: list[list[dict]] = []
    for step in steps:
        grp = step.get("parallel_group")
        if grp and out and out[-1] and out[-1][0].get("parallel_group") == grp:
            out[-1].append(step)
        else:
            out.append([step])
    return out


async def run_workflow_spec(spec: dict) -> dict:
    _js_validate(instance=spec, schema=_SCHEMA)
    cap = _max_threads()
    sem = asyncio.Semaphore(cap)
    override = _nested_mcp_override(spec.get("mcp_servers"))
    log.info("run_workflow start steps=%d cap=%d mcp=%s", len(spec["steps"]), cap, spec.get("mcp_servers") or "none")

    # Recursion guard: the nested `codex mcp-server` must NOT re-load codex-workflow
    # (it would spawn us again → runaway). The override drops codex-workflow (and,
    # by default, all other downstream servers); opt in to specific servers via the
    # spec's `mcp_servers`. This is what makes the `codex` target safe.
    params = StdioServerParameters(command="codex", args=["mcp-server", "-c", f"mcp_servers={override}"])
    results: list[dict] = []
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()

            async def guarded(step: dict) -> dict:
                async with sem:
                    return await _run_step(session, step)

            for batch in _batches(spec["steps"]):
                if len(batch) == 1:
                    results.append(await guarded(batch[0]))
                else:
                    results.extend(await asyncio.gather(*(guarded(s) for s in batch)))

    ok = sum(1 for r in results if r["status"] == "ok")
    log.info("run_workflow done %d/%d ok", ok, len(results))
    return {"results": results}
