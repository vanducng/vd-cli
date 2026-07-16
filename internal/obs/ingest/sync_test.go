package ingest

import (
	"context"
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
