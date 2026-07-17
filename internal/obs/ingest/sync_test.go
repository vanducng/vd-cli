package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
	"github.com/vanducng/vd-cli/v2/internal/obs/store"
)

func syncStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(store.Config{Path: filepath.Join(t.TempDir(), "obs.sqlite")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// syncTree points HOME at a temp dir holding a claude transcript, so Sync exercises
// the real enumerate -> parse -> ingest path without touching the user's ~/.claude.
func syncTree(t *testing.T, lines []byte) (home, transcript string) {
	t.Helper()
	home = t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "-Users-dev-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript = filepath.Join(dir, "11111111-1111-4111-8111-111111111111.jsonl")
	if err := os.WriteFile(transcript, lines, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	return home, transcript
}

func totals(t *testing.T, s *store.Store) (sessions int, tokens int) {
	t.Helper()
	list, err := s.ListSessions(context.Background(), model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range list {
		tokens += x.Tokens.Total()
	}
	return len(list), tokens
}

func TestSyncIsIdempotent(t *testing.T) {
	data := mustRead(t, "testdata/claude/session.jsonl")
	_, _ = syncTree(t, data)
	s := syncStore(t)
	ctx := context.Background()

	first, err := Sync(ctx, s, SyncOptions{Agents: []string{model.AgentClaude}})
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if first.FilesParsed == 0 {
		t.Fatal("first sync parsed nothing")
	}
	wantSessions, wantTokens := totals(t, s)

	second, err := Sync(ctx, s, SyncOptions{Agents: []string{model.AgentClaude}})
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if second.FilesParsed != 0 {
		t.Errorf("second sync reparsed %d unchanged files; the watermark must skip them", second.FilesParsed)
	}
	gotSessions, gotTokens := totals(t, s)
	if gotSessions != wantSessions || gotTokens != wantTokens {
		t.Errorf("re-sync changed totals: %d/%d -> %d/%d", wantSessions, wantTokens, gotSessions, gotTokens)
	}
}

// A file that grew must be reparsed whole, and its totals must never shrink —
// the failure that killed mid-file resume.
func TestSyncOnGrowingFileNeverShrinksTotals(t *testing.T) {
	data := mustRead(t, "testdata/claude/session.jsonl")
	half := len(data) / 2
	for half < len(data) && data[half] != '\n' {
		half++
	}
	half++

	_, path := syncTree(t, data[:half])
	s := syncStore(t)
	ctx := context.Background()

	if _, err := Sync(ctx, s, SyncOptions{Agents: []string{model.AgentClaude}}); err != nil {
		t.Fatal(err)
	}
	_, headTokens := totals(t, s)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// mtime granularity: make the change unambiguous
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	stats, err := Sync(ctx, s, SyncOptions{Agents: []string{model.AgentClaude}})
	if err != nil {
		t.Fatal(err)
	}
	if stats.FilesParsed != 1 {
		t.Fatalf("grown file was not reparsed: FilesParsed=%d", stats.FilesParsed)
	}
	sessions, grownTokens := totals(t, s)
	if sessions != 1 {
		t.Fatalf("sessions = %d, want 1", sessions)
	}
	if grownTokens < headTokens {
		t.Errorf("totals shrank after sync: %d -> %d", headTokens, grownTokens)
	}
}

func TestSyncSkipsFilesOlderThanSince(t *testing.T) {
	data := mustRead(t, "testdata/claude/session.jsonl")
	_, path := syncTree(t, data)
	old := time.Now().Add(-90 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	s := syncStore(t)

	stats, err := Sync(context.Background(), s, SyncOptions{
		Agents: []string{model.AgentClaude},
		Since:  time.Now().Add(-DefaultSince),
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.FilesParsed != 0 || stats.Skipped == 0 {
		t.Errorf("--since did not bound the scan: parsed=%d skipped=%d", stats.FilesParsed, stats.Skipped)
	}
}

// An empty session id would upsert a phantom row that ListSessions then returns.
func TestSyncRejectsRecordsWithoutSessionID(t *testing.T) {
	_, path := syncTree(t, []byte(`{"type":"assistant","message":{"id":"m1","model":"claude-fable-5","usage":{"input_tokens":5}}}`+"\n"))
	_ = path
	s := syncStore(t)

	if _, err := Sync(context.Background(), s, SyncOptions{Agents: []string{model.AgentClaude}}); err != nil {
		t.Fatal(err)
	}
	if n, _ := totals(t, s); n != 0 {
		t.Errorf("phantom session ingested: %d rows with no session id", n)
	}
}

func TestSyncFullRebuildEqualsIncremental(t *testing.T) {
	data := mustRead(t, "testdata/claude/session.jsonl")
	_, _ = syncTree(t, data)
	s := syncStore(t)
	ctx := context.Background()

	if _, err := Sync(ctx, s, SyncOptions{Agents: []string{model.AgentClaude}}); err != nil {
		t.Fatal(err)
	}
	incSessions, incTokens := totals(t, s)

	if _, err := Sync(ctx, s, SyncOptions{Agents: []string{model.AgentClaude}, Full: true}); err != nil {
		t.Fatal(err)
	}
	fullSessions, fullTokens := totals(t, s)
	if incSessions != fullSessions || incTokens != fullTokens {
		t.Errorf("full rebuild != incremental: %d/%d vs %d/%d", incSessions, incTokens, fullSessions, fullTokens)
	}
}

// A parent and every subagent it spawns share one promptId. Un-namespaced turn
// ids made them collide in the store, where the upsert let the last writer erase
// the others' tokens — measured as a 50-70% assistant-heavy undercount.
func TestSiblingSubagentsSharingPromptIDDoNotCollide(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "-Users-dev-demo")
	parentID := "11111111-1111-4111-8111-111111111111"
	sub := filepath.Join(proj, parentID, "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	const promptID = "69d4e5b1-1517-4bba-8abb-3f3ada8a279a"
	line := func(sess string, tokens int) string {
		return `{"type":"user","sessionId":"` + sess + `","promptId":"` + promptID + `","timestamp":"2026-07-15T10:00:00.000Z","message":{"content":"go"},"origin":{"kind":"human"}}
{"type":"assistant","sessionId":"` + sess + `","promptId":"` + promptID + `","timestamp":"2026-07-15T10:00:05.000Z","message":{"id":"msg-` + sess[:8] + `","model":"claude-fable-5","usage":{"input_tokens":` + itoa(tokens) + `,"output_tokens":100}}}
`
	}
	if err := os.WriteFile(filepath.Join(proj, parentID+".jsonl"), []byte(line(parentID, 1000)), 0o644); err != nil {
		t.Fatal(err)
	}
	// two siblings: same promptId, no human records (subagent shape), distinct agent files
	subLine := func(agent string, tokens int) string {
		return `{"type":"assistant","sessionId":"` + parentID + `","agentId":"` + agent + `","promptId":"` + promptID + `","timestamp":"2026-07-15T10:00:10.000Z","message":{"id":"msg-` + agent + `","model":"claude-fable-5","usage":{"input_tokens":` + itoa(tokens) + `,"output_tokens":100}}}
`
	}
	for i, ag := range []string{"a1b2c3d4e5f607181", "a1b2c3d4e5f607182"} {
		p := filepath.Join(sub, "agent-"+ag+".jsonl")
		if err := os.WriteFile(p, []byte(subLine(ag, 500+i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := syncStore(t)
	if _, err := Sync(context.Background(), s, SyncOptions{Agents: []string{model.AgentClaude}}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Usage(context.Background(), model.UsageFilter{Group: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	var input int
	for _, r := range rows {
		input += r.Tokens.Input
	}
	// 1000 (parent) + 500 + 501 (siblings) — every writer's tokens survive
	if input != 2001 {
		t.Fatalf("input = %d, want 2001: sibling turns sharing a promptId overwrote each other", input)
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
