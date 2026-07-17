package store

import (
	"context"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// skillScene seeds three sessions that exercise every attribution edge: spans
// before the first invocation, two interleaved skills in one session, a session
// whose only skill makes it "solo", and a session that invokes nothing.
//
//	A (claude): t0 none | t1 $plan | t2 | t3 $cook | t4      -> plan[1,3), cook[3,∞)
//	B (codex):  t0 $plan | t1                                -> plan[0,∞), solo
//	C (claude): t0 (no invocation)                           -> all (none)
//
// Every turn carries 10 input tokens, so the token legs are countable by hand.
func skillScene(t *testing.T, s *Store) {
	t.Helper()
	tok := model.TokenUsage{Input: 10}
	now := time.Unix(1_780_000_000, 0).UTC()

	a := model.Record{
		Session: model.Session{ID: "A", Agent: model.AgentClaude, Project: "vd-cli", CWD: "/repo/vd-cli", StartedAt: now},
		Turns: []model.Turn{
			{ID: "A-t0", SessionID: "A", Index: 0, StartedAt: now, Tokens: tok},
			{ID: "A-t1", SessionID: "A", Index: 1, StartedAt: now, Tokens: tok},
			{ID: "A-t2", SessionID: "A", Index: 2, StartedAt: now, Tokens: tok},
			{ID: "A-t3", SessionID: "A", Index: 3, StartedAt: now, Tokens: tok},
			{ID: "A-t4", SessionID: "A", Index: 4, StartedAt: now, Tokens: tok},
		},
		ToolSpans: []model.ToolSpan{
			{ID: "a0", TurnID: "A-t0", Name: "Bash", OK: true},
			{ID: "a1", TurnID: "A-t1", Name: "Bash", OK: true},
			{ID: "a1e", TurnID: "A-t1", Name: "Bash", OK: false},
			{ID: "a2", TurnID: "A-t2", Name: "Bash", OK: true},
			{ID: "a3", TurnID: "A-t3", Name: "Bash", OK: true},
			{ID: "a3e", TurnID: "A-t3", Name: "Bash", OK: false},
			{ID: "a4e", TurnID: "A-t4", Name: "Bash", OK: false},
		},
		Skills: []model.Skill{
			{TurnID: "A-t1", Name: "plan"},
			{TurnID: "A-t3", Name: "cook"},
		},
	}
	b := model.Record{
		Session: model.Session{ID: "B", Agent: model.AgentCodex, Project: "vd-cli", CWD: "/repo/vd-cli", StartedAt: now},
		Turns: []model.Turn{
			{ID: "B-t0", SessionID: "B", Index: 0, StartedAt: now, Tokens: tok},
			{ID: "B-t1", SessionID: "B", Index: 1, StartedAt: now, Tokens: tok},
		},
		ToolSpans: []model.ToolSpan{
			{ID: "b0", TurnID: "B-t0", Name: "exec", OK: true},
			{ID: "b1e", TurnID: "B-t1", Name: "exec", OK: false},
		},
		Skills: []model.Skill{{TurnID: "B-t0", Name: "plan"}},
	}
	c := model.Record{
		Session: model.Session{ID: "C", Agent: model.AgentClaude, Project: "vd-cli", CWD: "/repo/vd-cli", StartedAt: now},
		Turns:   []model.Turn{{ID: "C-t0", SessionID: "C", Index: 0, StartedAt: now, Tokens: tok}},
		ToolSpans: []model.ToolSpan{
			{ID: "c0", TurnID: "C-t0", Name: "Bash", OK: true},
			{ID: "c1", TurnID: "C-t0", Name: "Bash", OK: true},
		},
	}
	for _, rec := range []model.Record{a, b, c} {
		if err := s.IngestFile(context.Background(), rec, Watermark{}); err != nil {
			t.Fatalf("seed %s: %v", rec.Session.ID, err)
		}
	}
}

func byName(rows []model.SkillSummary) map[string]model.SkillSummary {
	m := map[string]model.SkillSummary{}
	for _, r := range rows {
		m[r.Name] = r
	}
	return m
}

func TestSkillsWindowAttributionAndMassConservation(t *testing.T) {
	s := openTestDB(t)
	skillScene(t, s)

	rows, err := s.Skills(context.Background(), model.SkillFilter{})
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	got := byName(rows)

	tests := []struct {
		name                                 string
		inv, sess, solo, calls, errs, tokens int
		agents                               []string
	}{
		{"plan", 2, 2, 1, 5, 2, 40, []string{model.AgentClaude, model.AgentCodex}},
		{"cook", 1, 1, 0, 3, 2, 20, []string{model.AgentClaude}},
		{model.SkillNone, 0, 0, 0, 3, 0, 20, []string{model.AgentClaude}},
	}
	for _, tt := range tests {
		r, ok := got[tt.name]
		if !ok {
			t.Errorf("missing skill %q", tt.name)
			continue
		}
		if r.Invocations != tt.inv || r.Sessions != tt.sess || r.SoloSessions != tt.solo {
			t.Errorf("%s: inv/sess/solo = %d/%d/%d, want %d/%d/%d",
				tt.name, r.Invocations, r.Sessions, r.SoloSessions, tt.inv, tt.sess, tt.solo)
		}
		if r.ToolCalls != tt.calls || r.ToolErrors != tt.errs || r.Tokens != tt.tokens {
			t.Errorf("%s: calls/errs/tokens = %d/%d/%d, want %d/%d/%d",
				tt.name, r.ToolCalls, r.ToolErrors, r.Tokens, tt.calls, tt.errs, tt.tokens)
		}
		if !eqStrs(r.Agents, tt.agents) {
			t.Errorf("%s: agents = %v, want %v", tt.name, r.Agents, tt.agents)
		}
	}

	// The invariant: windows plus "(none)" partition every span exactly once.
	total := 0
	for _, r := range rows {
		total += r.ToolCalls
	}
	if total != 11 {
		t.Errorf("mass conservation broken: sum of tool_calls = %d, want 11 total spans", total)
	}

	// A span-driving bucket reports a rate; an idle one would report nil (none here).
	if p := got["cook"]; p.ErrRate == nil || *p.ErrRate < 0.66 || *p.ErrRate > 0.67 {
		t.Errorf("cook err_rate = %v, want ~0.667", p.ErrRate)
	}

	// "(none)" must sink to the last row regardless of its error count.
	if rows[len(rows)-1].Name != model.SkillNone {
		t.Errorf("last row = %q, want the (none) bucket", rows[len(rows)-1].Name)
	}
}

// A session-level filter must restrict the windows and the "(none)" bucket the
// same way: agent=codex keeps only session B, so plan sheds session A's window,
// cook disappears with A, and no claude span leaks into (none).
func TestSkillsAgentFilterRestrictsBothLegs(t *testing.T) {
	s := openTestDB(t)
	skillScene(t, s)

	rows, err := s.Skills(context.Background(), model.SkillFilter{Agent: model.AgentCodex})
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	got := byName(rows)

	if _, ok := got["cook"]; ok {
		t.Errorf("cook leaked past the codex filter: %+v", got["cook"])
	}
	p, ok := got["plan"]
	if !ok {
		t.Fatalf("plan missing under codex filter")
	}
	if p.Invocations != 1 || p.Sessions != 1 || p.SoloSessions != 1 {
		t.Errorf("plan inv/sess/solo = %d/%d/%d, want 1/1/1", p.Invocations, p.Sessions, p.SoloSessions)
	}
	if p.ToolCalls != 2 || p.ToolErrors != 1 || p.Tokens != 20 {
		t.Errorf("plan calls/errs/tokens = %d/%d/%d, want 2/1/20", p.ToolCalls, p.ToolErrors, p.Tokens)
	}
	total := 0
	for _, r := range rows {
		total += r.ToolCalls
	}
	if total != 2 {
		t.Errorf("codex-scoped spans = %d, want 2 (session B only)", total)
	}
}

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
