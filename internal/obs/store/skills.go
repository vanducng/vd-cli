package store

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// correctionRe flags a user turn that opens by pushing back on the last result.
// Ported from the skill-audit miner; RE2 has no backrefs but needs none here.
// It runs in Go because modernc SQLite ships no REGEXP; the input is a trimmed
// prompt prefix, so the start anchor still means "start of the message".
var correctionRe = regexp.MustCompile(
	`(?i)^(no[,. ]|nope\b|wrong\b|not what|that'?s not|revert\b|undo\b|you broke|` +
		`still (fail|broken|wrong)|didn'?t work|doesn'?t work|try again)`)

// interruptMarker is the text a user interruption leaves in a turn's payload. It
// is the only per-turn abort trace obs persists today (Codex turn_aborted folds
// into a duration only), so aborts count it rather than inventing new storage.
const interruptMarker = "%[Request interrupted by user%"

// skillConds turns a SkillFilter into session-level predicates over alias `s`.
// Every leg of the rollup applies the same set, so a filter can never restrict
// one leg (say the windows) without restricting the "(none)" bucket the same way.
func skillConds(f model.SkillFilter) ([]string, []any) {
	var conds []string
	var args []any
	if f.Agent != "" {
		conds = append(conds, "s.agent = ?")
		args = append(args, f.Agent)
	}
	if f.Project != "" {
		conds = append(conds, `(s.project = ? OR s.cwd LIKE ? ESCAPE '\')`)
		args = append(args, f.Project, likeEscape(f.Project)+"%")
	}
	if !f.Since.IsZero() {
		conds = append(conds, "s.started_at >= ?")
		args = append(args, timeToMs(f.Since))
	}
	return conds, args
}

func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(conds, " AND ")
}

func andClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " AND " + strings.Join(conds, " AND ")
}

// skillWindowCTE builds the per-invocation attribution windows once, for reuse by
// the span and token legs. Each invocation opens a window at its turn's idx that
// runs to the next invocation in the session (LEAD), or to a sentinel past every
// real idx for the last one; `firsts` marks where the first window opens so the
// "(none)" legs can claim everything before it. The %s is the session filter.
const skillWindowCTE = `WITH inv AS (
	SELECT si.session_id, si.seq, si.name, t.idx AS start_idx
	FROM skill_invocations si
	JOIN turns t ON t.id = si.turn_id
	JOIN sessions s ON s.id = si.session_id
	%s
),
windows AS (
	SELECT session_id, name, start_idx,
		LEAD(start_idx, 1, 1000000000000000000) OVER (
			PARTITION BY session_id ORDER BY start_idx, seq) AS end_idx
	FROM inv
),
firsts AS (SELECT session_id, MIN(start_idx) AS first_idx FROM inv GROUP BY session_id)`

type skillAgg struct {
	invocations, sessions, solo int
	toolCalls, toolErrors       int
	tokens                      int
	corrections, aborts         int
	agents                      map[string]struct{}
}

func (a *skillAgg) addAgents(csv string) {
	if csv == "" {
		return
	}
	if a.agents == nil {
		a.agents = map[string]struct{}{}
	}
	for _, ag := range strings.Split(csv, ",") {
		if ag != "" {
			a.agents[ag] = struct{}{}
		}
	}
}

