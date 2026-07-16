package store

import (
	"context"
	"fmt"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// Watermark is what lets a sync skip an unchanged file, and resume a growing one
// mid-line rather than reparsing it.
type Watermark struct {
	Path        string
	ByteOffset  int64
	MTime       int64
	Size        int64
	ParserState string
}

// GetWatermark returns the stored watermark for path; ok is false when unseen.
func (s *Store) GetWatermark(ctx context.Context, path string) (Watermark, bool, error) {
	w := Watermark{Path: path}
	err := s.db.QueryRowContext(ctx,
		"SELECT byte_offset, mtime, size, parser_state FROM ingest_state WHERE path = ?", path).
		Scan(&w.ByteOffset, &w.MTime, &w.Size, &w.ParserState)
	if err != nil {
		if isNoRows(err) {
			return Watermark{Path: path}, false, nil
		}
		return w, false, fmt.Errorf("read watermark: %w", err)
	}
	return w, true, nil
}

// IngestFile writes one parsed transcript and advances its watermark in a single
// transaction: a unit of work is a store method, so callers never hold a *sql.Tx.
func (s *Store) IngestFile(ctx context.Context, rec model.Record, w Watermark) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ingest tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	sess := rec.Session
	// Upsert on the transcript-native id: re-ingesting a file that grew is a
	// no-op for rows already written, which is what makes watermarks safe.
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions
		(id, agent, title, cwd, project, git_branch, git_sha, model, cli_version,
		 originator, workflow_id, parent_id, started_at, ended_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title, model=excluded.model, ended_at=excluded.ended_at,
			git_branch=excluded.git_branch, git_sha=excluded.git_sha,
			cli_version=excluded.cli_version, cwd=excluded.cwd, project=excluded.project,
			originator=excluded.originator, workflow_id=excluded.workflow_id,
			-- parent_id is learned late: a subagent is parsed before the parent span
			-- that links it, and dropping the update leaks it into the session list.
			-- COALESCE so a later parentless re-parse cannot unlink it again.
			parent_id=CASE WHEN excluded.parent_id != '' THEN excluded.parent_id ELSE sessions.parent_id END`,
		sess.ID, sess.Agent, sess.Title, sess.CWD, sess.Project, sess.GitBranch, sess.GitSHA,
		sess.Model, sess.CLIVersion, sess.Originator, sess.WorkflowID, sess.ParentID,
		timeToMs(sess.StartedAt), timeToMs(sess.EndedAt)); err != nil {
		return fmt.Errorf("upsert session %s: %w", sess.ID, err)
	}

	for _, t := range rec.Turns {
		if _, err := tx.ExecContext(ctx, `INSERT INTO turns
			(id, session_id, idx, model, started_at, duration_ms, input, output,
			 cache_read, cache_write, reasoning)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
				duration_ms=excluded.duration_ms, input=excluded.input, output=excluded.output,
				cache_read=excluded.cache_read, cache_write=excluded.cache_write,
				reasoning=excluded.reasoning`,
			t.ID, t.SessionID, t.Index, t.Model, timeToMs(t.StartedAt), t.DurationMs,
			t.Tokens.Input, t.Tokens.Output, t.Tokens.CacheRead, t.Tokens.CacheWrite,
			t.Tokens.ReasoningOutput); err != nil {
			return fmt.Errorf("upsert turn %s: %w", t.ID, err)
		}
		if t.PromptText != "" || t.ResponseText != "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO turn_payloads (turn_id, prompt_text, response_text)
				VALUES (?,?,?) ON CONFLICT(turn_id) DO UPDATE SET
					prompt_text=excluded.prompt_text, response_text=excluded.response_text`,
				t.ID, truncateMid(t.PromptText, MaxPayloadBytes),
				truncateMid(t.ResponseText, MaxPayloadBytes)); err != nil {
				return fmt.Errorf("upsert turn payload %s: %w", t.ID, err)
			}
		}
	}

	for _, sp := range rec.ToolSpans {
		ok := 0
		if sp.OK {
			ok = 1
		}
		var roll model.TokenUsage
		if sp.RollupTokens != nil {
			roll = *sp.RollupTokens
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO tool_spans
			(id, turn_id, session_id, name, kind, duration_ms, ok, subagent_session_id,
			 subagent_name, rollup_input, rollup_output, rollup_cache_read, rollup_cache_write)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
				duration_ms=excluded.duration_ms, ok=excluded.ok, name=excluded.name,
				kind=excluded.kind, subagent_session_id=excluded.subagent_session_id,
				subagent_name=excluded.subagent_name, rollup_input=excluded.rollup_input,
				rollup_output=excluded.rollup_output, rollup_cache_read=excluded.rollup_cache_read,
				rollup_cache_write=excluded.rollup_cache_write`,
			sp.ID, sp.TurnID, sess.ID, sp.Name, sp.Kind, sp.DurationMs, ok,
			sp.SubagentSessionID, sp.SubagentName, roll.Input, roll.Output,
			roll.CacheRead, roll.CacheWrite); err != nil {
			return fmt.Errorf("upsert tool span %s: %w", sp.ID, err)
		}
		if sp.Input != "" || sp.Output != "" || sp.Error != "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO tool_span_payloads (span_id, input, output, error)
				VALUES (?,?,?,?) ON CONFLICT(span_id) DO UPDATE SET
					input=excluded.input, output=excluded.output, error=excluded.error`,
				sp.ID, truncateMid(sp.Input, MaxPayloadBytes),
				truncateMid(sp.Output, MaxPayloadBytes),
				truncateMid(sp.Error, MaxPayloadBytes)); err != nil {
				return fmt.Errorf("upsert span payload %s: %w", sp.ID, err)
			}
		}
	}

	for _, h := range rec.HookExecs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO hook_execs
			(turn_id, session_id, hook_name, event, duration_ms, exit_code) VALUES (?,?,?,?,?,?)
			ON CONFLICT(turn_id, hook_name, event) DO UPDATE SET
				duration_ms=excluded.duration_ms, exit_code=excluded.exit_code`,
			h.TurnID, sess.ID, h.HookName, h.Event, h.DurationMs, h.ExitCode); err != nil {
			return fmt.Errorf("upsert hook exec: %w", err)
		}
	}

	for _, sk := range rec.Skills {
		if _, err := tx.ExecContext(ctx, `INSERT INTO skill_invocations
			(turn_id, session_id, name, args) VALUES (?,?,?,?)
			ON CONFLICT(turn_id, name) DO UPDATE SET args=excluded.args`,
			sk.TurnID, sess.ID, sk.Name, sk.Args); err != nil {
			return fmt.Errorf("upsert skill: %w", err)
		}
	}

	if w.Path != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO ingest_state
			(path, byte_offset, mtime, size, parser_state) VALUES (?,?,?,?,?)
			ON CONFLICT(path) DO UPDATE SET
				byte_offset=excluded.byte_offset, mtime=excluded.mtime,
				size=excluded.size, parser_state=excluded.parser_state`,
			w.Path, w.ByteOffset, w.MTime, w.Size, w.ParserState); err != nil {
			return fmt.Errorf("advance watermark %s: %w", w.Path, err)
		}
	}
	return tx.Commit()
}

// Reset drops every derived row for a --full rebuild.
func (s *Store) Reset(ctx context.Context) error {
	if err := dropAll(s.db); err != nil {
		return err
	}
	return ensureSchema(s.db)
}
