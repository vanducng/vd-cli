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
}

func TestTruncIsRuneAware(t *testing.T) {
	if got := trunc("日本語のセッションタイトル", 6); !strings.HasSuffix(got, "…") || strings.ContainsRune(got, '�') {
		t.Fatalf("trunc produced mojibake: %q", got)
	}
}
