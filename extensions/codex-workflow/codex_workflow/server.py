"""MCP server exposing the run_workflow tool to Codex and Claude."""
from __future__ import annotations

from mcp.server.fastmcp import FastMCP

from .orchestrator import run_workflow_spec

mcp = FastMCP("codex-workflow")


@mcp.tool()
async def run_workflow(spec: dict) -> dict:
    """Run a deterministic multi-step workflow over Codex.

    spec = {"steps": [{"id", "prompt", "agent"?, "parallel_group"?}, ...]}.
    Sequential by default; steps sharing a parallel_group run concurrently.
    Returns {"results": [{"id", "status", "output"}, ...]} — one per step.
    Errors are surfaced per-step (status="error"), never aborting the run.
    """
    return await run_workflow_spec(spec)


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()
