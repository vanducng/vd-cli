package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// openTestDB goes through the production constructor against a temp file rather
// than ":memory:". Without cache=shared each pooled connection gets its own empty
// database, so any list-then-fetch returns "no such table"; a real file also
// exercises the WAL-once step in New, which is where the interesting bug lives.
func openTestDB(t *testing.T) *Store {
	t.Helper()
	s, err := New(Config{Path: filepath.Join(t.TempDir(), "obs.sqlite")})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedSession(t *testing.T, s *Store, id, agent string, started time.Time, tokens model.TokenUsage) {
	t.Helper()
	rec := model.Record{
		Session: model.Session{
			ID: id, Agent: agent, Title: "session " + id, Model: "claude-fable-5",
			Project: "vd-cli", CWD: "/repo/vd-cli", StartedAt: started, EndedAt: started.Add(time.Minute),
		},
		Turns: []model.Turn{{
			ID: id + "-t1", SessionID: id, Index: 0, Model: "claude-fable-5",
			StartedAt: started, DurationMs: 1200, Tokens: tokens,
			PromptText: "hello", ResponseText: "world",
		}},
	}
	if err := s.IngestFile(context.Background(), rec, Watermark{Path: "/tmp/" + id + ".jsonl", ByteOffset: 10}); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestSchemaIsIdempotentAndRebuildsOnVersionMismatch(t *testing.T) {
	s := openTestDB(t)
	if err := ensureSchema(s.db); err != nil {
		t.Fatalf("second ensureSchema: %v", err)
	}

	seedSession(t, s, "sess-1", model.AgentClaude, time.Now(), model.TokenUsage{Input: 1})

	if _, err := s.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := ensureSchema(s.db); err != nil {
		t.Fatalf("rebuild on mismatch: %v", err)
	}
	n, err := s.CountSessions(context.Background(), model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("version mismatch should drop derived rows, got %d sessions", n)
	}
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != schemaVersion {
		t.Fatalf("user_version = %d, want %d", v, schemaVersion)
	}
}

func TestUpsertOnNaturalKeyIsIdempotent(t *testing.T) {
	s := openTestDB(t)
	now := time.Now()
	seedSession(t, s, "sess-1", model.AgentClaude, now, model.TokenUsage{Input: 10, Output: 5})
	seedSession(t, s, "sess-1", model.AgentClaude, now, model.TokenUsage{Input: 10, Output: 5})

	n, err := s.CountSessions(context.Background(), model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("re-ingest created duplicates: %d sessions", n)
	}
	list, err := s.ListSessions(context.Background(), model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if got := list[0].Tokens.Input; got != 10 {
		t.Fatalf("tokens double-counted on re-ingest: input=%d, want 10", got)
	}
}

func TestListAndCountAgreeUnderFilter(t *testing.T) {
	s := openTestDB(t)
	now := time.Now()
	seedSession(t, s, "claude-1", model.AgentClaude, now, model.TokenUsage{Input: 1})
	seedSession(t, s, "claude-2", model.AgentClaude, now.Add(-time.Hour), model.TokenUsage{Input: 1})
	seedSession(t, s, "codex-01", model.AgentCodex, now, model.TokenUsage{Input: 1})

	f := model.SessionFilter{Agent: model.AgentClaude}
	list, err := s.ListSessions(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	total, err := s.CountSessions(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || total != 2 {
		t.Fatalf("filter drift: list=%d total=%d, want 2/2", len(list), total)
	}
}

// Subagent transcripts carry usage but must never appear as sessions of their own.
func TestSubagentSessionsAreExcludedFromListing(t *testing.T) {
	s := openTestDB(t)
	now := time.Now()
	seedSession(t, s, "parent-1", model.AgentClaude, now, model.TokenUsage{Input: 100})

	sub := model.Record{
		Session: model.Session{
			ID: "sub-1", Agent: model.AgentClaude, ParentID: "parent-1",
			WorkflowID: "wf_abc", StartedAt: now,
		},
		Turns: []model.Turn{{ID: "sub-1-t1", SessionID: "sub-1", StartedAt: now,
			Model: "claude-fable-5", Tokens: model.TokenUsage{Input: 50}}},
	}
	if err := s.IngestFile(context.Background(), sub, Watermark{}); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListSessions(context.Background(), model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "parent-1" {
		t.Fatalf("subagent leaked into session list: %+v", list)
	}

	// but its tokens are still billed
	rows, err := s.Usage(context.Background(), model.UsageFilter{Group: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	var total int
	for _, r := range rows {
		total += r.Tokens.Input
	}
	if total != 150 {
		t.Fatalf("subagent tokens = %d, want 150 (parent 100 + subagent 50, each once)", total)
	}
}

// The :memory: trap: holding a cursor open while querying on another pooled
// connection must not lose the schema.
func TestListThenFetchAcrossPooledConnections(t *testing.T) {
	s := openTestDB(t)
	now := time.Now()
	for _, id := range []string{"a1b2c3d4-one", "a1b2c3d4-two", "b9z8y7x6-three"} {
		seedSession(t, s, id, model.AgentClaude, now, model.TokenUsage{Input: 1})
	}

	rows, err := s.db.Query("SELECT id FROM sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	rows.Next()

	if _, err := s.CountSessions(context.Background(), model.SessionFilter{}); err != nil {
		t.Fatalf("query on second pooled conn while cursor open: %v", err)
	}
}

// Two vd processes (vd web + vd obs sync) first-opening the same fresh file race
// on PRAGMA journal_mode: it is persistent per-file, and SQLite does not run the
// busy handler for journal-mode changes, so busy_timeout cannot cover it. New must
// survive that. Runs repeatedly because the collision is probabilistic.
func TestConcurrentNewOnFreshDB(t *testing.T) {
	for round := 0; round < 20; round++ {
		path := filepath.Join(t.TempDir(), "obs.sqlite")
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, err := New(Config{Path: path})
				if err != nil {
					errs <- err
					return
				}
				if _, err := s.CountSessions(context.Background(), model.SessionFilter{}); err != nil {
					errs <- err
				}
				s.Close()
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("round %d: concurrent New on fresh db: %v", round, err)
		}
	}
}

// A session whose parent link is only discovered on a later pass must not stay
// listed as top-level: ingest resumes mid-file, and the parent's Task span
// resolves after the subagent transcript is already written.
func TestLateLearnedParentIDIsNotDiscarded(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	now := time.Now()
	seedSession(t, s, "parent-1", model.AgentClaude, now, model.TokenUsage{Input: 10})

	// pass 1: no parent known yet
	sub := model.Record{Session: model.Session{ID: "sub-9", Agent: model.AgentClaude, StartedAt: now}}
	if err := s.IngestFile(ctx, sub, Watermark{}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListSessions(ctx, model.SessionFilter{})
	if len(list) != 2 {
		t.Fatalf("pass 1: want subagent listed while unlinked, got %d", len(list))
	}

	// pass 2: the link is learned
	sub.Session.ParentID = "parent-1"
	sub.Session.WorkflowID = "wf_abc"
	sub.Session.Project = "vd-cli"
	if err := s.IngestFile(ctx, sub, Watermark{}); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(ctx, model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "parent-1" {
		t.Fatalf("late-learned parent_id discarded: subagent still listed: %+v", ids(list))
	}
}

// ...and a later parentless re-parse must not unlink it again.
func TestParentIDIsNotClearedByLaterParentlessIngest(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	now := time.Now()
	linked := model.Record{Session: model.Session{
		ID: "sub-9", Agent: model.AgentClaude, ParentID: "parent-1", StartedAt: now}}
	if err := s.IngestFile(ctx, linked, Watermark{}); err != nil {
		t.Fatal(err)
	}
	linked.Session.ParentID = ""
	if err := s.IngestFile(ctx, linked, Watermark{}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListSessions(ctx, model.SessionFilter{})
	if len(list) != 0 {
		t.Fatalf("parentless re-ingest unlinked a subagent: %+v", ids(list))
	}
}

func ids(list []model.SessionSummary) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.ID
	}
	return out
}

// LIKE metacharacters in user input must not widen the filter: `q=%` returning
// everything is the "silently ignored filter" the contract forbids, and a `_` in
// an id prefix must not resolve a different session than the one typed.
func TestLikeMetacharactersDoNotWidenFilters(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	now := time.Now()
	seedSession(t, s, "aaa11111-x", model.AgentClaude, now, model.TokenUsage{})
	seedSession(t, s, "aaa11222-x", model.AgentClaude, now, model.TokenUsage{})

	for _, q := range []string{"%", "_"} {
		list, err := s.ListSessions(ctx, model.SessionFilter{Q: q})
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 0 {
			t.Fatalf("q=%q matched %d sessions; wildcards must be escaped", q, len(list))
		}
	}

	if _, err := s.resolveID(ctx, "aaa11___-x", ""); err != ErrSessionNotFound {
		t.Fatalf("underscore prefix resolved a session it should not match: %v", err)
	}
}

func TestEmptyListMarshalsToArrayNotNull(t *testing.T) {
	s := openTestDB(t)
	list, err := s.ListSessions(context.Background(), model.SessionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(model.SessionList{Sessions: list})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got[:24] != `{"sessions":[],"total":0` {
		t.Fatalf("empty list must marshal as [], got %s", got)
	}
}

func TestResolveIDRejectsShortAndAmbiguousPrefixes(t *testing.T) {
	s := openTestDB(t)
	now := time.Now()
	seedSession(t, s, "019abc11-1111-1111", model.AgentCodex, now, model.TokenUsage{})
	seedSession(t, s, "019abc22-2222-2222", model.AgentCodex, now, model.TokenUsage{})

	ctx := context.Background()
	if _, err := s.resolveID(ctx, "019", ""); err != ErrPrefixTooShort {
		t.Fatalf("short prefix: got %v, want ErrPrefixTooShort", err)
	}
	if _, err := s.resolveID(ctx, "019abc11-1111-1111", ""); err != nil {
		t.Fatalf("exact id: %v", err)
	}
	if _, err := s.resolveID(ctx, "019abc99", ""); err != ErrSessionNotFound {
		t.Fatalf("unknown prefix: got %v, want ErrSessionNotFound", err)
	}
	// UUIDv7 codex ids all share a leading timestamp, so this is the common case
	if _, err := s.resolveID(ctx, "019abc", ""); err != ErrPrefixTooShort {
		t.Fatalf("6-char prefix: got %v, want ErrPrefixTooShort", err)
	}
}

func TestWatermarkRoundTrip(t *testing.T) {
	s := openTestDB(t)
	ctx := context.Background()
	if _, ok, err := s.GetWatermark(ctx, "/nope.jsonl"); err != nil || ok {
		t.Fatalf("unseen path: ok=%v err=%v", ok, err)
	}
	seedSession(t, s, "sess-1", model.AgentClaude, time.Now(), model.TokenUsage{})
	w, ok, err := s.GetWatermark(ctx, "/tmp/sess-1.jsonl")
	if err != nil || !ok {
		t.Fatalf("watermark: ok=%v err=%v", ok, err)
	}
	if w.ByteOffset != 10 {
		t.Fatalf("byte_offset = %d, want 10", w.ByteOffset)
	}
}

func TestTruncateMidKeepsHeadAndTail(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "abcdefghij"
	}
	got := truncateMid(long, 100)
	if len(got) > 100 {
		t.Fatalf("truncated to %d bytes, want <= 100", len(got))
	}
	if got[:3] != "abc" {
		t.Fatalf("head lost: %q", got[:10])
	}
	if got[len(got)-3:] != "hij" {
		t.Fatalf("tail lost: %q", got[len(got)-10:])
	}
	if short := truncateMid("tiny", 100); short != "tiny" {
		t.Fatalf("short payload altered: %q", short)
	}
}
