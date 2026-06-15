package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

// makeSrc creates a temp source dir populated with the given rel->content files.
func makeSrc(t *testing.T, files map[string]string) string {
	t.Helper()
	src := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return src
}

var sampleFiles = map[string]string{
	"session-init.cjs": "// session-init\n",
	"lib/config.cjs":   "// lib/config\n",
}

func sampleList() []string { return []string{"session-init.cjs", "lib/config.cjs"} }

func TestInstallFromLayout(t *testing.T) {
	src := makeSrc(t, sampleFiles)
	dest := t.TempDir()

	results, err := InstallFrom(src, dest, sampleList())
	if err != nil {
		t.Fatalf("InstallFrom: %v", err)
	}

	for _, rel := range sampleList() {
		dstPath := filepath.Join(dest, filepath.FromSlash(rel))
		info, err := os.Stat(dstPath)
		if err != nil {
			t.Errorf("installed file missing: %s: %v", dstPath, err)
			continue
		}
		if info.Mode().Perm()&0o755 != 0o755 {
			t.Errorf("%s: want at least 0755, got %04o", rel, info.Mode().Perm())
		}
	}

	if len(results) != len(sampleList()) {
		t.Errorf("got %d results, want %d", len(results), len(sampleList()))
	}
	for _, r := range results {
		if r.Action != "wrote" {
			t.Errorf("first install: expected 'wrote' for %s, got %q", r.RelPath, r.Action)
		}
	}
}

func TestInstallFromIdempotent(t *testing.T) {
	src := makeSrc(t, sampleFiles)
	dest := t.TempDir()

	if _, err := InstallFrom(src, dest, sampleList()); err != nil {
		t.Fatalf("first InstallFrom: %v", err)
	}
	second, err := InstallFrom(src, dest, sampleList())
	if err != nil {
		t.Fatalf("second InstallFrom: %v", err)
	}
	for _, r := range second {
		if r.Action != "unchanged" {
			t.Errorf("second install: expected 'unchanged' for %s, got %q", r.RelPath, r.Action)
		}
	}
}

func TestInstallFromBackupOnce(t *testing.T) {
	src := makeSrc(t, sampleFiles)
	dest := t.TempDir()

	// Pre-populate session-init.cjs with different content to trigger a backup.
	target := filepath.Join(dest, "session-init.cjs")
	if err := os.WriteFile(target, []byte("old content"), 0o755); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}

	results, err := InstallFrom(src, dest, sampleList())
	if err != nil {
		t.Fatalf("InstallFrom: %v", err)
	}

	var got string
	for _, r := range results {
		if r.RelPath == "session-init.cjs" {
			got = r.Action
		}
	}
	if got != "backed-up+wrote" {
		t.Errorf("action: got %q, want 'backed-up+wrote'", got)
	}

	entries, _ := filepath.Glob(filepath.Join(dest, "session-init.bak.*"))
	if len(entries) == 0 {
		t.Error("expected a .bak.* backup file, found none")
	}

	// Second run: content now matches, no new backup.
	if _, err := InstallFrom(src, dest, sampleList()); err != nil {
		t.Fatalf("second InstallFrom: %v", err)
	}
	entries2, _ := filepath.Glob(filepath.Join(dest, "session-init.bak.*"))
	if len(entries2) != len(entries) {
		t.Errorf("backup count grew on second run: %d -> %d", len(entries), len(entries2))
	}
}

func TestInstallFromNoUnknownFilesRemoved(t *testing.T) {
	src := makeSrc(t, sampleFiles)
	dest := t.TempDir()
	stranger := filepath.Join(dest, "my-custom-hook.cjs")
	if err := os.WriteFile(stranger, []byte("custom"), 0o644); err != nil {
		t.Fatalf("write stranger: %v", err)
	}

	if _, err := InstallFrom(src, dest, sampleList()); err != nil {
		t.Fatalf("InstallFrom: %v", err)
	}

	if _, err := os.Stat(stranger); err != nil {
		t.Errorf("InstallFrom removed unknown file %s: %v", stranger, err)
	}
}

func TestInstallFromMissingSourceErrors(t *testing.T) {
	src := t.TempDir() // empty — no hook files
	dest := t.TempDir()
	if _, err := InstallFrom(src, dest, []string{"session-init.cjs"}); err == nil {
		t.Fatal("expected error for missing source file, got nil")
	}
}
