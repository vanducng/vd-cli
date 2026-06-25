import asyncio
import os

import pytest

from codex_workflow.orchestrator import _batches, run_workflow_spec


def test_batches_grouping():
    steps = [
        {"id": "a", "prompt": "x"},
        {"id": "b", "prompt": "y", "parallel_group": "g1"},
        {"id": "c", "prompt": "z", "parallel_group": "g1"},
        {"id": "d", "prompt": "w"},
    ]
    assert [len(b) for b in _batches(steps)] == [1, 2, 1]


def test_schema_rejects_empty_steps():
    with pytest.raises(Exception):
        asyncio.run(run_workflow_spec({"steps": []}))


def test_schema_rejects_missing_prompt():
    with pytest.raises(Exception):
        asyncio.run(run_workflow_spec({"steps": [{"id": "a"}]}))


@pytest.mark.skipif(
    not os.getenv("CODEX_WORKFLOW_LIVE"),
    reason="live smoke needs a codex login; set CODEX_WORKFLOW_LIVE=1 to run",
)
def test_live_smoke_two_step_parallel():
    res = asyncio.run(
        run_workflow_spec(
            {
                "steps": [
                    {"id": "s1", "prompt": "Reply with exactly: ping", "parallel_group": "g"},
                    {"id": "s2", "prompt": "Reply with exactly: pong", "parallel_group": "g"},
                ]
            }
        )
    )
    assert len(res["results"]) == 2
    assert all("output" in r for r in res["results"])
