package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/config"
)

func piContext(root string) Context {
	return Context{Manifest: &config.Manifest{}, Lock: &config.Lockfile{}, RepoRoot: root}
}

func TestPiEmitterCreatesManagedSymlinks(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	makeTestSkillDir(t, tmp, "beta")
	ctx := piContext(tmp)
	for range 2 {
		if err := emitPi(ctx, false); err != nil {
			t.Fatalf("emitPi: %v", err)
		}
	}
	for _, name := range []string{"alpha", "beta"} {
		got, err := os.Readlink(filepath.Join(tmp, ".pi", "skills", name))
		if err != nil {
			t.Fatalf("readlink %s: %v", name, err)
		}
		want := filepath.Join("..", "..", "skills", name)
		if got != want {
			t.Errorf("link %s = %q, want %q", name, got, want)
		}
		if _, err := os.Stat(filepath.Join(tmp, ".pi", piManagedDir, name)); err != nil {
			t.Errorf("ownership marker %s missing: %v", name, err)
		}
	}
}

func TestPiEmitterPrunesOnlyManagedEntries(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "stale")
	ctx := piContext(tmp)
	if err := emitPi(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(tmp, "skills", "stale")); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(tmp, ".pi", "skills", "manual")
	if err := os.MkdirAll(manual, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manual, "keep"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(tmp, ".pi", "skills", "foreign")
	if err := os.Symlink("../elsewhere", foreign); err != nil {
		t.Fatal(err)
	}
	if err := emitPi(ctx, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tmp, ".pi", "skills", "stale")); !os.IsNotExist(err) {
		t.Fatalf("stale managed entry remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manual, "keep")); err != nil {
		t.Fatalf("manual entry changed: %v", err)
	}
	if got, err := os.Readlink(foreign); err != nil || got != "../elsewhere" {
		t.Fatalf("foreign link changed: %q, %v", got, err)
	}
}

func TestPiEmitterPreflightsAllCollisions(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	makeTestSkillDir(t, tmp, "beta")
	keep := filepath.Join(tmp, ".pi", "skills", "beta", "keep")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := emitPi(piContext(tmp), false); err == nil {
		t.Fatal("expected unmanaged collision error")
	}
	if _, err := os.Lstat(filepath.Join(tmp, ".pi", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("alpha was created before later collision: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Fatalf("collision was modified: %q, %v", data, err)
	}
}

func TestPiEmitterRefreshesManagedCopies(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	source := filepath.Join(tmp, "skills", "alpha", "value")
	if err := os.WriteFile(source, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := piContext(tmp)
	if err := emitPi(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := emitPi(ctx, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, ".pi", "skills", "alpha", "value"))
	if err != nil || string(got) != "two" {
		t.Fatalf("managed copy = %q, %v", got, err)
	}
}

func TestReplacePiCopyPreservesDestinationOnStageFailure(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "skills", "alpha")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dst, "keep")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replacePiCopy(tmp, filepath.Join(tmp, "missing"), dst); err == nil {
		t.Fatal("expected staging failure")
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Fatalf("destination changed after stage failure: %q, %v", data, err)
	}
}

func TestReplacePiCopyPreservesBackupWhenRestoreFails(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	dst := filepath.Join(tmp, "skills", "alpha")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "new"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "keep"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalRename := renamePiPath
	t.Cleanup(func() { renamePiPath = originalRename })
	calls := 0
	renamePiPath = func(oldPath, newPath string) error {
		calls++
		if calls > 1 {
			return os.ErrPermission
		}
		return os.Rename(oldPath, newPath)
	}

	err := replacePiCopy(tmp, src, dst)
	if err == nil {
		t.Fatal("expected replace and restore failure")
	}
	matches, globErr := filepath.Glob(filepath.Join(tmp, ".vd-pi-stage-*", "old", "keep"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("preserved backup matches = %v, want one", matches)
	}
	if data, readErr := os.ReadFile(matches[0]); readErr != nil || string(data) != "keep" {
		t.Fatalf("preserved backup = %q, %v", data, readErr)
	}
}

func TestPiEmitterRejectsSymlinkedConfigPath(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(tmp, ".pi")); err != nil {
		t.Fatal(err)
	}
	if err := emitPi(piContext(tmp), false); err == nil {
		t.Fatal("expected symlinked destination rejection")
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("external destination was modified: %v", entries)
	}
}

func TestPiEmitterRejectsInvalidOwnershipMarker(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, ".pi", piManagedDir, "alpha")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := emitPi(piContext(tmp), false); err == nil {
		t.Fatal("expected invalid ownership marker error")
	}
}

func TestUnclaimPiSkillRejectsPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "../sentinel", `a\b`} {
		if err := UnclaimPiSkill(tmp, name); err == nil {
			t.Fatalf("expected rejection for name %q", name)
		}
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("guard let os.Remove escape the managed dir: %v", err)
	}
}
