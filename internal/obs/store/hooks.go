package store

import (
	"context"
	"fmt"
	"sort"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// hookConds mirrors skillConds: the same session-level scope, applied to every
// leg so the rollup and the error-share denominator agree on what is in range.
func hookConds(f model.HookFilter) ([]string, []any) {
	return skillConds(model.SkillFilter{Agent: f.Agent, Project: f.Project, Since: f.Since})
}

type hookKey struct{ name, event string }

type hookAgg struct {
	fires, nonzero, errsInBlockedTurns int
}

// Hooks rolls up hook_execs by hook and event: how often each fired, how often it
// exited nonzero, and the share of all in-scope tool errors that land in a turn
// where the hook blocked (the number that surfaces a gate hook taxing tool calls).
// hook_execs are Claude-only, so a codex filter yields nothing.
func (s *Store) Hooks(ctx context.Context, f model.HookFilter) ([]model.HookSummary, error) {
	conds, args := hookConds(f)
	where := whereClause(conds)

	// Denominator: every failed tool span in scope.
	var totalErrs int
	errQ := `SELECT COUNT(*) FROM tool_spans ts JOIN sessions s ON s.id = ts.session_id
		WHERE ts.ok = 0` + andClause(conds)
	if err := s.db.QueryRowContext(ctx, errQ, args...).Scan(&totalErrs); err != nil {
		return nil, fmt.Errorf("hook error total: %w", err)
	}

	agg := map[hookKey]*hookAgg{}
	get := func(k hookKey) *hookAgg {
		a := agg[k]
		if a == nil {
			a = &hookAgg{}
			agg[k] = a
		}
		return a
	}

	// Fires and nonzero exits per hook/event.
	if err := s.scanRows(ctx,
		`SELECT he.hook_name, he.event, COUNT(*),
			COALESCE(SUM(CASE WHEN he.exit_code != 0 THEN 1 ELSE 0 END), 0)
		FROM hook_execs he JOIN sessions s ON s.id = he.session_id
		`+where+` GROUP BY he.hook_name, he.event`, args,
		func(sc scanner) error {
			var k hookKey
			var fires, nonzero int
			if err := sc.Scan(&k.name, &k.event, &fires, &nonzero); err != nil {
				return err
			}
			a := get(k)
			a.fires, a.nonzero = fires, nonzero
			return nil
		}); err != nil {
		return nil, err
	}

	// Errors that co-occur (same turn) with a nonzero exit of this hook.
	if err := s.scanRows(ctx,
		`SELECT he.hook_name, he.event, COUNT(DISTINCT ts.id)
		FROM hook_execs he
		JOIN tool_spans ts ON ts.turn_id = he.turn_id AND ts.ok = 0
		JOIN sessions s ON s.id = he.session_id
		WHERE he.exit_code != 0`+andClause(conds)+`
		GROUP BY he.hook_name, he.event`, args,
		func(sc scanner) error {
			var k hookKey
			var n int
			if err := sc.Scan(&k.name, &k.event, &n); err != nil {
				return err
			}
			get(k).errsInBlockedTurns = n
			return nil
		}); err != nil {
		return nil, err
	}

	out := make([]model.HookSummary, 0, len(agg))
	for k, a := range agg {
		sum := model.HookSummary{
			HookName: k.name, Event: k.event, Fires: a.fires, NonzeroExits: a.nonzero,
		}
		if a.fires > 0 {
			sum.BlockRate = float64(a.nonzero) / float64(a.fires)
		}
		if totalErrs > 0 {
			share := float64(a.errsInBlockedTurns) / float64(totalErrs)
			sum.ErrShare = &share
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NonzeroExits != out[j].NonzeroExits {
			return out[i].NonzeroExits > out[j].NonzeroExits
		}
		if out[i].Fires != out[j].Fires {
			return out[i].Fires > out[j].Fires
		}
		if out[i].HookName != out[j].HookName {
			return out[i].HookName < out[j].HookName
		}
		return out[i].Event < out[j].Event
	})
	return out, nil
}
