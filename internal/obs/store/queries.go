package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

func msToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

func timeToMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// likeEscape neutralizes LIKE wildcards in user input. Without it `q=%` matches
// every session while looking like a filter — the exact "silently return
// unfiltered data as if the filter applied" failure the contract forbids.
func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// buildSessionWhere is shared by ListSessions and CountSessions so a page and its
// total can never disagree about what was filtered.
func buildSessionWhere(f model.SessionFilter) (string, []any) {
	// Subagent transcripts contribute usage but are not sessions in their own right.
	conds := []string{"parent_id = ''"}
	var args []any

	if f.Agent != "" {
		conds = append(conds, "agent = ?")
		args = append(args, f.Agent)
	}
	if f.Project != "" {
		conds = append(conds, `(project = ? OR cwd LIKE ? ESCAPE '\')`)
		args = append(args, f.Project, likeEscape(f.Project)+"%")
	}
	if f.Q != "" {
		conds = append(conds, `(title LIKE ? ESCAPE '\' OR cwd LIKE ? ESCAPE '\')`)
		args = append(args, "%"+likeEscape(f.Q)+"%", "%"+likeEscape(f.Q)+"%")
	}
	if !f.Since.IsZero() {
		conds = append(conds, "started_at >= ?")
		args = append(args, timeToMs(f.Since))
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

const sessionCols = `s.id, s.agent, s.title, s.cwd, s.project, s.git_branch, s.git_sha,
	s.model, s.cli_version, s.originator, s.workflow_id, s.parent_id, s.started_at, s.ended_at`

// CountSessions is the unpaginated total for the list envelope.
func (s *Store) CountSessions(ctx context.Context, f model.SessionFilter) (int, error) {
	where, args := buildSessionWhere(f)
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions "+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	return n, nil
}

// ListSessions returns one page of sessions with their rolled-up token totals.
func (s *Store) ListSessions(ctx context.Context, f model.SessionFilter) ([]model.SessionSummary, error) {
	where, args := buildSessionWhere(f)

	order := "s.started_at DESC"
	if f.Sort == SortOldest {
		order = "s.started_at ASC"
	}
	limit := ClampLimit(f.Limit)

	q := fmt.Sprintf(`SELECT %s,
		(SELECT COUNT(*) FROM turns t WHERE t.session_id = s.id),
		COALESCE((SELECT SUM(t.input)       FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.output)      FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.cache_read)  FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.cache_write) FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.reasoning)   FROM turns t WHERE t.session_id = s.id), 0)
		FROM sessions s %s ORDER BY %s LIMIT ? OFFSET ?`, sessionCols, where, order)

	args = append(args, limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.SessionSummary{} // never nil: a nil slice marshals to null
	for rows.Next() {
		sum, err := scanSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanSummary(r scanner) (model.SessionSummary, error) {
	var m model.SessionSummary
	var startedMs, endedMs int64
	err := r.Scan(&m.ID, &m.Agent, &m.Title, &m.CWD, &m.Project, &m.GitBranch, &m.GitSHA,
		&m.Model, &m.CLIVersion, &m.Originator, &m.WorkflowID, &m.ParentID, &startedMs, &endedMs,
		&m.TurnCount, &m.Tokens.Input, &m.Tokens.Output, &m.Tokens.CacheRead,
		&m.Tokens.CacheWrite, &m.Tokens.ReasoningOutput)
	if err != nil {
		return m, fmt.Errorf("scan session: %w", err)
	}
	m.StartedAt = msToTime(startedMs)
	m.EndedAt = msToTime(endedMs)
	return m, nil
}

// resolveID expands a session-id prefix. Codex ids are UUIDv7 and all begin with
// the same timestamp bytes, so short prefixes are ambiguous by construction.
func (s *Store) resolveID(ctx context.Context, idOrPrefix, agent string) (string, error) {
	var exact string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM sessions WHERE id = ?", idOrPrefix).Scan(&exact)
	if err == nil {
		return exact, nil
	}
	if !isNoRows(err) {
		return "", fmt.Errorf("resolve session: %w", err)
	}
	if len(idOrPrefix) < MinPrefixLen {
		return "", ErrPrefixTooShort
	}

	// substr rather than LIKE: a `_` in the prefix is a LIKE wildcard and would
	// resolve to a different session than the one typed.
	q := "SELECT id FROM sessions WHERE substr(id, 1, ?) = ?"
	args := []any{len(idOrPrefix), idOrPrefix}
	if agent != "" {
		q += " AND agent = ?"
		args = append(args, agent)
	}
	rows, err := s.db.QueryContext(ctx, q+" LIMIT 2", args...)
	if err != nil {
		return "", fmt.Errorf("resolve session prefix: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var found []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		found = append(found, id)
	}
	switch len(found) {
	case 0:
		return "", ErrSessionNotFound
	case 1:
		return found[0], nil
	default:
		return "", ErrAmbiguousPrefix
	}
}

// GetSession loads one session with a page of its turns.
func (s *Store) GetSession(ctx context.Context, idOrPrefix, agent string, turnLimit, turnOffset int) (*model.SessionDetail, error) {
	id, err := s.resolveID(ctx, idOrPrefix, agent)
	if err != nil {
		return nil, err
	}

	q := fmt.Sprintf(`SELECT %s,
		(SELECT COUNT(*) FROM turns t WHERE t.session_id = s.id),
		COALESCE((SELECT SUM(t.input)       FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.output)      FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.cache_read)  FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.cache_write) FROM turns t WHERE t.session_id = s.id), 0),
		COALESCE((SELECT SUM(t.reasoning)   FROM turns t WHERE t.session_id = s.id), 0)
		FROM sessions s WHERE s.id = ?`, sessionCols)

	sum, err := scanSummary(s.db.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}

	turns, err := s.listTurns(ctx, id, turnLimit, turnOffset)
	if err != nil {
		return nil, err
	}
	return &model.SessionDetail{SessionSummary: sum, Turns: turns}, nil
}

func (s *Store) listTurns(ctx context.Context, sessionID string, limit, offset int) ([]model.Turn, error) {
	if limit <= 0 {
		limit = MaxSessionLimit
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.session_id, t.idx, t.model, t.started_at,
		t.duration_ms, t.input, t.output, t.cache_read, t.cache_write, t.reasoning,
		COALESCE(p.prompt_text, ''), COALESCE(p.response_text, '')
		FROM turns t LEFT JOIN turn_payloads p ON p.turn_id = t.id
		WHERE t.session_id = ? ORDER BY t.idx ASC LIMIT ? OFFSET ?`, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list turns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	turns := []model.Turn{}
	ids := []string{}
	for rows.Next() {
		var t model.Turn
		var startedMs int64
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Index, &t.Model, &startedMs, &t.DurationMs,
			&t.Tokens.Input, &t.Tokens.Output, &t.Tokens.CacheRead, &t.Tokens.CacheWrite,
			&t.Tokens.ReasoningOutput, &t.PromptText, &t.ResponseText); err != nil {
			return nil, fmt.Errorf("scan turn: %w", err)
		}
		t.StartedAt = msToTime(startedMs)
		t.ToolSpans = []model.ToolSpan{}
		t.HookExecs = []model.HookExec{}
		t.Skills = []model.Skill{}
		turns = append(turns, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(turns) == 0 {
		return turns, nil
	}
	if err := s.attachSpans(ctx, turns, ids); err != nil {
		return nil, err
	}
	return turns, nil
}

func (s *Store) attachSpans(ctx context.Context, turns []model.Turn, ids []string) error {
	byTurn := map[string]*model.Turn{}
	for i := range turns {
		byTurn[turns[i].ID] = &turns[i]
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := s.db.QueryContext(ctx, `SELECT sp.id, sp.turn_id, sp.name, sp.kind, sp.duration_ms, sp.ok,
		sp.subagent_session_id, sp.subagent_name,
		sp.rollup_input, sp.rollup_output, sp.rollup_cache_read, sp.rollup_cache_write,
		COALESCE(pp.input, ''), COALESCE(pp.output, ''), COALESCE(pp.error, '')
		FROM tool_spans sp LEFT JOIN tool_span_payloads pp ON pp.span_id = sp.id
		WHERE sp.turn_id IN (`+ph+`)`, args...)
	if err != nil {
		return fmt.Errorf("list tool spans: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var sp model.ToolSpan
		var ok int
		var roll model.TokenUsage
		if err := rows.Scan(&sp.ID, &sp.TurnID, &sp.Name, &sp.Kind, &sp.DurationMs, &ok,
			&sp.SubagentSessionID, &sp.SubagentName,
			&roll.Input, &roll.Output, &roll.CacheRead, &roll.CacheWrite,
			&sp.Input, &sp.Output, &sp.Error); err != nil {
			return fmt.Errorf("scan tool span: %w", err)
		}
		sp.OK = ok == 1
		// Only a span that actually rolled up a subagent carries tokens; leaving the
		// pointer nil is what keeps rolluptokens out of every other span's JSON.
		if roll != (model.TokenUsage{}) {
			sp.RollupTokens = &roll
		}
		if t := byTurn[sp.TurnID]; t != nil {
			t.ToolSpans = append(t.ToolSpans, sp)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	hooks, err := s.db.QueryContext(ctx, `SELECT turn_id, hook_name, event, duration_ms, exit_code
		FROM hook_execs WHERE turn_id IN (`+ph+`)`, args...)
	if err != nil {
		return fmt.Errorf("list hook execs: %w", err)
	}
	defer func() { _ = hooks.Close() }()
	for hooks.Next() {
		var h model.HookExec
		if err := hooks.Scan(&h.TurnID, &h.HookName, &h.Event, &h.DurationMs, &h.ExitCode); err != nil {
			return fmt.Errorf("scan hook exec: %w", err)
		}
		if t := byTurn[h.TurnID]; t != nil {
			t.HookExecs = append(t.HookExecs, h)
		}
	}
	if err := hooks.Err(); err != nil {
		return err
	}

	skills, err := s.db.QueryContext(ctx, `SELECT turn_id, name, args
		FROM skill_invocations WHERE turn_id IN (`+ph+`)`, args...)
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}
	defer func() { _ = skills.Close() }()
	for skills.Next() {
		var sk model.Skill
		if err := skills.Scan(&sk.TurnID, &sk.Name, &sk.Args); err != nil {
			return fmt.Errorf("scan skill: %w", err)
		}
		if t := byTurn[sk.TurnID]; t != nil {
			t.Skills = append(t.Skills, sk)
		}
	}
	return skills.Err()
}

// UsageRaw is one grouped bucket before pricing is applied. The store does not
// know about money; the service turns these into model.UsageRow.
type UsageRaw struct {
	Date   string
	Agent  string
	Model  string
	Tokens model.TokenUsage
}

// Usage aggregates tokens by date/agent/model. Subagent turns are included —
// they are billed — but their parents' rollup columns are not summed here.
func (s *Store) Usage(ctx context.Context, f model.UsageFilter) ([]UsageRaw, error) {
	format := "%Y-%m-%d"
	if f.Group == UsageGroupMonthly {
		format = "%Y-%m"
	}

	conds := []string{"1=1"}
	var args []any
	args = append(args, format)
	if f.Agent != "" {
		conds = append(conds, "s.agent = ?")
		args = append(args, f.Agent)
	}
	if !f.Since.IsZero() {
		conds = append(conds, "t.started_at >= ?")
		args = append(args, timeToMs(f.Since))
	}

	q := `SELECT strftime(?, t.started_at/1000, 'unixepoch', 'localtime') AS bucket, s.agent, t.model,
		SUM(t.input), SUM(t.output), SUM(t.cache_read), SUM(t.cache_write), SUM(t.reasoning)
		FROM turns t JOIN sessions s ON s.id = t.session_id
		WHERE ` + strings.Join(conds, " AND ") + `
		GROUP BY bucket, s.agent, t.model ORDER BY bucket DESC, s.agent, t.model`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []UsageRaw{}
	for rows.Next() {
		var u UsageRaw
		if err := rows.Scan(&u.Date, &u.Agent, &u.Model, &u.Tokens.Input, &u.Tokens.Output,
			&u.Tokens.CacheRead, &u.Tokens.CacheWrite, &u.Tokens.ReasoningOutput); err != nil {
			return nil, fmt.Errorf("scan usage: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
