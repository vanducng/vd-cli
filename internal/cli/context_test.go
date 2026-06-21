package cli

import "testing"

func TestExtractAdditionalContext(t *testing.T) {
	raw := []byte(`{"hookSpecificOutput":{"additionalContext":"## Paths\nReports: /tmp/reports"}}`)
	got, err := extractAdditionalContext(raw)
	if err != nil {
		t.Fatalf("extractAdditionalContext: %v", err)
	}
	want := "## Paths\nReports: /tmp/reports"
	if got != want {
		t.Fatalf("context = %q, want %q", got, want)
	}
}

func TestExtractAdditionalContextRejectsMissingContext(t *testing.T) {
	if _, err := extractAdditionalContext([]byte(`{"hookSpecificOutput":{}}`)); err == nil {
		t.Fatal("expected missing additionalContext error")
	}
}
