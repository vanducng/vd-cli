package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/config"
)

func droidContext(root string) Context {
	return Context{Manifest: &config.Manifest{}, Lock: &config.Lockfile{}, RepoRoot: root}
}

func TestDroidEmitterCreatesManagedSymlinks(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	makeTestSkillDir(t, tmp, "beta")
	ctx := droidContext(tmp)
	for range 2 {
		if err := emitDroid(ctx, false); err != nil {
			t.Fatalf("emitDroid: %v", err)
		}
	}
	for _, name := range []string{"alpha", "beta"} {
		got, err := os.Readlink(filepath.Join(tmp, ".factory", "skills", name))
		if err != nil {
			t.Fatalf("readlink %s: %v", name, err)
		}
		want := filepath.Join("..", "..", "skills", name)
		if got != want {
			t.Errorf("link %s = %q, want %q", name, got, want)
		}
		if _, err := os.Stat(filepath.Join(tmp, ".factory", droidManagedDir, name)); err != nil {
			t.Errorf("ownership marker %s missing: %v", name, err)
		}
	}
}

func TestDroidEmitterPrunesOnlyManagedEntries(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "stale")
	ctx := droidContext(tmp)
	if err := emitDroid(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(tmp, "skills", "stale")); err != nil {
		t.Fatal(err)
	}
	manual := filepath.Join(tmp, ".factory", "skills", "manual")
	if err := os.MkdirAll(manual, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manual, "keep"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(tmp, ".factory", "skills", "foreign")
	if err := os.Symlink("../elsewhere", foreign); err != nil {
		t.Fatal(err)
	}
	if err := emitDroid(ctx, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tmp, ".factory", "skills", "stale")); !os.IsNotExist(err) {
		t.Fatalf("stale managed entry remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manual, "keep")); err != nil {
		t.Fatalf("manual entry changed: %v", err)
	}
	if got, err := os.Readlink(foreign); err != nil || got != "../elsewhere" {
		t.Fatalf("foreign link changed: %q, %v", got, err)
	}
}

func TestDroidEmitterPreflightsAllCollisions(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	makeTestSkillDir(t, tmp, "beta")
	keep := filepath.Join(tmp, ".factory", "skills", "beta", "keep")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := emitDroid(droidContext(tmp), false); err == nil {
		t.Fatal("expected unmanaged collision error")
	}
	if _, err := os.Lstat(filepath.Join(tmp, ".factory", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("alpha was created before later collision: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Fatalf("collision was modified: %q, %v", data, err)
	}
}

func TestDroidEmitterRefreshesManagedCopies(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	source := filepath.Join(tmp, "skills", "alpha", "value")
	if err := os.WriteFile(source, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := droidContext(tmp)
	if err := emitDroid(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := emitDroid(ctx, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, ".factory", "skills", "alpha", "value"))
	if err != nil || string(got) != "two" {
		t.Fatalf("managed copy = %q, %v", got, err)
	}
}

func TestReplaceDroidCopyPreservesDestinationOnStageFailure(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "skills", "alpha")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dst, "keep")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceDroidCopy(tmp, filepath.Join(tmp, "missing"), dst); err == nil {
		t.Fatal("expected staging failure")
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "keep" {
		t.Fatalf("destination changed after stage failure: %q, %v", data, err)
	}
}

func TestReplaceDroidCopyPreservesBackupWhenRestoreFails(t *testing.T) {
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

	originalRename := renameDroidPath
	t.Cleanup(func() { renameDroidPath = originalRename })
	calls := 0
	renameDroidPath = func(oldPath, newPath string) error {
		calls++
		if calls > 1 {
			return os.ErrPermission
		}
		return os.Rename(oldPath, newPath)
	}

	err := replaceDroidCopy(tmp, src, dst)
	if err == nil {
		t.Fatal("expected replace and restore failure")
	}
	matches, globErr := filepath.Glob(filepath.Join(tmp, ".vd-droid-stage-*", "old", "keep"))
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

func TestDroidEmitterRejectsSymlinkedFactoryPath(t *testing.T) {
	tmp := t.TempDir()
	makeTestSkillDir(t, tmp, "alpha")
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(tmp, ".factory")); err != nil {
		t.Fatal(err)
	}
	if err := emitDroid(droidContext(tmp), false); err == nil {
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

func TestDroidEmitterRejectsInvalidOwnershipMarker(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, ".factory", droidManagedDir, "alpha")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := emitDroid(droidContext(tmp), false); err == nil {
		t.Fatal("expected invalid ownership marker error")
	}
}
