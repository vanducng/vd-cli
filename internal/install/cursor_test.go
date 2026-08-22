package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCursor_UserScopeUsesCursorSkills(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VD_CURSOR_HOME", "")
	writeSkill(t, repo, "foo")

	results, err := Cursor(repo, CursorOptions{Skills: []string{"foo"}, DryRun: true})
	if err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	want := filepath.Join(home, ".cursor", "skills", "foo")
	if len(results) != 1 || results[0].Dest != want || results[0].Action != "would symlink" {
		t.Fatalf("results = %#v, want dry-run dest %s", results, want)
	}
}

func TestCursor_UserScopeHonorsVDCursorHome(t *testing.T) {
	repo := t.TempDir()
	cursorHome := t.TempDir()
	t.Setenv("VD_CURSOR_HOME", cursorHome)
	writeSkill(t, repo, "foo")

	results, err := Cursor(repo, CursorOptions{Skills: []string{"foo"}, DryRun: true})
	if err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	want := filepath.Join(cursorHome, "skills", "foo")
	if len(results) != 1 || results[0].Dest != want {
		t.Fatalf("results = %#v, want dest %s", results, want)
	}
}

func TestCursor_RepoScopeUsesDotCursorSkills(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")

	results, err := Cursor(repo, CursorOptions{Scope: "repo", DryRun: true})
	if err != nil {
		t.Fatalf("Cursor dry-run: %v", err)
	}
	want := filepath.Join(repo, ".cursor", "skills", "foo")
	if len(results) != 1 || results[0].Dest != want || results[0].Action != "would symlink" {
		t.Fatalf("results = %#v, want dry-run dest %s", results, want)
	}
}

func TestCursor_InstallsRequestedSkillToDest(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "foo")
	writeSkill(t, repo, "bar")

	results, err := Cursor(repo, CursorOptions{
		Dest:   dest,
		Skills: []string{"foo"},
	})
	if err != nil {
		t.Fatalf("Cursor: %v", err)
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

func TestCursor_RepoScopeInstallsSelectedSkill(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")
	writeSkill(t, repo, "bar")

	results, err := Cursor(repo, CursorOptions{Scope: "repo", Skills: []string{"foo"}})
	if err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if len(results) != 1 || results[0].Name != "foo" {
		t.Fatalf("results = %#v, want only foo", results)
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "foo", "SKILL.md")); err != nil {
		t.Fatalf("installed skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "bar")); !os.IsNotExist(err) {
		t.Fatalf("bar should not be installed, stat err = %v", err)
	}
}

func TestCursor_CopyAndForceReplaceExistingDestination(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(t.TempDir(), "skills")
	writeSkill(t, repo, "foo")
	existing := filepath.Join(dest, "foo")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Cursor(repo, CursorOptions{Dest: dest, Skills: []string{"foo"}, Copy: true}); err == nil {
		t.Fatal("expected existing destination error")
	}
	results, err := Cursor(repo, CursorOptions{Dest: dest, Skills: []string{"foo"}, Copy: true, Force: true})
	if err != nil {
		t.Fatalf("Cursor force: %v", err)
	}
	if len(results) != 1 || results[0].Action != "copied" {
		t.Fatalf("results = %#v, want copied", results)
	}
	if _, err := os.Stat(filepath.Join(existing, "SKILL.md")); err != nil {
		t.Fatalf("force install missing SKILL.md: %v", err)
	}
}

func TestCursor_RejectsInvalidScopeWithDestOverride(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")

	_, err := Cursor(repo, CursorOptions{
		Scope:  "project",
		Dest:   filepath.Join(t.TempDir(), "skills"),
		DryRun: true,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid cursor scope") {
		t.Fatalf("error = %v, want invalid cursor scope", err)
	}
}

func TestCursor_RejectsDangerousDestRoot(t *testing.T) {
	repo := t.TempDir()
	writeSkill(t, repo, "foo")

	_, err := Cursor(repo, CursorOptions{
		Dest:   "/etc/skills",
		Skills: []string{"foo"},
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected refusal for /etc dest")
	}
	if !strings.Contains(err.Error(), "system path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
