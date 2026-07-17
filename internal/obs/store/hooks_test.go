package store

import (
	"context"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// hookScene seeds one claude session whose guard hook blocks on the turn that also
// carries a tool error, so block rate and error-share are both non-trivial:
//
//	t0: guard PreToolUse exit 0 | span err          -> a non-blocked error
//	t1: guard PreToolUse exit 2 | span err, span ok -> the block + its co-error
//	t2: logger Stop exit 0                           -> a clean, non-gate hook
func hookScene(t *testing.T, s *Store) {
	t.Helper()
	now := time.Unix(1_780_000_000, 0).UTC()
	rec := model.Record{
		Session: model.Session{ID: "S1", Agent: model.AgentClaude, Project: "vd-cli", CWD: "/repo/vd-cli", StartedAt: now},
		Turns: []model.Turn{
			{ID: "S1-t0", SessionID: "S1", Index: 0, StartedAt: now},
			{ID: "S1-t1", SessionID: "S1", Index: 1, StartedAt: now},
			{ID: "S1-t2", SessionID: "S1", Index: 2, StartedAt: now},
		},
		ToolSpans: []model.ToolSpan{
			{ID: "e0", TurnID: "S1-t0", Name: "Bash", OK: false},
			{ID: "e1", TurnID: "S1-t1", Name: "Bash", OK: false},
			{ID: "o1", TurnID: "S1-t1", Name: "Bash", OK: true},
		},
		HookExecs: []model.HookExec{
			{TurnID: "S1-t0", HookName: "guard", Event: "PreToolUse", Seq: 0, ExitCode: 0},
			{TurnID: "S1-t1", HookName: "guard", Event: "PreToolUse", Seq: 0, ExitCode: 2},
			{TurnID: "S1-t2", HookName: "logger", Event: "Stop", Seq: 0, ExitCode: 0},
		},
	}
	if err := s.IngestFile(context.Background(), rec, Watermark{}); err != nil {
		t.Fatalf("seed hooks: %v", err)
	}
}

func TestHooksBlockRateAndErrorShare(t *testing.T) {
	s := openTestDB(t)
	hookScene(t, s)

	rows, err := s.Hooks(context.Background(), model.HookFilter{})
	if err != nil {
		t.Fatalf("Hooks: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (guard, logger)", len(rows))
	}

	// Sorted by nonzero exits desc: guard first.
	guard := rows[0]
	if guard.HookName != "guard" || guard.Event != "PreToolUse" {
		t.Fatalf("first row = %s/%s, want guard/PreToolUse", guard.HookName, guard.Event)
	}
	if guard.Fires != 2 || guard.NonzeroExits != 1 {
		t.Errorf("guard fires/nonzero = %d/%d, want 2/1", guard.Fires, guard.NonzeroExits)
	}
	if guard.BlockRate < 0.49 || guard.BlockRate > 0.51 {
		t.Errorf("guard block_rate = %v, want 0.5", guard.BlockRate)
	}
	// One of the two total errors (e1) sits in the blocked turn t1.
	if guard.ErrShare == nil || *guard.ErrShare < 0.49 || *guard.ErrShare > 0.51 {
		t.Errorf("guard err_share = %v, want 0.5", guard.ErrShare)
	}

	logger := rows[1]
	if logger.NonzeroExits != 0 || logger.BlockRate != 0 {
		t.Errorf("logger nonzero/block_rate = %d/%v, want 0/0", logger.NonzeroExits, logger.BlockRate)
	}
	if logger.ErrShare == nil || *logger.ErrShare != 0 {
		t.Errorf("logger err_share = %v, want 0 (it blocked nothing)", logger.ErrShare)
	}
}

// hook_execs are Claude-only; a codex scope must return nothing, and a since past
// the session must exclude it — both legs honor the same filter.
func TestHooksFilterExcludes(t *testing.T) {
	s := openTestDB(t)
	hookScene(t, s)
	ctx := context.Background()

	codex, err := s.Hooks(ctx, model.HookFilter{Agent: model.AgentCodex})
	if err != nil {
		t.Fatalf("Hooks(codex): %v", err)
	}
	if len(codex) != 0 {
		t.Errorf("codex hooks = %d, want 0", len(codex))
	}

	future, err := s.Hooks(ctx, model.HookFilter{Since: time.Unix(1_790_000_000, 0)})
	if err != nil {
		t.Fatalf("Hooks(since): %v", err)
	}
	if len(future) != 0 {
		t.Errorf("hooks after the session = %d, want 0", len(future))
	}
}
