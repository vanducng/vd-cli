package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/obs/model"
)

// Transcript-derived strings reach the terminal from files other agents wrote:
// raw C0/C1 bytes would let an injected title retitle the terminal, clear the
// screen, or write the clipboard via OSC52.
func TestRenderersStripTerminalEscapes(t *testing.T) {
	evil := "\x1b]0;pwned\a\x1b[2J\x1b[31mred"
	list := &model.SessionList{Sessions: []model.SessionSummary{{
		Session: model.Session{ID: "s1", Agent: evil, Title: evil, Model: evil, StartedAt: time.Now()},
	}}, Total: 1, Limit: 50}

	var buf bytes.Buffer
	renderSessions(&buf, list)
	if strings.ContainsRune(buf.String(), '\x1b') || strings.ContainsRune(buf.String(), '\a') {
		t.Fatalf("renderSessions leaked escape bytes: %q", buf.String())
	}

	detail := &model.SessionDetail{
		SessionSummary: list.Sessions[0],
		Turns: []model.Turn{{
			Index: 0, StartedAt: time.Now(), PromptText: evil,
			ToolSpans: []model.ToolSpan{{Name: evil, OK: false}},
			HookExecs: []model.HookExec{{HookName: evil, Event: "PreToolUse"}},
			Skills:    []model.Skill{{Name: evil}},
		}},
	}
	buf.Reset()
	renderSession(&buf, detail)
	if strings.ContainsRune(buf.String(), '\x1b') || strings.ContainsRune(buf.String(), '\a') {
		t.Fatalf("renderSession leaked escape bytes: %q", buf.String())
	}

	rep := &model.UsageReport{
		Rows:           []model.UsageRow{{Date: "2026-07-17", Agent: evil, Model: evil}},
		UnpricedModels: []string{evil},
	}
	buf.Reset()
	renderUsage(&buf, rep)
	if strings.ContainsRune(buf.String(), '\x1b') || strings.ContainsRune(buf.String(), '\a') {
		t.Fatalf("renderUsage leaked escape bytes: %q", buf.String())
	}

	skills := &model.SkillReport{Skills: []model.SkillSummary{
		{Name: evil, Agents: []string{evil}, Invocations: 1, ToolCalls: 1, ToolErrors: 1},
	}}
	buf.Reset()
	renderSkills(&buf, skills)
	if strings.ContainsRune(buf.String(), '\x1b') || strings.ContainsRune(buf.String(), '\a') {
		t.Fatalf("renderSkills leaked escape bytes: %q", buf.String())
	}

	hooks := &model.HookReport{Hooks: []model.HookSummary{
		{HookName: evil, Event: evil, Fires: 2, NonzeroExits: 1},
	}}
	buf.Reset()
	renderHooks(&buf, hooks)
	if strings.ContainsRune(buf.String(), '\x1b') || strings.ContainsRune(buf.String(), '\a') {
		t.Fatalf("renderHooks leaked escape bytes: %q", buf.String())
	}
}

func TestRenderHooksColumns(t *testing.T) {
	block, share := 0.4167, 0.42
	rep := &model.HookReport{Hooks: []model.HookSummary{
		{HookName: "scout-block", Event: "PreToolUse", Fires: 120, NonzeroExits: 50,
			BlockRate: &block, ErrShare: &share},
	}}
	var buf bytes.Buffer
	renderHooks(&buf, rep)
	out := buf.String()
	for _, want := range []string{"scout-block", "PreToolUse", "41.7%", "42.0%", "claude-only"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderHooks output missing %q\n%s", want, out)
		}
	}

	empty := &model.HookReport{}
	buf.Reset()
	renderHooks(&buf, empty)
	if !strings.Contains(buf.String(), "no hook activity") {
		t.Errorf("empty hook report should say so, got %q", buf.String())
	}
}

func TestRenderSkillsColumnsAndNilRate(t *testing.T) {
	rate := 0.043
	rep := &model.SkillReport{Skills: []model.SkillSummary{
		{Name: "ship", Agents: []string{model.AgentClaude, model.AgentCodex},
			Invocations: 12, Sessions: 9, SoloSessions: 4, ToolCalls: 3348, ToolErrors: 144,
			ErrRate: &rate, Tokens: 2_500_000, Corrections: 6, Aborts: 2},
		{Name: model.SkillNone, ToolCalls: 100, ToolErrors: 0},
	}}
	var buf bytes.Buffer
	renderSkills(&buf, rep)
	out := buf.String()

	for _, want := range []string{"ship", "claude+codex", "4.3%", "2.5M", "(none)", "CORR", "ABRT"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderSkills output missing %q\n%s", want, out)
		}
	}
	// (none) has no invocations and thus a nil ErrRate: rendered "-", never "0.0%".
	none := lineContaining(out, "(none)")
	if none == "" {
		t.Fatalf("no (none) row in output:\n%s", out)
	}
	if strings.Contains(none, "%") {
		t.Errorf("(none) row shows a percentage for its undefined error rate: %q", none)
	}

	empty := &model.SkillReport{}
	buf.Reset()
	renderSkills(&buf, empty)
	if !strings.Contains(buf.String(), "no skill activity") {
		t.Errorf("empty report should say so, got %q", buf.String())
	}
}

func lineContaining(s, sub string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.Contains(ln, sub) {
			return ln
		}
	}
	return ""
}

func TestTruncIsRuneAware(t *testing.T) {
	if got := trunc("日本語のセッションタイトル", 6); !strings.HasSuffix(got, "…") || strings.ContainsRune(got, '�') {
		t.Fatalf("trunc produced mojibake: %q", got)
	}
}
