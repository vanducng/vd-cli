"""Deterministic workflow sequencer over `codex mcp-server`.

No external LLM/SDK: each step is dispatched to Codex's own `codex` MCP tool,
so all model work runs through the user's Codex login. Steps run sequentially;
consecutive steps sharing a `parallel_group` run concurrently, capped by the
`[agents].max_threads` config value.
"""
from __future__ import annotations

import asyncio
import json
import os
import tomllib
from pathlib import Path

from jsonschema import validate as _js_validate
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

_SCHEMA = json.loads((Path(__file__).resolve().parent.parent / "schema" / "run_workflow.v1.json").read_text())


def _max_threads(default: int = 4) -> int:
    cfg = Path.home() / ".codex" / "config.toml"
    try:
        return int(tomllib.loads(cfg.read_text()).get("agents", {}).get("max_threads", default))
    except Exception:
        return default


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
    try:
        res = await session.call_tool(
            "codex",
            {"prompt": _step_prompt(step), "approval-policy": "never", "sandbox": "workspace-write"},
        )
        if getattr(res, "isError", False):
            return {"id": step["id"], "status": "error", "output": _result_text(res) or "tool error"}
        return {"id": step["id"], "status": "ok", "output": _result_text(res)}
    except Exception as exc:  # surface terminal errors per-step, don't abort the run
        return {"id": step["id"], "status": "error", "output": f"{type(exc).__name__}: {exc}"}


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

    params = StdioServerParameters(command="codex", args=["mcp-server"])
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

    return {"results": results}
