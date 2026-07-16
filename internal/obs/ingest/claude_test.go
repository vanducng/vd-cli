package ingest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

const (
	treeRoot   = "testdata/claude/tree"
	parentID   = "11111111-1111-4111-8111-111111111111"
	directSub  = "a1b2c3d4e5f60718a"
	workflowWF = "wf_deadbeef123"
)

func parseFixture(t *testing.T, path string) model.Record {
	t.Helper()
	rec, _, err := ParseClaudeFile(path, &ScanState{})
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return rec
}

func billed(rec model.Record) model.TokenUsage {
	var total model.TokenUsage
	for _, turn := range rec.Turns {
		total.Add(turn.Tokens)
	}
	return total
}

func spanByID(t *testing.T, rec model.Record, id string) model.ToolSpan {
	t.Helper()
	for _, s := range rec.ToolSpans {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("tool span %q not found in %d spans", id, len(rec.ToolSpans))
	return model.ToolSpan{}
}

// TestClaudeEnumerateTiersListsOnlyTopLevelSessions is the tier that fails silently:
// the fixture tree holds 5 .jsonl files, of which only 2 are sessions.
func TestClaudeEnumerateTiersListsOnlyTopLevelSessions(t *testing.T) {
	top, subs, err := EnumerateClaude(treeRoot)
	if err != nil {
		t.Fatalf("enumerate: %v", err)
	}

	wantTop := []string{
		filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID+".jsonl"),
		filepath.Join(treeRoot, "projects", "-Users-dev-other", "22222222-2222-4222-8222-222222222222.jsonl"),
	}
	if len(top) != len(wantTop) {
		t.Fatalf("top-level sessions = %d, want %d: %v", len(top), len(wantTop), top)
	}
	for i, want := range wantTop {
		if top[i] != want {
			t.Errorf("top[%d] = %s, want %s", i, top[i], want)
		}
	}

	subDir := filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID, "subagents")
	wantSubs := []string{
		filepath.Join(subDir, "agent-"+directSub+".jsonl"),
		filepath.Join(subDir, "workflows", workflowWF, "agent-f0e1d2c3b4a59687b.jsonl"),
	}
	if len(subs) != len(wantSubs) {
		t.Fatalf("subagents = %d, want %d: %v", len(subs), len(wantSubs), subs)
	}
	for i, want := range wantSubs {
		if subs[i] != want {
			t.Errorf("subagents[%d] = %s, want %s", i, subs[i], want)
		}
	}
	for _, p := range append(append([]string{}, top...), subs...) {
		if filepath.Base(p) == "journal.jsonl" {
			t.Errorf("journal.jsonl enumerated as a transcript: %s", p)
		}
	}
}

// TestClaudeSubagentTokensCountedOnce is the mechanical defense against the
// double-count trap: the subagent bills its own 5607, and the parent's rollup of the
// same 5607 must stay display-only.
func TestClaudeSubagentTokensCountedOnce(t *testing.T) {
	parent := parseFixture(t, filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID+".jsonl"))
	sub := parseFixture(t, filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID,
		"subagents", "agent-"+directSub+".jsonl"))

	wantParent := model.TokenUsage{Input: 1000, Output: 100}
	if got := billed(parent); got != wantParent {
		t.Errorf("parent billed tokens = %+v, want %+v (rollup must not be summed)", got, wantParent)
	}
	wantSub := model.TokenUsage{Input: 7, Output: 500, CacheRead: 5000, CacheWrite: 100}
	if got := billed(sub); got != wantSub {
		t.Errorf("subagent billed tokens = %+v, want %+v", got, wantSub)
	}
	if got, want := billed(parent).Total()+billed(sub).Total(), 1100+5607; got != want {
		t.Errorf("tree billed total = %d, want %d", got, want)
	}

	span := spanByID(t, parent, "toolu_task_1")
	if span.RollupTokens == nil {
		t.Fatal("Task span has no RollupTokens: the parent's view of the subagent is lost")
	}
	if got := span.RollupTokens.Total(); got != 5607 {
		t.Errorf("RollupTokens.Total() = %d, want 5607", got)
	}
	if span.SubagentSessionID != directSub {
		t.Errorf("SubagentSessionID = %q, want %q", span.SubagentSessionID, directSub)
	}
	if span.SubagentName != "researcher" {
		t.Errorf("SubagentName = %q, want researcher", span.SubagentName)
	}
	if span.Kind != "subagent" {
		t.Errorf("Kind = %q, want subagent", span.Kind)
	}
	if span.DurationMs != 9000 {
		t.Errorf("DurationMs = %d, want 9000", span.DurationMs)
	}
	if sub.Session.ID != directSub || sub.Session.ParentID != parentID {
		t.Errorf("subagent identity = %q/parent %q, want %q/parent %q",
			sub.Session.ID, sub.Session.ParentID, directSub, parentID)
	}
}

