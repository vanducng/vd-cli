package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ErrorEvent is one failed tool_spans row joined to its session and turn
// context, plus any skills invoked in the same turn. Raw: the service
// normalizes ErrorText into a signature and clusters in Go.
type ErrorEvent struct {
	SessionID string
	Agent     string
	Project   string
	TurnID    string
	TurnIndex int
	ToolName  string
	ErrorText string
	StartedAt time.Time
	Skills    []string
}

// ErrorEvents returns every tool_spans row with ok=0 whose turn started in
// [since, until), scoped by agent/project. until is exclusive so the previous
// window and a current window starting where it ends never double-count the
// same turn.
func (s *Store) ErrorEvents(ctx context.Context, since, until time.Time, agent, project string) ([]ErrorEvent, error) {
	conds, args := sessionFilterConds(agent, project, "s.")
	conds = append(conds, "ts.ok = 0", "t.started_at > 0")
	if !since.IsZero() {
		conds = append(conds, "t.started_at >= ?")
		args = append(args, timeToMs(since))
	}
	if !until.IsZero() {
		conds = append(conds, "t.started_at < ?")
		args = append(args, timeToMs(until))
	}

	q := fmt.Sprintf(`SELECT ts.session_id, s.agent, s.project, ts.turn_id, t.idx, ts.name,
		COALESCE(tsp.error, ''), t.started_at
		FROM tool_spans ts
		JOIN turns t ON t.id = ts.turn_id
		JOIN sessions s ON s.id = ts.session_id
		LEFT JOIN tool_span_payloads tsp ON tsp.span_id = ts.id
		WHERE %s ORDER BY t.started_at DESC`, strings.Join(conds, " AND "))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("error events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []ErrorEvent{}
	turnIDs := map[string]bool{}
	for rows.Next() {
		var e ErrorEvent
		var startedMs int64
		if err := rows.Scan(&e.SessionID, &e.Agent, &e.Project, &e.TurnID, &e.TurnIndex,
			&e.ToolName, &e.ErrorText, &startedMs); err != nil {
			return nil, fmt.Errorf("scan error event: %w", err)
		}
		e.StartedAt = msToTime(startedMs)
		out = append(out, e)
		turnIDs[e.TurnID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	skills, err := s.skillsForTurns(ctx, turnIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Skills = skills[out[i].TurnID]
	}
	return out, nil
}

// skillsForTurns batches the skill_invocations lookup for a set of turns,
// mirroring attachSpans' fan-in so a cluster's evidence never costs one query
// per event.
func (s *Store) skillsForTurns(ctx context.Context, turnIDs map[string]bool) (map[string][]string, error) {
	ids := make([]any, 0, len(turnIDs))
	for id := range turnIDs {
		ids = append(ids, id)
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	rows, err := s.db.QueryContext(ctx, `SELECT turn_id, name FROM skill_invocations
		WHERE turn_id IN (`+ph+`) ORDER BY seq`, ids...)
	if err != nil {
		return nil, fmt.Errorf("skills for turns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string][]string{}
	for rows.Next() {
		var turnID, name string
		if err := rows.Scan(&turnID, &name); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}
		out[turnID] = append(out[turnID], name)
	}
	return out, rows.Err()
}

// ToolSpanTotal counts every tool_spans row (ok or not) in [since, until),
// scoped by agent/project — the denominator for HealthReport.ErrorRate.
func (s *Store) ToolSpanTotal(ctx context.Context, since, until time.Time, agent, project string) (int, error) {
	conds, args := sessionFilterConds(agent, project, "s.")
	conds = append(conds, "t.started_at > 0")
	if !since.IsZero() {
		conds = append(conds, "t.started_at >= ?")
		args = append(args, timeToMs(since))
	}
	if !until.IsZero() {
		conds = append(conds, "t.started_at < ?")
		args = append(args, timeToMs(until))
	}

	q := fmt.Sprintf(`SELECT COUNT(*) FROM tool_spans ts
		JOIN turns t ON t.id = ts.turn_id
		JOIN sessions s ON s.id = ts.session_id
		WHERE %s`, strings.Join(conds, " AND "))

	var n int
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("tool span total: %w", err)
	}
	return n, nil
}
