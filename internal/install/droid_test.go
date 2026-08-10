package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/config"
	"github.com/vanducng/vd-cli/v2/internal/target"
)

func TestDroid_UserScopeUsesFactorySkills(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeSkill(t, repo, "foo")

	results, err := Droid(repo, DroidOptions{Skills: []string{"foo"}, DryRun: true})
	if err != nil {
		t.Fatalf("Droid: %v", err)
	}
	want := filepath.Join(home, ".factory", "skills", "foo")
	if len(results) != 1 || results[0].Dest != want || results[0].Action != "would symlink" {
		t.Fatalf("results = %#v, want dry-run dest %s", results, want)
	}
}

func TestDroid_RepoScopeInstallsSelectedSkill(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	writeSkill(t, repo, "bar")

	results, err := Droid(repo, DroidOptions{Scope: "repo", Skills: []string{"foo"}})
	if err != nil {
		t.Fatalf("Droid: %v", err)
	}
	if len(results) != 1 || results[0].Name != "foo" {
		t.Fatalf("results = %#v, want only foo", results)
	}
	if _, err := os.Stat(filepath.Join(repo, ".factory", "skills", "foo", "SKILL.md")); err != nil {
		t.Fatalf("installed skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".factory", "skills", "bar")); !os.IsNotExist(err) {
		t.Fatalf("bar should not be installed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".factory", ".vd-managed-skills", "foo")); err != nil {
		t.Fatalf("ownership marker missing: %v", err)
	}
}

func TestDroid_RepoScopeRemainsBuildCompatible(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	if _, err := Droid(repo, DroidOptions{Scope: "repo", Skills: []string{"foo"}}); err != nil {
		t.Fatalf("Droid: %v", err)
	}
	emitter, err := target.NewEmitter("droid")
	if err != nil {
		t.Fatalf("NewEmitter: %v", err)
	}
	ctx := target.Context{RepoRoot: repo, Manifest: &config.Manifest{}, Lock: &config.Lockfile{}}
	if err := emitter.Emit(ctx); err != nil {
		t.Fatalf("build after install: %v", err)
	}
}

func TestDroid_ExplicitRepoDestinationRemainsBuildCompatible(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	dest := filepath.Join(repo, ".factory", "skills")
	if _, err := Droid(repo, DroidOptions{Scope: "repo", Dest: dest, Skills: []string{"foo"}}); err != nil {
		t.Fatalf("Droid: %v", err)
	}
	emitter, err := target.NewEmitter("droid")
	if err != nil {
		t.Fatalf("NewEmitter: %v", err)
	}
	ctx := target.Context{RepoRoot: repo, Manifest: &config.Manifest{}, Lock: &config.Lockfile{}}
	if err := emitter.Emit(ctx); err != nil {
		t.Fatalf("build after install: %v", err)
	}
}

func TestDroid_RepoScopeRollsBackOwnershipOnCollision(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	dst := filepath.Join(repo, ".factory", "skills", "foo")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Droid(repo, DroidOptions{Scope: "repo", Skills: []string{"foo"}}); err == nil {
		t.Fatal("expected destination collision")
	}
	marker := filepath.Join(repo, ".factory", ".vd-managed-skills", "foo")
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("ownership marker remains after failed install: %v", err)
	}
}

func TestDroid_CopyAndForceReplaceExistingDestination(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "foo")
	existing := filepath.Join(dest, "foo")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Droid(repo, DroidOptions{Dest: dest, Skills: []string{"foo"}, Copy: true}); err == nil {
		t.Fatal("expected existing destination error")
	}
	results, err := Droid(repo, DroidOptions{Dest: dest, Skills: []string{"foo"}, Copy: true, Force: true})
	if err != nil {
		t.Fatalf("Droid force: %v", err)
	}
	if len(results) != 1 || results[0].Action != "copied" {
		t.Fatalf("results = %#v, want copied", results)
	}
}

func TestDroid_RejectsInvalidScopeWithDestOverride(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")

	_, err := Droid(repo, DroidOptions{
		Scope:  "project",
		Dest:   filepath.Join(t.TempDir(), "skills"),
		DryRun: true,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid droid scope") {
		t.Fatalf("error = %v, want invalid droid scope", err)
	}
}
