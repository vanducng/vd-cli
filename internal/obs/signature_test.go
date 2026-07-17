package obs

import (
	"strings"
	"testing"
)

// Two consecutive runs over the same cache must produce identical signatures
// — the goal's success criterion for the cluster's cross-run identity.
func TestNormalizeSignatureIsStableAcrossRuns(t *testing.T) {
	cases := []string{
		`open /Users/dev/project/internal/foo/bar.go: no such file or directory`,
		`session 019abc11-2222-3333-4444-555566667777 not found`,
		`exit status 127: command not found: "some-tool"`,
		`tool_span sp_9f8e7d6c5b4a failed after 12345678ms`,
		``,
	}
	for _, in := range cases {
		want := normalizeSignature(in)
		for i := 0; i < 5; i++ {
			if got := normalizeSignature(in); got != want {
				t.Fatalf("normalizeSignature(%q) not stable: first=%q run%d=%q", in, want, i, got)
			}
		}
	}
}

func TestNormalizeSignatureStripsVolatileDetail(t *testing.T) {
	got := normalizeSignature(`open /Users/dev/project/internal/foo/bar.go: no such file or directory`)
	if strings.Contains(got, "/Users/dev/project") {
		t.Fatalf("path not stripped: %q", got)
	}

	got = normalizeSignature(`session 019abc11-2222-3333-4444-555566667777 not found`)
	if strings.Contains(got, "019abc11-2222-3333-4444-555566667777") {
		t.Fatalf("uuid not stripped: %q", got)
	}

	got = normalizeSignature(`command not found: "some-tool"`)
	if strings.Contains(got, "some-tool") {
		t.Fatalf("quoted string not stripped: %q", got)
	}

	got = normalizeSignature(`tool_span sp_9f8e7d6c5b4a failed after 12345678ms`)
	if strings.Contains(got, "9f8e7d6c5b4a") || strings.Contains(got, "12345678") {
		t.Fatalf("hex id / long digit run not stripped: %q", got)
	}
}

// Same-shaped errors that differ only in volatile detail (a different path,
// a different id) must collapse to the same signature: that is what makes a
// signature useful as a clustering key at all.
func TestNormalizeSignatureCollapsesVolatileDifferences(t *testing.T) {
	a := normalizeSignature(`open /Users/dev/project-a/file.go: no such file or directory`)
	b := normalizeSignature(`open /Users/dev/project-b/other/deep/path.go: no such file or directory`)
	if a != b {
		t.Fatalf("same-shaped errors with different paths produced different signatures:\n  a=%q\n  b=%q", a, b)
	}

	a = normalizeSignature(`session 019abc11-2222-3333-4444-555566667777 not found`)
	b = normalizeSignature(`session abc12345-6789-abcd-ef01-234567890abc not found`)
	if a != b {
		t.Fatalf("same-shaped errors with different uuids produced different signatures:\n  a=%q\n  b=%q", a, b)
	}
}
