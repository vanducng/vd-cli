package ingest

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeriveCodexTitleFromFirstPrompt(t *testing.T) {
	got := deriveCodexTitle("Add a health endpoint.")
	if got != "Add a health endpoint." {
		t.Errorf("deriveCodexTitle = %q, want %q", got, "Add a health endpoint.")
	}
}

func TestDeriveCodexTitleStripsLeadingSkillToken(t *testing.T) {
	tests := []struct{ msg, want string }{
		{"$vd:ship Refactor the auth module please", "Refactor the auth module please"},
		{"$ship Refactor the auth module please", "Refactor the auth module please"},
		{"Not $vd:ship at the start", "Not $vd:ship at the start"},
	}
	for _, tt := range tests {
		if got := deriveCodexTitle(tt.msg); got != tt.want {
			t.Errorf("deriveCodexTitle(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}

func TestDeriveCodexTitleTakesFirstNonEmptyLine(t *testing.T) {
	msg := "\n\n   \nFix the flaky retry test\nsecond line ignored"
	if got, want := deriveCodexTitle(msg), "Fix the flaky retry test"; got != want {
		t.Errorf("deriveCodexTitle = %q, want %q", got, want)
	}
}

func TestDeriveCodexTitleCollapsesWhitespace(t *testing.T) {
	if got, want := deriveCodexTitle("Fix   the    spacing   bug"), "Fix the spacing bug"; got != want {
		t.Errorf("deriveCodexTitle = %q, want %q", got, want)
	}
}

func TestDeriveCodexTitleEmptyOrWhitespaceStaysEmpty(t *testing.T) {
	for _, msg := range []string{"", "   ", "\n\n\t \n"} {
		if got := deriveCodexTitle(msg); got != "" {
			t.Errorf("deriveCodexTitle(%q) = %q, want empty (no fabricated title)", msg, got)
		}
	}
}

func TestDeriveCodexTitleTruncatesOnWordBoundary(t *testing.T) {
	msg := strings.Repeat("word ", 30) // 150 runes, well over the 80 cap
	got := deriveCodexTitle(msg)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("deriveCodexTitle = %q, want a truncated title ending in …", got)
	}
	if r := []rune(got); len(r) > maxDerivedTitleLen+1 { // +1 for the … itself
		t.Errorf("deriveCodexTitle len = %d runes, want <= %d", len(r), maxDerivedTitleLen+1)
	}
	if strings.Contains(strings.TrimSuffix(got, "…"), "wor ") {
		t.Errorf("deriveCodexTitle = %q, truncated mid-word instead of on a word boundary", got)
	}
}

func TestDeriveCodexTitleShortMessageUntruncated(t *testing.T) {
	msg := "Short prompt"
	if got := deriveCodexTitle(msg); got != msg {
		t.Errorf("deriveCodexTitle(%q) = %q, want unchanged", msg, got)
	}
}

// Every secretPatterns entry gets a positive case (secret in the derived
// title gets redacted) and a clean-text negative case (unrelated text is
// left untouched).
func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"openai-key positive", "leaked key sk-abcdEFGH12345678xyz here", "leaked key … here"},
		{"openai-key negative", "sk-short is not a real key", "sk-short is not a real key"},

		{"ghp-token positive", "found ghp_1234567890abcdEFGH in the log", "found … in the log"},
		{"gho-token positive", "found gho_1234567890abcdEFGH in the log", "found … in the log"},
		{"github-pat positive", "leaked github_pat_11ABCDEFG0123456789_abcdefghij", "leaked …"},
		{"github-token negative", "ghost story about ghosts", "ghost story about ghosts"},

		{"aws-key positive", "rotate AKIAABCDEFGHIJ12345K now", "rotate … now"},
		{"aws-key negative", "AKIA short id", "AKIA short id"},

		{"slack-token positive", "webhook xoxb-1234567890-abcdefghij broke", "webhook … broke"},
		{"slack-token negative", "xox is not a token prefix alone", "xox is not a token prefix alone"},

		{"jwt positive", "auth eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 failing", "auth … failing"},
		{"jwt negative", "eyJshort is not a jwt", "eyJshort is not a jwt"},

		{"bearer positive", "curl -H Bearer abcdefghijklmnopqrstuvwx failed", "curl -H … failed"},
		{"bearer negative", "Bearer abc failed with short token", "Bearer abc failed with short token"},

		{"password positive", "fix login password=Sup3rSecret!123 bug", "fix login … bug"},
		{"token-kv positive", "rotate token=abcdef123456 asap", "rotate … asap"},
		{"apikey-kv positive", "set apikey=zzz999yyy888 config", "set … config"},
		{"secret-kv positive", "leaked secret=topsecretvalue1 oops", "leaked … oops"},
		{"kv negative", "reset the user password after signup", "reset the user password after signup"},

		{"long-hex-run positive", "blob deadbeefdeadbeefdeadbeefdeadbeefcafe stuck", "blob … stuck"},
		{"long-run negative", "just a normal short sentence", "just a normal short sentence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactSecrets(tt.input); got != tt.want {
				t.Errorf("redactSecrets(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDeriveCodexTitleRedactsBeforeTruncation(t *testing.T) {
	msg := "Rotate the leaked key sk-abcdEFGH12345678xyz immediately"
	got := deriveCodexTitle(msg)
	if strings.Contains(got, "sk-abcdEFGH12345678xyz") {
		t.Errorf("deriveCodexTitle = %q, secret was not redacted", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("deriveCodexTitle = %q, want redaction marker present", got)
	}
}

// End to end through ParseCodex: a rollout with no title field gets one
// derived from its first user message, and TitleDerived is set.
func TestCodexTitleDerivedEndToEnd(t *testing.T) {
	in := "{\"timestamp\":\"2026-07-01T10:00:00.000Z\",\"type\":\"session_meta\",\"payload\":{\"session_id\":\"s1\",\"cwd\":\"/tmp/x\"}}\n" +
		"{\"timestamp\":\"2026-07-01T10:00:01.000Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"Add rate limiting to the API.\"}}\n"

	rec, _, err := ParseCodex(bytes.NewReader([]byte(in)), &ScanState{}, nil)
	if err != nil {
		t.Fatalf("ParseCodex: %v", err)
	}
	if rec.Session.Title != "Add rate limiting to the API." {
		t.Errorf("Title = %q, want derived from first prompt", rec.Session.Title)
	}
	if !rec.Session.TitleDerived {
		t.Errorf("TitleDerived = false, want true")
	}
}

// A whitespace-only first message must not fabricate a title.
func TestCodexTitleWhitespaceOnlyFirstMessageStaysEmpty(t *testing.T) {
	in := "{\"timestamp\":\"2026-07-01T10:00:00.000Z\",\"type\":\"session_meta\",\"payload\":{\"session_id\":\"s2\",\"cwd\":\"/tmp/x\"}}\n" +
		"{\"timestamp\":\"2026-07-01T10:00:01.000Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"user_message\",\"message\":\"   \\n  \"}}\n"

	rec, _, err := ParseCodex(bytes.NewReader([]byte(in)), &ScanState{}, nil)
	if err != nil {
		t.Fatalf("ParseCodex: %v", err)
	}
	if rec.Session.Title != "" {
		t.Errorf("Title = %q, want empty for a whitespace-only first message", rec.Session.Title)
	}
	if rec.Session.TitleDerived {
		t.Errorf("TitleDerived = true, want false when no title was derived")
	}
}
