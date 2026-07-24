package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanducng/vd-cli/v2/internal/config"
	"github.com/vanducng/vd-cli/v2/internal/target"
)

func TestPi_UserScopeUsesAgentSkills(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeSkill(t, repo, "foo")

	results, err := Pi(repo, PiOptions{Skills: []string{"foo"}, DryRun: true})
	if err != nil {
		t.Fatalf("Pi: %v", err)
	}
	want := filepath.Join(home, ".pi", "agent", "skills", "foo")
	if len(results) != 1 || results[0].Dest != want || results[0].Action != "would symlink" {
		t.Fatalf("results = %#v, want dry-run dest %s", results, want)
	}
}

func TestPi_RepoScopeInstallsSelectedSkill(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	writeSkill(t, repo, "bar")

	results, err := Pi(repo, PiOptions{Scope: "repo", Skills: []string{"foo"}})
	if err != nil {
		t.Fatalf("Pi: %v", err)
	}
	if len(results) != 1 || results[0].Name != "foo" {
		t.Fatalf("results = %#v, want only foo", results)
	}
	if _, err := os.Stat(filepath.Join(repo, ".pi", "skills", "foo", "SKILL.md")); err != nil {
		t.Fatalf("installed skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".pi", "skills", "bar")); !os.IsNotExist(err) {
		t.Fatalf("bar should not be installed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".pi", ".vd-managed-skills", "foo")); err != nil {
		t.Fatalf("ownership marker missing: %v", err)
	}
}

func TestPi_RepoScopeRemainsBuildCompatible(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	if _, err := Pi(repo, PiOptions{Scope: "repo", Skills: []string{"foo"}}); err != nil {
		t.Fatalf("Pi: %v", err)
	}
	emitter, err := target.NewEmitter("pi")
	if err != nil {
		t.Fatalf("NewEmitter: %v", err)
	}
	ctx := target.Context{RepoRoot: repo, Manifest: &config.Manifest{}, Lock: &config.Lockfile{}}
	if err := emitter.Emit(ctx); err != nil {
		t.Fatalf("build after install: %v", err)
	}
}

func TestPi_ExplicitRepoDestinationRemainsBuildCompatible(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	dest := filepath.Join(repo, ".pi", "skills")
	if _, err := Pi(repo, PiOptions{Scope: "repo", Dest: dest, Skills: []string{"foo"}}); err != nil {
		t.Fatalf("Pi: %v", err)
	}
	emitter, err := target.NewEmitter("pi")
	if err != nil {
		t.Fatalf("NewEmitter: %v", err)
	}
	ctx := target.Context{RepoRoot: repo, Manifest: &config.Manifest{}, Lock: &config.Lockfile{}}
	if err := emitter.Emit(ctx); err != nil {
		t.Fatalf("build after install: %v", err)
	}
}

func TestPi_RepoScopeRollsBackOwnershipOnCollision(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	dst := filepath.Join(repo, ".pi", "skills", "foo")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Pi(repo, PiOptions{Scope: "repo", Skills: []string{"foo"}}); err == nil {
		t.Fatal("expected destination collision")
	}
	marker := filepath.Join(repo, ".pi", ".vd-managed-skills", "foo")
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("ownership marker remains after failed install: %v", err)
	}
}

func TestPi_CopyAndForceReplaceExistingDestination(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "foo")
	existing := filepath.Join(dest, "foo")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Pi(repo, PiOptions{Dest: dest, Skills: []string{"foo"}, Copy: true}); err == nil {
		t.Fatal("expected existing destination error")
	}
	results, err := Pi(repo, PiOptions{Dest: dest, Skills: []string{"foo"}, Copy: true, Force: true})
	if err != nil {
		t.Fatalf("Pi force: %v", err)
	}
	if len(results) != 1 || results[0].Action != "copied" {
		t.Fatalf("results = %#v, want copied", results)
	}
}

func TestPi_RejectsInvalidScopeWithDestOverride(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")

	_, err := Pi(repo, PiOptions{
		Scope:  "project",
		Dest:   filepath.Join(t.TempDir(), "skills"),
		DryRun: true,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid pi scope") {
		t.Fatalf("error = %v, want invalid pi scope", err)
	}
}
