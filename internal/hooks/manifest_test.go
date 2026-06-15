package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestLoadManifestParsesEntries(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "session-init.cjs"
runtime = "node"
event = "SessionStart"
matcher = "startup|resume|clear|compact"
args = []

[[hook]]
file = "agent-notify.py"
runtime = "python3"
event = "Stop"
args = ["claude", "stop"]

[[hook]]
file = "lib/config.cjs"
lib = true
`)

	hooks, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(hooks) != 3 {
		t.Fatalf("got %d hooks, want 3", len(hooks))
	}

	node := hooks[0]
	if node.File != "session-init.cjs" || node.Runtime != "node" || node.Event != "SessionStart" {
		t.Errorf("node hook parsed wrong: %+v", node)
	}
	if node.Matcher != "startup|resume|clear|compact" {
		t.Errorf("matcher parsed wrong: %q", node.Matcher)
	}

	py := hooks[1]
	if py.Runtime != "python3" || py.Event != "Stop" {
		t.Errorf("python hook parsed wrong: %+v", py)
	}
	if len(py.Args) != 2 || py.Args[0] != "claude" || py.Args[1] != "stop" {
		t.Errorf("python args parsed wrong: %v", py.Args)
	}

	lib := hooks[2]
	if !lib.Lib || lib.File != "lib/config.cjs" {
		t.Errorf("lib hook parsed wrong: %+v", lib)
	}
}

func TestLoadManifestMissingFile(t *testing.T) {
	_, err := LoadManifest(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil {
		t.Fatal("expected error for missing manifest, got nil")
	}
}

func TestLoadManifestRejectsBadRuntime(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "x.sh"
runtime = "bash"
event = "Stop"
`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected error for invalid runtime, got nil")
	}
}

func TestLoadManifestRejectsMissingEvent(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "x.cjs"
runtime = "node"
`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected error for missing event on non-lib hook, got nil")
	}
}

func TestLoadManifestRejectsEmptyFile(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
runtime = "node"
event = "Stop"
`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected error for empty file field, got nil")
	}
}

func TestLoadManifestRejectsPathTraversal(t *testing.T) {
	for _, bad := range []string{"../../etc/evil", "/etc/evil", "a/../../b"} {
		path := writeManifest(t, "[[hook]]\nfile = \""+bad+"\"\nruntime = \"node\"\nevent = \"Stop\"\n")
		if _, err := LoadManifest(path); err == nil {
			t.Errorf("expected error for unsafe path %q, got nil", bad)
		}
	}
}

func TestLoadManifestRejectsUnknownField(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "x.cjs"
runtimes = "node"
event = "Stop"
`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected error for unknown field (typo), got nil")
	}
}

func TestLoadManifestAcceptsCodexNotifyEvent(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "agent-notify.py"
runtime = "python3"
event = "codex.notify"
args = ["codex"]
`)
	hooks, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(hooks) != 1 || hooks[0].Event != "codex.notify" {
		t.Errorf("codex.notify hook parsed wrong: %+v", hooks)
	}
}

func TestLoadManifestRejectsUnknownEvent(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "x.cjs"
runtime = "node"
event = "Stahp"
`)
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected error for unknown event, got nil")
	}
}

func TestFilesDeduplicates(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "notify.py"
runtime = "python3"
event = "Stop"

[[hook]]
file = "notify.py"
runtime = "python3"
event = "Notification"
`)
	hooks, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if files := Files(hooks); len(files) != 1 || files[0] != "notify.py" {
		t.Errorf("Files = %v, want [notify.py] (deduplicated)", files)
	}
}

func TestFilesReturnsAll(t *testing.T) {
	path := writeManifest(t, `
[[hook]]
file = "a.cjs"
runtime = "node"
event = "Stop"

[[hook]]
file = "lib/b.cjs"
lib = true
`)
	hooks, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	files := Files(hooks)
	if len(files) != 2 || files[0] != "a.cjs" || files[1] != "lib/b.cjs" {
		t.Errorf("Files returned %v, want [a.cjs lib/b.cjs]", files)
	}
}
