package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ErrorEvent is one failed tool_spans row joined to its session and turn
// context, plus the skill (if any) that owns the turn's invocation window.
// Raw: the service normalizes ErrorText into a signature and clusters in Go.
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
// same turn. Skill attribution reuses skillWindowCTE from skills.go: a turn's
// co-occurring skill is whichever invocation's window owns it (invocation to
// the next invocation in the session), the same attribution `vd obs skills`
// uses, rather than a second same-turn-only definition of "co-occurs".
func (s *Store) ErrorEvents(ctx context.Context, since, until time.Time, agent, project string) ([]ErrorEvent, error) {
	cteConds, cteArgs := sessionFilterConds(agent, project, "s.")
	cteWhere := whereClause(cteConds)

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

	q := fmt.Sprintf(skillWindowCTE+`
		SELECT ts.session_id, s.agent, s.project, ts.turn_id, t.idx, ts.name,
			COALESCE(tsp.error, ''), t.started_at, COALESCE(w.name, '')
		FROM tool_spans ts
		JOIN turns t ON t.id = ts.turn_id
		JOIN sessions s ON s.id = ts.session_id
		LEFT JOIN tool_span_payloads tsp ON tsp.span_id = ts.id
		LEFT JOIN windows w ON w.session_id = ts.session_id AND t.idx >= w.start_idx AND t.idx < w.end_idx
		WHERE %s
		ORDER BY t.started_at DESC, ts.id`, cteWhere, strings.Join(conds, " AND "))

	out := []ErrorEvent{}
	err := s.scanRows(ctx, q, append(cteArgs, args...), func(sc scanner) error {
		var e ErrorEvent
		var startedMs int64
		var skillName string
		if err := sc.Scan(&e.SessionID, &e.Agent, &e.Project, &e.TurnID, &e.TurnIndex,
			&e.ToolName, &e.ErrorText, &startedMs, &skillName); err != nil {
			return err
		}
		e.StartedAt = msToTime(startedMs)
		if skillName != "" {
			e.Skills = []string{skillName}
		}
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error events: %w", err)
	}
	return out, nil
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
