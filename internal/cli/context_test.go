package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindContextHookInPrefersPython(t *testing.T) {
	writeHook := func(t *testing.T, dir, name string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/x\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	cases := []struct {
		name        string
		files       []string
		wantName    string
		wantRuntime string
		wantOK      bool
	}{
		{
			name:        "only python present runs via python3",
			files:       []string{contextHookFilePy},
			wantName:    contextHookFilePy,
			wantRuntime: "python3",
			wantOK:      true,
		},
		{
			name:        "only cjs present runs via node",
			files:       []string{contextHookFileCjs},
			wantName:    contextHookFileCjs,
			wantRuntime: "node",
			wantOK:      true,
		},
		{
			name:        "both present python wins",
			files:       []string{contextHookFileCjs, contextHookFilePy},
			wantName:    contextHookFilePy,
			wantRuntime: "python3",
			wantOK:      true,
		},
		{
			name:   "neither present not found",
			files:  nil,
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range c.files {
				writeHook(t, dir, f)
			}
			path, runtime, ok := findContextHookIn([]string{dir})
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if want := filepath.Join(dir, c.wantName); path != want {
				t.Errorf("path = %q, want %q", path, want)
			}
			if runtime != c.wantRuntime {
				t.Errorf("runtime = %q, want %q", runtime, c.wantRuntime)
			}
		})
	}
}

func TestFindContextHookInDirPrecedence(t *testing.T) {
	// An earlier dir's .cjs beats a later dir's .py — directory order wins over
	// per-dir python preference.
	codex := t.TempDir()
	claude := t.TempDir()
	if err := os.WriteFile(filepath.Join(codex, contextHookFileCjs), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claude, contextHookFilePy), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, runtime, ok := findContextHookIn([]string{codex, claude})
	if !ok {
		t.Fatal("expected a hook to be found")
	}
	if want := filepath.Join(codex, contextHookFileCjs); path != want {
		t.Errorf("path = %q, want %q (first dir wins)", path, want)
	}
	if runtime != "node" {
		t.Errorf("runtime = %q, want node", runtime)
	}
}

func TestContextHookRuntime(t *testing.T) {
	if got := contextHookRuntime("/x/dev-rules-reminder.py"); got != "python3" {
		t.Errorf(".py runtime = %q, want python3", got)
	}
	if got := contextHookRuntime("/x/dev-rules-reminder.cjs"); got != "node" {
		t.Errorf(".cjs runtime = %q, want node", got)
	}
}

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
