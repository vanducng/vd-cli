package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeDev_SymlinksRequestedSkill(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "foo")
	writeSkill(t, repo, "bar")

	results, err := ClaudeDev(repo, ClaudeDevOptions{
		Dest:   dest,
		Skills: []string{"foo"},
	})
	if err != nil {
		t.Fatalf("ClaudeDev: %v", err)
	}
	if len(results) != 1 || results[0].Name != "foo" {
		t.Fatalf("results = %#v, want only foo", results)
	}
	if _, err := os.Stat(filepath.Join(dest, "foo", "SKILL.md")); err != nil {
		t.Fatalf("installed SKILL.md missing: %v", err)
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Readlink(filepath.Join(dest, "foo")); err != nil {
			t.Fatalf("expected symlink install: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "bar")); !os.IsNotExist(err) {
		t.Fatalf("bar should not be installed, stat err = %v", err)
	}
}

func TestClaudeDev_ForceReplacesStaleStandalone(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "worktree")

	stale := filepath.Join(dest, "worktree")
	if err := os.MkdirAll(filepath.Join(stale, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "SKILL.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ClaudeDev(repo, ClaudeDevOptions{Dest: dest, Skills: []string{"worktree"}}); err == nil {
		t.Fatal("expected existing destination error without --force")
	}

	if _, err := ClaudeDev(repo, ClaudeDevOptions{Dest: dest, Skills: []string{"worktree"}, Force: true}); err != nil {
		t.Fatalf("ClaudeDev force: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(stale, "SKILL.md"))
	if err != nil {
		t.Fatalf("post-force read: %v", err)
	}
	if string(got) == "stale" {
		t.Fatal("force did not replace the stale standalone")
	}
}

func TestClaudeDev_DryRunReportsActions(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "foo")

	results, err := ClaudeDev(repo, ClaudeDevOptions{Dest: dest, Skills: []string{"foo"}, DryRun: true})
	if err != nil {
		t.Fatalf("ClaudeDev: %v", err)
	}
	if len(results) != 1 || results[0].Action != "would symlink" {
		t.Fatalf("results = %#v", results)
	}
	if _, err := os.Stat(filepath.Join(dest, "foo")); !os.IsNotExist(err) {
		t.Fatalf("dry run should not create dest, stat err = %v", err)
	}
}

func TestClaudeDev_NoSkillsErrors(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := ClaudeDev(repo, ClaudeDevOptions{Dest: filepath.Join(t.TempDir(), "skills")})
	if err == nil {
		t.Fatal("expected error for empty skills dir")
	}
}
