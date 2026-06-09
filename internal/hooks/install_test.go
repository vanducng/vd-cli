package hooks

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// expectedCJS lists every .cjs file that must be embedded (rel to assets/).
var expectedCJS = []string{
	"session-init.cjs",
	"subagent-init.cjs",
	"dev-rules-reminder.cjs",
	"statusline.cjs",
	"scout-block.cjs",
	"team-context-inject.cjs",
	"task-completed-handler.cjs",
	"teammate-idle-handler.cjs",
	filepath.Join("lib", "config.cjs"),
	filepath.Join("lib", "paths.cjs"),
	filepath.Join("lib", "state.cjs"),
}

func TestEmbedContainsAllCJS(t *testing.T) {
	for _, rel := range expectedCJS {
		path := "assets/" + filepath.ToSlash(rel)
		if _, err := FS.Open(path); err != nil {
			t.Errorf("embed FS missing %s: %v", path, err)
		}
	}
}

func TestInstallLayout(t *testing.T) {
	dest := t.TempDir()
	results, err := Install(dest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// All expected files must be present at the correct paths.
	for _, rel := range expectedCJS {
		dstPath := filepath.Join(dest, rel)
		info, err := os.Stat(dstPath)
		if err != nil {
			t.Errorf("installed file missing: %s: %v", dstPath, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected file, got dir: %s", dstPath)
		}
		// Check 0755 perm bits (owner rwx, group rx, other rx).
		got := info.Mode().Perm()
		if got&0o755 != 0o755 {
			t.Errorf("%s: want at least 0755, got %04o", rel, got)
		}
	}

	// Verify result slice covers all expected files (no extras required, but
	// none of our managed files should be missing from the report).
	reported := make(map[string]string, len(results))
	for _, r := range results {
		reported[filepath.ToSlash(r.RelPath)] = r.Action
	}
	for _, rel := range expectedCJS {
		if _, ok := reported[filepath.ToSlash(rel)]; !ok {
			t.Errorf("Install result missing entry for %s", rel)
		}
	}
}

func TestInstallIdempotent(t *testing.T) {
	dest := t.TempDir()

	first, err := Install(dest)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	for _, r := range first {
		if r.Action != "wrote" {
			t.Errorf("first run: expected 'wrote' for %s, got %q", r.RelPath, r.Action)
		}
	}

	second, err := Install(dest)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	for _, r := range second {
		if r.Action != "unchanged" {
			t.Errorf("second run: expected 'unchanged' for %s, got %q", r.RelPath, r.Action)
		}
	}

	// No duplicate files — count equals expected.
	if len(second) != len(expectedCJS) {
		t.Errorf("idempotent run: got %d results, want %d", len(second), len(expectedCJS))
	}
}

func TestInstallBackupOnce(t *testing.T) {
	dest := t.TempDir()

	// Pre-populate session-init.cjs with different content to trigger backup.
	target := filepath.Join(dest, "session-init.cjs")
	if err := os.WriteFile(target, []byte("old content"), 0o755); err != nil {
		t.Fatalf("pre-populate: %v", err)
	}

	results, err := Install(dest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	var sessionResult *FileResult
	for i := range results {
		if results[i].RelPath == "session-init.cjs" {
			sessionResult = &results[i]
			break
		}
	}
	if sessionResult == nil {
		t.Fatal("no result for session-init.cjs")
	}
	if sessionResult.Action != "backed-up+wrote" {
		t.Errorf("action: got %q, want 'backed-up+wrote'", sessionResult.Action)
	}

	// Backup file must exist.
	entries, err := filepath.Glob(filepath.Join(dest, "session-init.bak.*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected a .bak.* backup file, found none")
	}

	// Second run on the now-matching content: no new backup.
	results2, err := Install(dest)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	for _, r := range results2 {
		if r.RelPath == "session-init.cjs" && r.Action != "unchanged" {
			t.Errorf("second run action: got %q, want 'unchanged'", r.Action)
		}
	}

	entries2, _ := filepath.Glob(filepath.Join(dest, "session-init.bak.*"))
	if len(entries2) != len(entries) {
		t.Errorf("backup count grew on second run: %d -> %d", len(entries), len(entries2))
	}
}

func TestInstallNoUnknownFilesRemoved(t *testing.T) {
	dest := t.TempDir()
	stranger := filepath.Join(dest, "my-custom-hook.cjs")
	if err := os.WriteFile(stranger, []byte("custom"), 0o644); err != nil {
		t.Fatalf("write stranger: %v", err)
	}

	if _, err := Install(dest); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Stranger must still be present.
	if _, err := os.Stat(stranger); err != nil {
		t.Errorf("Install removed unknown file %s: %v", stranger, err)
	}
}

func TestInstallSessionInitRunsViaNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}

	dest := t.TempDir()
	if _, err := Install(dest); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The hook writes VD_* vars to CLAUDE_ENV_FILE, not stdout.
	envFile := filepath.Join(t.TempDir(), "env.sh")

	hookPath := filepath.Join(dest, "session-init.cjs")
	cmd := exec.Command("node", hookPath)
	cmd.Env = append(os.Environ(), "CLAUDE_ENV_FILE="+envFile)
	// Run from dest so relative requires (./lib/...) resolve.
	cmd.Dir = dest
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session-init.cjs exited non-zero: %v\noutput:\n%s", err, out)
	}

	// Verify VD_* variables were written to the env file.
	envContent, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read CLAUDE_ENV_FILE %s: %v", envFile, err)
	}
	if !strings.Contains(string(envContent), "VD_") {
		t.Errorf("CLAUDE_ENV_FILE contains no VD_* lines:\n%s", envContent)
	}
}

func TestEmbedContainsNoCKFiles(t *testing.T) {
	// Guard: none of the embedded files should be ck-prefixed or from the
	// upstream ck skill tree.
	err := fs.WalkDir(FS, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "ck-") || strings.HasPrefix(base, "ck_") {
			t.Errorf("embedded file looks like a ck file: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embed FS: %v", err)
	}
}
