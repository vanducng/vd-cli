package ingest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

func parseCodexFixture(t *testing.T, name string) model.Record {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "codex", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rec, off, err := ParseCodex(bytes.NewReader(data), &ScanState{})
	if err != nil {
		t.Fatalf("ParseCodex: %v", err)
	}
	if off != int64(len(data)) {
		t.Fatalf("offset = %d, want %d (whole fixture ends in a newline)", off, len(data))
	}
	return rec
}

func TestCodexSessionMetadata(t *testing.T) {
	rec := parseCodexFixture(t, "session-basic.jsonl")
	s := rec.Session

	tests := []struct {
		field string
		got   string
		want  string
	}{
		{"ID", s.ID, "019f0000-0000-7000-8000-000000000001"},
		{"Agent", s.Agent, model.AgentCodex},
		{"CWD", s.CWD, "/home/dev/src/widget-api"},
		{"Project", s.Project, "widget-api"},
		{"GitBranch", s.GitBranch, "feat/widget"},
		{"GitSHA", s.GitSHA, "1111111111111111111111111111111111111111"},
		{"Originator", s.Originator, "codex_cli"},
		{"CLIVersion", s.CLIVersion, "9.9.9"},
		{"Model", s.Model, "synthetic-large"},
		{"ParentID", s.ParentID, ""},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("Session.%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}
	if got := s.StartedAt.UTC().Format("2006-01-02T15:04:05Z"); got != "2026-07-01T10:00:00Z" {
		t.Errorf("StartedAt = %s", got)
	}
	if got := s.EndedAt.UTC().Format("2006-01-02T15:04:05Z"); got != "2026-07-01T10:00:13Z" {
		t.Errorf("EndedAt = %s, want the last record's timestamp", got)
	}
}

// Three turn_context records carry two distinct turn_ids: codex re-emits the
// context inside a turn, so a turn per record would fragment the session.
func TestCodexTurnGrouping(t *testing.T) {
	rec := parseCodexFixture(t, "session-basic.jsonl")
	if len(rec.Turns) != 2 {
		t.Fatalf("len(Turns) = %d, want 2 (turn_context is re-emitted within a turn)", len(rec.Turns))
	}

	tests := []struct {
		idx      int
		id       string
		model    string
		prompt   string
		response string
		duration int64
	}{
		{0, "019f0000-0000-7000-8000-000000000001:turn-aaaa", "synthetic-large", "Add a health endpoint.", "Added the health endpoint.", 5000},
		{1, "019f0000-0000-7000-8000-000000000001:turn-bbbb", "synthetic-small", "Now add a test.", "Test added.", 2500},
	}
	for _, tt := range tests {
		got := rec.Turns[tt.idx]
		if got.ID != tt.id {
			t.Errorf("Turns[%d].ID = %q, want %q", tt.idx, got.ID, tt.id)
		}
		if got.Index != tt.idx {
			t.Errorf("Turns[%d].Index = %d", tt.idx, got.Index)
		}
		if got.SessionID != "019f0000-0000-7000-8000-000000000001" {
			t.Errorf("Turns[%d].SessionID = %q", tt.idx, got.SessionID)
		}
		if got.Model != tt.model {
			t.Errorf("Turns[%d].Model = %q, want %q", tt.idx, got.Model, tt.model)
		}
		if got.PromptText != tt.prompt {
			t.Errorf("Turns[%d].PromptText = %q, want %q", tt.idx, got.PromptText, tt.prompt)
		}
		if got.ResponseText != tt.response {
			t.Errorf("Turns[%d].ResponseText = %q, want %q", tt.idx, got.ResponseText, tt.response)
		}
		if got.DurationMs != tt.duration {
			t.Errorf("Turns[%d].DurationMs = %d, want %d", tt.idx, got.DurationMs, tt.duration)
		}
	}
}

// Tokens are summed from last_token_usage (per request). Summing the adjacent
// total_token_usage instead would yield input 1000+3000=4000 on turn one.
func TestCodexPerRequestTokens(t *testing.T) {
	rec := parseCodexFixture(t, "session-basic.jsonl")

	tests := []struct {
		idx       int
		want      model.TokenUsage
		wantTotal int
	}{
		// (1000-400)+(2000-1500) input, 100+200 output, 400+1500 cached, 40+60 reasoning.
		{0, model.TokenUsage{Input: 1100, Output: 300, CacheRead: 1900, ReasoningOutput: 100}, 3300},
		// (500-100) input, 50 output, 100 cached, 10 reasoning.
		{1, model.TokenUsage{Input: 400, Output: 50, CacheRead: 100, ReasoningOutput: 10}, 550},
	}
	for _, tt := range tests {
		got := rec.Turns[tt.idx].Tokens
		if got != tt.want {
			t.Errorf("Turns[%d].Tokens = %+v, want %+v", tt.idx, got, tt.want)
		}
		// Total must equal codex's own total_tokens for those requests.
		if got.Total() != tt.wantTotal {
			t.Errorf("Turns[%d].Tokens.Total() = %d, want %d", tt.idx, got.Total(), tt.wantTotal)
		}
	}
}

// reasoning_output_tokens is a breakdown of output_tokens, not an addend.
func TestCodexReasoningNotDoubleCounted(t *testing.T) {
	rec := parseCodexFixture(t, "session-basic.jsonl")
	tok := rec.Turns[0].Tokens

	if tok.Output != 300 {
		t.Errorf("Output = %d, want 300 (reasoning must not inflate it)", tok.Output)
	}
	if tok.ReasoningOutput != 100 {
		t.Errorf("ReasoningOutput = %d, want 100", tok.ReasoningOutput)
	}
	if tok.ReasoningOutput > tok.Output {
		t.Errorf("ReasoningOutput %d > Output %d: reasoning lives inside output",
			tok.ReasoningOutput, tok.Output)
	}
	if got := tok.Total(); got != 3300 {
		t.Errorf("Total() = %d, want 3300 (adding reasoning again would give 3400)", got)
	}
}

// cached_input_tokens is a subset of input_tokens, but Total() adds CacheRead to
// Input, so the cached share must be netted out of Input at parse time.
func TestCodexCachedTokensNotDoubleCounted(t *testing.T) {
	tests := []struct {
		name      string
		in        codexTokens
		want      model.TokenUsage
		wantTotal int
	}{
		{
			name:      "cached subset of input",
			in:        codexTokens{Input: 1000, Cached: 400, Output: 100, Reasoning: 40},
			want:      model.TokenUsage{Input: 600, Output: 100, CacheRead: 400, ReasoningOutput: 40},
			wantTotal: 1100,
		},
		{
			name:      "fully cached input",
			in:        codexTokens{Input: 900, Cached: 900, Output: 10},
			want:      model.TokenUsage{Input: 0, Output: 10, CacheRead: 900},
			wantTotal: 910,
		},
		{
			name:      "no cache",
			in:        codexTokens{Input: 500, Output: 50},
			want:      model.TokenUsage{Input: 500, Output: 50},
			wantTotal: 550,
		},
		{
			name:      "cached exceeding input clamps rather than going negative",
			in:        codexTokens{Input: 100, Cached: 300, Output: 5},
			want:      model.TokenUsage{Input: 0, Output: 5, CacheRead: 300},
			wantTotal: 305,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codexUsage(tt.in)
			if got != tt.want {
				t.Errorf("codexUsage(%+v) = %+v, want %+v", tt.in, got, tt.want)
			}
			if got.Total() != tt.wantTotal {
				t.Errorf("Total() = %d, want %d (codex total_tokens = input+output)",
					got.Total(), tt.wantTotal)
			}
		})
	}
}

func TestCodexToolSpansJoinedByCallID(t *testing.T) {
	rec := parseCodexFixture(t, "tools-aborted.jsonl")
	if len(rec.ToolSpans) != 4 {
		t.Fatalf("len(ToolSpans) = %d, want 4", len(rec.ToolSpans))
	}

	byID := map[string]model.ToolSpan{}
	for _, sp := range rec.ToolSpans {
		byID[sp.ID] = sp
	}

	tests := []struct {
		callID     string
		name       string
		kind       string
		input      string
		wantOutput string
		durationMs int64
	}{
		{
			callID:     "call_AAA",
			name:       "exec",
			kind:       "custom_tool_call",
			input:      "git status --short",
			wantOutput: "Script completed\nWall time 2.5 seconds\nOutput:\n M app/main.go\n",
			durationMs: 2500,
		},
		{
			callID:     "call_BBB",
			name:       "exec",
			kind:       "custom_tool_call",
			input:      "const hits = ALL_TOOLS.filter(x => /docs/i.test(x.name));\ntext(hits);\n",
			wantOutput: "Script completed\nOutput:\n[]\n",
			durationMs: 1000,
		},
		{
			callID:     "call_DDD",
			name:       "exec",
			kind:       "custom_tool_call",
			input:      "cat go.mod\nls -la",
			wantOutput: "Script completed\nOutput:\nmodule example.invalid/widget-api\n",
			durationMs: 1000,
		},
		{
			callID:     "call_CCC",
			name:       "exec_command",
			kind:       "function_call",
			input:      "go test ./...",
			wantOutput: "Chunk ID: 0f7e3b\nWall time: 2.0000 seconds\nProcess exited with code 0\nok example.invalid/widget-api\n",
			durationMs: 2000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.callID, func(t *testing.T) {
			sp, ok := byID[tt.callID]
			if !ok {
				t.Fatalf("no span for call_id %s", tt.callID)
			}
			if sp.Name != tt.name {
				t.Errorf("Name = %q, want %q", sp.Name, tt.name)
			}
			if sp.Kind != tt.kind {
				t.Errorf("Kind = %q, want %q", sp.Kind, tt.kind)
			}
			if sp.Input != tt.input {
				t.Errorf("Input = %q, want %q", sp.Input, tt.input)
			}
			if sp.Output != tt.wantOutput {
				t.Errorf("Output = %q, want %q", sp.Output, tt.wantOutput)
			}
			if sp.DurationMs != tt.durationMs {
				t.Errorf("DurationMs = %d, want %d", sp.DurationMs, tt.durationMs)
			}
			if !sp.OK {
				t.Errorf("OK = false, want true")
			}
			if sp.TurnID != "019f0000-0000-7000-8000-000000000002:turn-cccc" {
				t.Errorf("TurnID = %q, want turn-cccc", sp.TurnID)
			}
		})
	}
}

// An output whose call_id was never opened must not fabricate a span.
func TestCodexOrphanToolOutputCounted(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "codex", "tools-aborted.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	st := &ScanState{}
	rec, _, err := ParseCodex(bytes.NewReader(data), st)
	if err != nil {
		t.Fatalf("ParseCodex: %v", err)
	}
	for _, sp := range rec.ToolSpans {
		if sp.ID == "call_ORPHAN" {
			t.Fatalf("orphan output produced a span")
		}
	}
	if st.UnknownTypes["orphan_tool_output"] != 1 {
		t.Errorf("UnknownTypes[orphan_tool_output] = %d, want 1",
			st.UnknownTypes["orphan_tool_output"])
	}
}

func TestCodexAbortedTurnDuration(t *testing.T) {
	rec := parseCodexFixture(t, "tools-aborted.jsonl")
	if len(rec.Turns) != 1 {
		t.Fatalf("len(Turns) = %d, want 1", len(rec.Turns))
	}
	if got := rec.Turns[0].DurationMs; got != 189494 {
		t.Errorf("DurationMs = %d, want 189494 from turn_aborted", got)
	}
}

func TestCodexCommandExtraction(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "js wrapper",
			raw:  `const r = await tools.exec_command({cmd:"git status --short",workdir:"/repo"});`,
			want: "git status --short",
		},
		{
			name: "js wrapper with escaped quotes",
			raw:  `tools.exec_command({cmd:"rg -n \"func main\" .",workdir:"/repo"});`,
			want: `rg -n "func main" .`,
		},
		{
			name: "js wrapper with newline escape",
			raw:  `tools.exec_command({cmd:"printf 'a\nb'",workdir:"/repo"});`,
			want: "printf 'a\nb'",
		},
		{
			name: "multiple exec_command calls join",
			raw:  "await Promise.all([\n tools.exec_command({cmd:\"cat go.mod\",workdir:\"/r\"}),\n tools.exec_command({cmd:\"ls -la\",workdir:\"/r\"}),\n]);",
			want: "cat go.mod\nls -la",
		},
		{
			name: "function_call json arguments",
			raw:  `{"cmd":"go test ./...","workdir":"/repo"}`,
			want: "go test ./...",
		},
		{
			name: "no exec_command keeps raw",
			raw:  "const hits = ALL_TOOLS.filter(x => /docs/i.test(x.name));\ntext(hits);\n",
			want: "const hits = ALL_TOOLS.filter(x => /docs/i.test(x.name));\ntext(hits);\n",
		},
		{
			name: "json without cmd keeps raw",
			raw:  `{"workdir":"/repo"}`,
			want: `{"workdir":"/repo"}`,
		},
		{
			name: "empty stays empty",
			raw:  "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := codexCommand(tt.raw); got != tt.want {
				t.Errorf("codexCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Codex has neither hooks nor skills; synthesizing either would invent data.
func TestCodexNoHooksOrSkills(t *testing.T) {
	for _, name := range []string{"session-basic.jsonl", "tools-aborted.jsonl", "subagent-legacy.jsonl"} {
		t.Run(name, func(t *testing.T) {
			rec := parseCodexFixture(t, name)
			if len(rec.HookExecs) != 0 {
				t.Errorf("HookExecs = %d, want 0", len(rec.HookExecs))
			}
			if len(rec.Skills) != 0 {
				t.Errorf("Skills = %d, want 0", len(rec.Skills))
			}
			for _, turn := range rec.Turns {
				if len(turn.HookExecs) != 0 || len(turn.Skills) != 0 {
					t.Errorf("turn %s carries hooks/skills", turn.ID)
				}
			}
		})
	}
}

// Subagent and pre-2026-06 rollouts never emit turn_context; user_message is the
// only boundary, and their tokens must still land on a turn.
func TestCodexLegacySubagentTurns(t *testing.T) {
	rec := parseCodexFixture(t, "subagent-legacy.jsonl")

	if rec.Session.ParentID != "019f0000-0000-7000-8000-000000000001" {
		t.Errorf("ParentID = %q, want the spawning thread", rec.Session.ParentID)
	}
	if len(rec.Turns) != 2 {
		t.Fatalf("len(Turns) = %d, want 2 (one per user_message)", len(rec.Turns))
	}
	tests := []struct {
		idx    int
		prompt string
		want   model.TokenUsage
	}{
		{0, "Scan the handlers.", model.TokenUsage{Input: 500, Output: 80, CacheRead: 300, ReasoningOutput: 20}},
		{1, "Now summarize them.", model.TokenUsage{Input: 400, Output: 60, CacheRead: 200, ReasoningOutput: 15}},
	}
	for _, tt := range tests {
		got := rec.Turns[tt.idx]
		if got.PromptText != tt.prompt {
			t.Errorf("Turns[%d].PromptText = %q, want %q", tt.idx, got.PromptText, tt.prompt)
		}
		if got.Tokens != tt.want {
			t.Errorf("Turns[%d].Tokens = %+v, want %+v", tt.idx, got.Tokens, tt.want)
		}
		if got.ID == "" {
			t.Errorf("Turns[%d].ID is empty; the store keys turns by id", tt.idx)
		}
	}
	if rec.Turns[0].ID == rec.Turns[1].ID {
		t.Errorf("implicit turn ids collide: %q", rec.Turns[0].ID)
	}
}

// A rollout is appended to while we read it: the trailing partial line must stay
// uncommitted, and the next pass must pick it up from the returned offset.
func TestCodexResumePartialLine(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "codex", "session-basic.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lastLine := bytes.LastIndex(data[:len(data)-1], []byte("\n")) + 1
	cut := lastLine + 40 // mid-way through the final task_complete record
	if cut >= len(data) {
		t.Fatalf("fixture too short to truncate")
	}

	st := &ScanState{}
	first, off, err := ParseCodex(bytes.NewReader(data[:cut]), st)
	if err != nil {
		t.Fatalf("ParseCodex(partial): %v", err)
	}
	if off != int64(lastLine) {
		t.Fatalf("offset = %d, want %d (the partial line must not be committed)", off, lastLine)
	}
	if st.Offset != off {
		t.Errorf("st.Offset = %d, want %d", st.Offset, off)
	}
	if len(first.Turns) != 2 {
		t.Fatalf("len(Turns) = %d, want 2", len(first.Turns))
	}
	if got := first.Turns[1].DurationMs; got != 0 {
		t.Errorf("DurationMs = %d, want 0: the truncated task_complete is not a record yet", got)
	}

	// Changed files are reparsed whole rather than resumed mid-way: a resumed parse
	// re-emits a turn holding only post-offset tokens, and the store's upsert
	// replaces rather than merges, so stored totals would shrink.
	full, off2, err := ParseCodex(bytes.NewReader(data), &ScanState{})
	if err != nil {
		t.Fatalf("ParseCodex(whole): %v", err)
	}
	if off2 != int64(len(data)) {
		t.Errorf("offset after whole parse = %d, want %d", off2, len(data))
	}
	if full.Session.ID == "" {
		t.Error("whole parse must carry the session id: an empty id upserts a phantom session row")
	}
	if len(full.Turns) != 2 {
		t.Fatalf("whole len(Turns) = %d, want 2", len(full.Turns))
	}
	if got := full.Turns[1].DurationMs; got != 2500 {
		t.Errorf("whole DurationMs = %d, want 2500", got)
	}
	// The head's partial view must never bill more than the whole file.
	if headTok, wholeTok := billed(first), billed(full); wholeTok.Total() < headTok.Total() {
		t.Errorf("whole-file tokens %+v bill less than the head prefix %+v", wholeTok, headTok)
	}
}

func TestCodexUnknownTypesCounted(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "codex", "session-basic.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	st := &ScanState{}
	if _, _, err := ParseCodex(bytes.NewReader(data), st); err != nil {
		t.Fatalf("ParseCodex: %v", err)
	}
	if st.UnknownTypes["world_state"] != 1 {
		t.Errorf("UnknownTypes[world_state] = %d, want 1", st.UnknownTypes["world_state"])
	}
	if st.UnknownTypes["event_msg/task_started"] != 2 {
		t.Errorf("UnknownTypes[event_msg/task_started] = %d, want 2",
			st.UnknownTypes["event_msg/task_started"])
	}
}

// A corrupt line must not abort the parse of the lines around it.
func TestCodexMalformedLineTolerated(t *testing.T) {
	in := "{\"timestamp\":\"2026-07-01T10:00:00.000Z\",\"type\":\"session_meta\",\"payload\":{\"session_id\":\"s1\",\"cwd\":\"/tmp/x\"}}\n" +
		"{not json at all\n" +
		"{\"timestamp\":\"2026-07-01T10:00:02.000Z\",\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"t1\",\"model\":\"m\"}}\n"

	st := &ScanState{}
	rec, off, err := ParseCodex(bytes.NewReader([]byte(in)), st)
	if err != nil {
		t.Fatalf("ParseCodex: %v", err)
	}
	if off != int64(len(in)) {
		t.Errorf("offset = %d, want %d", off, len(in))
	}
	if rec.Session.ID != "s1" {
		t.Errorf("Session.ID = %q, want s1", rec.Session.ID)
	}
	if len(rec.Turns) != 1 {
		t.Errorf("len(Turns) = %d, want 1: the good line after the bad one still parses", len(rec.Turns))
	}
	if st.UnknownTypes["malformed"] != 1 {
		t.Errorf("UnknownTypes[malformed] = %d, want 1", st.UnknownTypes["malformed"])
	}
}

func TestEnumerateCodex(t *testing.T) {
	root := t.TempDir()
	want := []string{
		filepath.Join(root, "2026", "06", "25", "rollout-2026-06-25T00-29-08-aaaa.jsonl"),
		filepath.Join(root, "2026", "07", "01", "rollout-2026-07-01T10-00-00-bbbb.jsonl"),
		filepath.Join(root, "2026", "07", "02", "rollout-2026-07-02T08-00-00-cccc.jsonl"),
	}
	ignored := []string{
		filepath.Join(root, "2026", "07", "02", "notes.txt"),
		filepath.Join(root, "2026", "07", "02", "rollout-2026-07-02T08-00-00-dddd.jsonl.tmp"),
		filepath.Join(root, "2026", "07", "rollout-2026-07-02T08-00-00-eeee.jsonl"),
	}
	for _, p := range append(append([]string{}, want...), ignored...) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got, err := EnumerateCodex(root)
	if err != nil {
		t.Fatalf("EnumerateCodex: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestEnumerateCodexMissingRoot(t *testing.T) {
	got, err := EnumerateCodex(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("EnumerateCodex on a missing root should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// Codex re-emits token_count for one API request with an identical last_token_usage;
// billing every event doubles the bill. Real rollouts contain these pairs.
func TestCodexDuplicateTokenCountBilledOnce(t *testing.T) {
	rec, _, err := ParseCodex(bytes.NewReader(mustRead(t, "testdata/codex/dup_tokens.jsonl")), &ScanState{})
	if err != nil {
		t.Fatal(err)
	}
	got := billed(rec)
	// codex input_tokens includes cached, so uncached Input = 100-40. The fixture
	// holds one true duplicate (identical last AND total -> dropped) plus one
	// distinct request with identical last but advanced total -> billed. 2x60=120.
	want := model.TokenUsage{Input: 120, Output: 40, CacheRead: 80, ReasoningOutput: 10}
	if got != want {
		t.Errorf("tokens = %+v, want %+v: duplicate/distinct token_count misjudged", got, want)
	}
}