func TestClaudeSubagentSessionFields(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantID       string
		wantParent   string
		wantWorkflow string
	}{
		{
			name:       "direct subagent carries no workflow",
			path:       filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID, "subagents", "agent-"+directSub+".jsonl"),
			wantID:     directSub,
			wantParent: parentID,
		},
		{
			name:         "workflow subagent carries the wf_ path segment",
			path:         filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID, "subagents", "workflows", workflowWF, "agent-f0e1d2c3b4a59687b.jsonl"),
			wantID:       "f0e1d2c3b4a59687b",
			wantParent:   parentID,
			wantWorkflow: workflowWF,
		},
		{
			name:   "top-level session has no parent",
			path:   filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID+".jsonl"),
			wantID: parentID,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := parseFixture(t, tc.path).Session
			if s.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", s.ID, tc.wantID)
			}
			if s.ParentID != tc.wantParent {
				t.Errorf("ParentID = %q, want %q", s.ParentID, tc.wantParent)
			}
			if s.WorkflowID != tc.wantWorkflow {
				t.Errorf("WorkflowID = %q, want %q", s.WorkflowID, tc.wantWorkflow)
			}
			if s.Agent != model.AgentClaude {
				t.Errorf("Agent = %q, want %q", s.Agent, model.AgentClaude)
			}
		})
	}
}

