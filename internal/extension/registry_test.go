package extension

import (
	"os"
	"path/filepath"
	"testing"
)

func writeExt(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "extensions", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

const validBody = `name = "%s"
transport = "stdio"
command = "uv"
targets = ["codex"]
enabled = true`

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "bravo", "name = \"bravo\"\ntransport = \"stdio\"\ncommand = \"uv\"\ntargets = [\"codex\"]")
	writeExt(t, root, "alpha", "name = \"alpha\"\ntransport = \"stdio\"\ncommand = \"uv\"\ntargets = [\"codex\"]")
	// a dir without a manifest is skipped
	if err := os.MkdirAll(filepath.Join(root, "extensions", "nope"), 0o755); err != nil {
		t.Fatal(err)
	}

	exts, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(exts) != 2 {
		t.Fatalf("got %d extensions, want 2", len(exts))
	}
	if exts[0].Name != "alpha" || exts[1].Name != "bravo" {
		t.Fatalf("not sorted: %s, %s", exts[0].Name, exts[1].Name)
	}
	if exts[0].Dir == "" {
		t.Fatalf("Dir not set")
	}
}

func TestDiscoverNoDir(t *testing.T) {
	exts, err := Discover(t.TempDir())
	if err != nil {
		t.Fatalf("Discover on missing dir should not error: %v", err)
	}
	if exts != nil {
		t.Fatalf("want nil, got %v", exts)
	}
}

func TestDiscoverDuplicateName(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "one", "name = \"dup\"\ntransport = \"stdio\"\ncommand = \"uv\"\ntargets = [\"codex\"]")
	writeExt(t, root, "two", "name = \"dup\"\ntransport = \"stdio\"\ncommand = \"uv\"\ntargets = [\"codex\"]")
	if _, err := Discover(root); err == nil {
		t.Fatalf("want duplicate-name error")
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "one", "name = \"one\"\ntransport = \"stdio\"\ncommand = \"uv\"\ntargets = [\"codex\"]")
	if _, err := Find(root, "one"); err != nil {
		t.Fatalf("Find: %v", err)
	}
	if _, err := Find(root, "missing"); err == nil {
		t.Fatalf("want not-found error")
	}
}