// Skills computes the per-skill rollup with per-invocation window attribution.
// The "(none)" bucket plus every skill's windows partition all in-scope tool
// spans exactly once, so summing the tool-call column reconstructs the total.
func (s *Store) Skills(ctx context.Context, f model.SkillFilter) ([]model.SkillSummary, error) {
	conds, args := skillConds(f)
	where := whereClause(conds)
	agg := map[string]*skillAgg{}
	get := func(name string) *skillAgg {
		a := agg[name]
		if a == nil {
			a = &skillAgg{}
			agg[name] = a
		}
		return a
	}

	// Invocations, sessions and invoking agents, straight off the invocation rows.
	if err := s.scanRows(ctx,
		`SELECT si.name, COUNT(*), COUNT(DISTINCT si.session_id),
			COALESCE(GROUP_CONCAT(DISTINCT s.agent), '')
		FROM skill_invocations si JOIN sessions s ON s.id = si.session_id
		`+where+` GROUP BY si.name`, args,
		func(sc scanner) error {
			var name, agents string
			var inv, sess int
			if err := sc.Scan(&name, &inv, &sess, &agents); err != nil {
				return err
			}
			a := get(name)
			a.invocations, a.sessions = inv, sess
			a.addAgents(agents)
			return nil
		}); err != nil {
		return nil, err
	}

	// Solo sessions: sessions that invoked exactly one distinct skill, per skill.
	if err := s.scanRows(ctx,
		`WITH ss AS (SELECT DISTINCT si.session_id, si.name
			FROM skill_invocations si JOIN sessions s ON s.id = si.session_id `+where+`),
		solo AS (SELECT session_id, MIN(name) AS name FROM ss GROUP BY session_id HAVING COUNT(*) = 1)
		SELECT name, COUNT(*) FROM solo GROUP BY name`, args,
		func(sc scanner) error {
			var name string
			var n int
			if err := sc.Scan(&name, &n); err != nil {
				return err
			}
			get(name).solo = n
			return nil
		}); err != nil {
		return nil, err
	}

	// Tool calls and errors, attributed by window; the "(none)" leg claims spans
	// before the first invocation and spans in no-skill sessions.
	twice := append(append([]any{}, args...), args...)
	spanQ := fmt.Sprintf(skillWindowCTE+`
		SELECT w.name, COUNT(ts.id), COALESCE(SUM(CASE WHEN ts.ok = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(GROUP_CONCAT(DISTINCT s.agent), '')
		FROM windows w
		JOIN turns tt ON tt.session_id = w.session_id AND tt.idx >= w.start_idx AND tt.idx < w.end_idx
		JOIN tool_spans ts ON ts.turn_id = tt.id
		JOIN sessions s ON s.id = w.session_id
		GROUP BY w.name
		UNION ALL
		SELECT '`+model.SkillNone+`', COUNT(ts.id),
			COALESCE(SUM(CASE WHEN ts.ok = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(GROUP_CONCAT(DISTINCT s.agent), '')
		FROM tool_spans ts
		JOIN turns tt ON tt.id = ts.turn_id
		JOIN sessions s ON s.id = tt.session_id
		LEFT JOIN firsts f ON f.session_id = tt.session_id
		WHERE (f.session_id IS NULL OR tt.idx < f.first_idx)%s`,
		where, andClause(conds))
	if err := s.scanRows(ctx, spanQ, twice, func(sc scanner) error {
		var name, agents string
		var calls, errs int
		if err := sc.Scan(&name, &calls, &errs, &agents); err != nil {
			return err
		}
		a := get(name)
		a.toolCalls += calls
		a.toolErrors += errs
		a.addAgents(agents)
		return nil
	}); err != nil {
		return nil, err
	}

	// Tokens, attributed by the same windows but summed over turns, not spans.
	tokenQ := fmt.Sprintf(skillWindowCTE+`
		SELECT w.name, COALESCE(SUM(tt.input + tt.output + tt.cache_read + tt.cache_write), 0)
		FROM windows w
		JOIN turns tt ON tt.session_id = w.session_id AND tt.idx >= w.start_idx AND tt.idx < w.end_idx
		GROUP BY w.name
		UNION ALL
		SELECT '`+model.SkillNone+`', COALESCE(SUM(tt.input + tt.output + tt.cache_read + tt.cache_write), 0)
		FROM turns tt
		JOIN sessions s ON s.id = tt.session_id
		LEFT JOIN firsts f ON f.session_id = tt.session_id
		WHERE (f.session_id IS NULL OR tt.idx < f.first_idx)%s`,
		where, andClause(conds))
	if err := s.scanRows(ctx, tokenQ, twice, func(sc scanner) error {
		var name string
		var tokens int
		if err := sc.Scan(&name, &tokens); err != nil {
			return err
		}
		get(name).tokens += tokens
		return nil
	}); err != nil {
		return nil, err
	}

	// Corrections and aborts: classified per windowed user turn. Only a 200-char
	// prompt prefix leaves the store (the correction match is start-anchored), and
	// the interrupt marker is a per-turn flag — no raw text reaches any aggregate.
	// The interrupt marker carries '%' (a LIKE wildcard and an fmt verb), so the CTE
	// is formatted first and the marker concatenated after, never routed through fmt.
	proseQ := fmt.Sprintf(skillWindowCTE, where) + `
		SELECT w.name, substr(p.prompt_text, 1, 200),
			CASE WHEN p.prompt_text LIKE '` + interruptMarker + `'
				OR p.response_text LIKE '` + interruptMarker + `' THEN 1 ELSE 0 END
		FROM windows w
		JOIN turns tt ON tt.session_id = w.session_id AND tt.idx >= w.start_idx AND tt.idx < w.end_idx
		JOIN turn_payloads p ON p.turn_id = tt.id
		UNION ALL
		SELECT '` + model.SkillNone + `', substr(p.prompt_text, 1, 200),
			CASE WHEN p.prompt_text LIKE '` + interruptMarker + `'
				OR p.response_text LIKE '` + interruptMarker + `' THEN 1 ELSE 0 END
		FROM turns tt
		JOIN sessions s ON s.id = tt.session_id
		JOIN turn_payloads p ON p.turn_id = tt.id
		LEFT JOIN firsts f ON f.session_id = tt.session_id
		WHERE (f.session_id IS NULL OR tt.idx < f.first_idx)` + andClause(conds)
	if err := s.scanRows(ctx, proseQ, twice, func(sc scanner) error {
		var name, prefix string
		var interrupted int
		if err := sc.Scan(&name, &prefix, &interrupted); err != nil {
			return err
		}
		a := get(name)
		if correctionRe.MatchString(strings.TrimSpace(prefix)) {
			a.corrections++
		}
		if interrupted != 0 {
			a.aborts++
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return finishSkills(agg), nil
}

// finishSkills turns the aggregate map into sorted summaries: errors desc, then
// name, with "(none)" always last. ErrRate stays nil when a skill drove no tool
// call, so an idle skill reads as "no data", never a misleading 0%.
func finishSkills(agg map[string]*skillAgg) []model.SkillSummary {
	out := make([]model.SkillSummary, 0, len(agg))
	for name, a := range agg {
		// Drop a "(none)" bucket that never collected anything, but always keep real
		// skills so an invoked-but-idle skill still shows up.
		if name == model.SkillNone && a.toolCalls == 0 && a.tokens == 0 && a.corrections == 0 && a.aborts == 0 {
			continue
		}
		sum := model.SkillSummary{
			Name: name, Agents: sortedKeys(a.agents),
			Invocations: a.invocations, Sessions: a.sessions, SoloSessions: a.solo,
			ToolCalls: a.toolCalls, ToolErrors: a.toolErrors, Tokens: a.tokens,
			Corrections: a.corrections, Aborts: a.aborts,
		}
		if a.toolCalls > 0 {
			r := float64(a.toolErrors) / float64(a.toolCalls)
			sum.ErrRate = &r
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, nj := out[i].Name == model.SkillNone, out[j].Name == model.SkillNone
		if ni != nj {
			return nj // "(none)" sinks to the bottom
		}
		if out[i].ToolErrors != out[j].ToolErrors {
			return out[i].ToolErrors > out[j].ToolErrors
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// scanRows runs a query and hands each row to fn, centralizing the open/close/err
// dance the skills legs would otherwise repeat four times.
func (s *Store) scanRows(ctx context.Context, q string, args []any, fn func(scanner) error) error {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("skills query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return fmt.Errorf("scan skills row: %w", err)
		}
	}
	return rows.Err()
}