// TestClaudeSubagentIdentityFromRecords pins the record-derived identity on its own.
// ParseClaudeFile's filename fallback would otherwise mask a break here, and a
// subagent keeping its parent's sessionId overwrites the parent's row on upsert.
func TestClaudeSubagentIdentityFromRecords(t *testing.T) {
	path := filepath.Join(treeRoot, "projects", "-Users-dev-demo", parentID,
		"subagents", "agent-"+directSub+".jsonl")
	rec, _, err := ParseClaude(bytes.NewReader(mustRead(t, path)), &ScanState{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.Session.ID != directSub {
		t.Errorf("ID = %q, want %q (from the agentId field, no path)", rec.Session.ID, directSub)
	}
	if rec.Session.ParentID != parentID {
		t.Errorf("ParentID = %q, want %q (the sessionId field names the parent)",
			rec.Session.ParentID, parentID)
	}
	for i, turn := range rec.Turns {
		if turn.SessionID != directSub {
			t.Errorf("turn[%d].SessionID = %q, want %q", i, turn.SessionID, directSub)
		}
	}
}

// TestClaudeSubagentIdentityFallsBackToFilename covers the drift case where records
// stop carrying agentId: the id must still not be the parent's.
func TestClaudeSubagentIdentityFallsBackToFilename(t *testing.T) {
	rec := parseFixture(t, "testdata/claude/orphan/subagents/agent-deadc0de1234.jsonl")
	if rec.Session.ID != "deadc0de1234" {
		t.Errorf("ID = %q, want deadc0de1234 (recovered from the filename)", rec.Session.ID)
	}
	if rec.Session.ParentID != "dddddddd-4444-4444-8444-dddddddddddd" {
		t.Errorf("ParentID = %q, want the sessionId field", rec.Session.ParentID)
	}
	for i, turn := range rec.Turns {
		if turn.SessionID != "deadc0de1234" {
			t.Errorf("turn[%d].SessionID = %q, want deadc0de1234", i, turn.SessionID)
		}
	}
}

func TestClaudeTurnsAndTokenDedupe(t *testing.T) {
	rec := parseFixture(t, "testdata/claude/session.jsonl")

	if len(rec.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(rec.Turns))
	}
	// msg_AAA repeats across 3 content-block records with one usage object; counting
	// each record would bill it 3x.
	wantTurns := []model.TokenUsage{
		{Input: 300, Output: 50, CacheRead: 3000, CacheWrite: 110},
		{Input: 300, Output: 40, CacheRead: 3000, CacheWrite: 70},
	}
	for i, want := range wantTurns {
		if got := rec.Turns[i].Tokens; got != want {
			t.Errorf("turn[%d].Tokens = %+v, want %+v", i, got, want)
		}
	}
	wantTotal := model.TokenUsage{Input: 600, Output: 90, CacheRead: 6000, CacheWrite: 180}
	if got := billed(rec); got != wantTotal {
		t.Errorf("session tokens = %+v, want %+v", got, wantTotal)
	}

	if got := rec.Turns[0].DurationMs; got != 4200 {
		t.Errorf("turn[0].DurationMs = %d, want 4200 (from system/turn_duration)", got)
	}
	if got := rec.Turns[1].DurationMs; got != 5000 {
		t.Errorf("turn[1].DurationMs = %d, want 5000 (timestamp delta fallback)", got)
	}

	if got := rec.Turns[0].PromptText; got != "Add a health endpoint." {
		t.Errorf("turn[0].PromptText = %q", got)
	}
	if got := rec.Turns[0].ResponseText; got != "Reading the router first.\nEndpoint added." {
		t.Errorf("turn[0].ResponseText = %q", got)
	}
	for i, turn := range rec.Turns {
		if turn.SessionID != rec.Session.ID {
			t.Errorf("turn[%d].SessionID = %q, want %q", i, turn.SessionID, rec.Session.ID)
		}
		if turn.Index != i {
			t.Errorf("turn[%d].Index = %d", i, turn.Index)
		}
	}

	s := rec.Session
	if s.ID != "aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa" || s.ParentID != "" {
		t.Errorf("session identity = %q/%q", s.ID, s.ParentID)
	}
	if s.Title != "Add health endpoint and test" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Project != "demo" || s.CWD != "/Users/dev/demo" {
		t.Errorf("Project/CWD = %q/%q, want demo//Users/dev/demo", s.Project, s.CWD)
	}
	if s.GitBranch != "main" || s.CLIVersion != "2.1.200" || s.Model != "claude-opus-4-8" {
		t.Errorf("branch/version/model = %q/%q/%q", s.GitBranch, s.CLIVersion, s.Model)
	}
	if s.StartedAt.IsZero() || !s.EndedAt.After(s.StartedAt) {
		t.Errorf("session window = %s..%s", s.StartedAt, s.EndedAt)
	}
}

func TestClaudeToolSpansHooksAndSkills(t *testing.T) {
	st := &ScanState{}
	rec, _, err := ParseClaudeFile("testdata/claude/tools.jsonl", st)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tests := []struct {
		id         string
		wantName   string
		wantKind   string
		wantOK     bool
		wantOutput string
		wantError  string
	}{
		{id: "toolu_bash_1", wantName: "Bash", wantKind: "builtin", wantOK: true,
			wantOutput: "router.go:12: undefined: healthz"},
		{id: "toolu_skill_1", wantName: "Skill", wantKind: "builtin", wantOK: true,
			wantOutput: "debug skill loaded"},
		{id: "toolu_edit_1", wantName: "Edit", wantKind: "builtin", wantOK: false,
			wantError: "String to replace not found in file."},
		{id: "toolu_mcp_1", wantName: "mcp__miudb__query", wantKind: "mcp", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			span := spanByID(t, rec, tc.id)
			if span.Name != tc.wantName || span.Kind != tc.wantKind {
				t.Errorf("name/kind = %q/%q, want %q/%q", span.Name, span.Kind, tc.wantName, tc.wantKind)
			}
			if span.OK != tc.wantOK {
				t.Errorf("OK = %v, want %v", span.OK, tc.wantOK)
			}
			if span.Output != tc.wantOutput {
				t.Errorf("Output = %q, want %q", span.Output, tc.wantOutput)
			}
			if span.Error != tc.wantError {
				t.Errorf("Error = %q, want %q", span.Error, tc.wantError)
			}
			if span.TurnID != "prompt-tools" {
				t.Errorf("TurnID = %q, want prompt-tools", span.TurnID)
			}
		})
	}
	if got := spanByID(t, rec, "toolu_bash_1").Input; got != `{"command":"golangci-lint run","description":"Lint the module"}` {
		t.Errorf("Bash span Input = %q", got)
	}

	wantHooks := []model.HookExec{{
		TurnID: "prompt-tools", HookName: "PostToolUse:lint-guard",
		Event: "PostToolUse", DurationMs: 37, ExitCode: 0,
	}}
	if len(rec.HookExecs) != 1 || rec.HookExecs[0] != wantHooks[0] {
		t.Errorf("HookExecs = %+v, want %+v", rec.HookExecs, wantHooks)
	}

	wantSkills := []model.Skill{{TurnID: "prompt-tools", Name: "debug", Args: "--trace"}}
	if len(rec.Skills) != 1 || rec.Skills[0] != wantSkills[0] {
		t.Errorf("Skills = %+v, want %+v", rec.Skills, wantSkills)
	}

	if got := rec.Turns[0].ResponseText; got != "Running the linter.\nFixed the router." {
		t.Errorf("ResponseText = %q", got)
	}
	if st.UnknownTypes["attachment/task_reminder"] != 1 {
		t.Errorf("unmodelled attachment not counted: %v", st.UnknownTypes)
	}
}

