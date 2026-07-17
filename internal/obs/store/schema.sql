-- obs.sqlite is a derived cache: every row is rebuildable from the JSONL on disk.
-- On user_version mismatch the whole file is dropped and re-synced, so there is no
-- migration ladder here. Timestamps are INTEGER unix-millis UTC: they sort and
-- range-query natively, and usage is all time-bucketed aggregation.

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    agent       TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    cwd         TEXT NOT NULL DEFAULT '',
    project     TEXT NOT NULL DEFAULT '',
    git_branch  TEXT NOT NULL DEFAULT '',
    git_sha     TEXT NOT NULL DEFAULT '',
    model       TEXT NOT NULL DEFAULT '',
    cli_version TEXT NOT NULL DEFAULT '',
    originator  TEXT NOT NULL DEFAULT '',
    workflow_id TEXT NOT NULL DEFAULT '',
    parent_id   TEXT NOT NULL DEFAULT '',
    started_at  INTEGER NOT NULL DEFAULT 0,
    ended_at    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_parent  ON sessions(parent_id);

CREATE TABLE IF NOT EXISTS turns (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL,
    idx         INTEGER NOT NULL DEFAULT 0,
    model       TEXT NOT NULL DEFAULT '',
    started_at  INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    input       INTEGER NOT NULL DEFAULT 0,
    output      INTEGER NOT NULL DEFAULT 0,
    cache_read  INTEGER NOT NULL DEFAULT 0,
    cache_write INTEGER NOT NULL DEFAULT 0,
    reasoning   INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_turns_session ON turns(session_id);

-- Payloads live apart from turns/tool_spans so that usage aggregation never scans
-- 32KB blobs it does not read.
CREATE TABLE IF NOT EXISTS turn_payloads (
    turn_id       TEXT PRIMARY KEY,
    prompt_text   TEXT NOT NULL DEFAULT '',
    response_text TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tool_spans (
    id                  TEXT PRIMARY KEY,
    turn_id             TEXT NOT NULL,
    session_id          TEXT NOT NULL,
    name                TEXT NOT NULL DEFAULT '',
    kind                TEXT NOT NULL DEFAULT '',
    duration_ms         INTEGER NOT NULL DEFAULT 0,
    ok                  INTEGER NOT NULL DEFAULT 1,
    subagent_session_id TEXT NOT NULL DEFAULT '',
    subagent_name       TEXT NOT NULL DEFAULT '',
    rollup_input        INTEGER NOT NULL DEFAULT 0,
    rollup_output       INTEGER NOT NULL DEFAULT 0,
    rollup_cache_read   INTEGER NOT NULL DEFAULT 0,
    rollup_cache_write  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tool_spans_turn ON tool_spans(turn_id);

CREATE TABLE IF NOT EXISTS tool_span_payloads (
    span_id TEXT PRIMARY KEY,
    input   TEXT NOT NULL DEFAULT '',
    output  TEXT NOT NULL DEFAULT '',
    error   TEXT NOT NULL DEFAULT ''
);

-- seq disambiguates repeats: PostToolUse-class hooks fire once per tool call,
-- so (turn, hook, event) alone collapses N fires into one row.
CREATE TABLE IF NOT EXISTS hook_execs (
    turn_id     TEXT NOT NULL,
    session_id  TEXT NOT NULL,
    hook_name   TEXT NOT NULL,
    event       TEXT NOT NULL,
    seq         INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    exit_code   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (turn_id, hook_name, event, seq)
);

CREATE TABLE IF NOT EXISTS skill_invocations (
    turn_id    TEXT NOT NULL,
    session_id TEXT NOT NULL,
    name       TEXT NOT NULL,
    seq        INTEGER NOT NULL DEFAULT 0,
    args       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (turn_id, name, seq)
);

-- Watermarks. byte_offset + parser_state let a growing transcript resume mid-file.
CREATE TABLE IF NOT EXISTS ingest_state (
    path         TEXT PRIMARY KEY,
    byte_offset  INTEGER NOT NULL DEFAULT 0,
    mtime        INTEGER NOT NULL DEFAULT 0,
    size         INTEGER NOT NULL DEFAULT 0,
    parser_state TEXT NOT NULL DEFAULT ''
);