// TestClaudePartialTrailingLineNotCommitted feeds half a record, which is what a live
// session being appended to looks like at sync time.
func TestClaudePartialTrailingLineNotCommitted(t *testing.T) {
	data, err := os.ReadFile("testdata/claude/session.jsonl")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lastLineStart := bytes.LastIndexByte(data[:len(data)-1], '\n') + 1
	cut := lastLineStart + 20
	if cut >= len(data) {
		t.Fatalf("fixture's last line is too short to cut: %d..%d", lastLineStart, len(data))
	}

	st := &ScanState{}
	first, off, err := ParseClaude(bytes.NewReader(data[:cut]), st)
	if err != nil {
		t.Fatalf("parse head: %v", err)
	}
	if off != int64(lastLineStart) {
		t.Fatalf("offset = %d, want %d: a partial line must not be committed", off, lastLineStart)
	}
	if first.Session.Title != "" {
		t.Errorf("Title = %q, want empty: it lives in the uncommitted line", first.Session.Title)
	}
	if st.Offset != off {
		t.Errorf("st.Offset = %d, want %d", st.Offset, off)
	}

	// The file is not resumed mid-way: a resumed parse would re-emit a turn holding
	// only post-offset tokens, and the store's upsert replaces rather than merges.
	// Sync reparses a changed file whole, so that is what this asserts.
	full, off2, err := ParseClaude(bytes.NewReader(data), &ScanState{})
	if err != nil {
		t.Fatalf("reparse whole file: %v", err)
	}
	if off2 != int64(len(data)) {
		t.Errorf("reparsed offset = %d, want %d", off2, len(data))
	}
	if full.Session.Title != "Add health endpoint and test" {
		t.Errorf("reparsed Title = %q: the completed line was not picked up", full.Session.Title)
	}
	// A whole-file parse must never bill less than a parse of one of its prefixes.
	head := billed(first)
	whole := billed(full)
	if whole.Total() < head.Total() {
		t.Errorf("whole-file tokens %+v bill less than the head prefix %+v", whole, head)
	}
	again, _, err := ParseClaude(bytes.NewReader(data), &ScanState{})
	if err != nil {
		t.Fatal(err)
	}
	if billed(again) != whole {
		t.Errorf("reparse is not deterministic: %+v vs %+v", billed(again), whole)
	}
}

func TestClaudeGarbageLineKeepsSessionAndCountsUnknown(t *testing.T) {
	st := &ScanState{}
	rec, _, err := ParseClaude(bytes.NewReader(mustRead(t, "testdata/claude/garbage.jsonl")), st)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.Session.ID != "cccccccc-3333-4333-8333-cccccccccccc" {
		t.Errorf("session lost to a bad line: %q", rec.Session.ID)
	}
	if len(rec.Turns) != 1 {
		t.Fatalf("turns = %d, want 1", len(rec.Turns))
	}
	want := model.TokenUsage{Input: 33, Output: 7}
	if got := billed(rec); got != want {
		t.Errorf("tokens = %+v, want %+v: records either side of the bad line must survive", got, want)
	}
	if st.UnknownTypes["malformed"] != 1 {
		t.Errorf("malformed lines counted = %d, want 1: %v", st.UnknownTypes["malformed"], st.UnknownTypes)
	}
	if st.UnknownTypes["some-future-record-type"] != 1 {
		t.Errorf("unmodelled type not counted: %v", st.UnknownTypes)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
