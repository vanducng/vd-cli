package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureClaude(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "skills", "alpha", "SKILL.md"),
		"---\nname: alpha\ndescription: alpha skill\n---\nbody\n")
	writeFile(t, filepath.Join(home, "skills", "beta", "SKILL.md.disabled"),
		"---\nname: beta\ndescription: beta skill\n---\nbody\n")
	writeFile(t, filepath.Join(home, "agents", "rev.md"),
		"---\nname: rev\ndescription: reviewer\n---\nx\n")
	writeFile(t, filepath.Join(home, "commands", "ship.md"), "do it\n")
	// no rules/ dir on purpose
	return home
}

func TestScan_SkillsEnabledDisabled(t *testing.T) {
	home := fixtureClaude(t)
	assets, err := NewClaudeAdapter(home).Scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	byName := map[string]Asset{}
	for _, a := range assets {
		byName[string(a.Type)+"/"+a.Name] = a
	}
	if a := byName["skill/alpha"]; !a.Enabled || a.Description != "alpha skill" {
		t.Errorf("alpha: %+v", a)
	}
	if a := byName["skill/beta"]; a.Enabled {
		t.Error("beta should be disabled")
	}
	if _, ok := byName["agent/rev"]; !ok {
		t.Error("agent rev not found")
	}
	if a := byName["command/ship"]; a.Name != "ship" {
		t.Errorf("command ship missing: %+v", a)
	}
}

func TestScan_MissingDirNotError(t *testing.T) {
	home := t.TempDir() // entirely empty
	assets, err := NewClaudeAdapter(home).Scan()
	if err != nil {
		t.Fatalf("scan empty: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("expected no assets, got %d", len(assets))
	}
}

func TestScan_ReadOnly(t *testing.T) {
	home := fixtureClaude(t)
	skill := filepath.Join(home, "skills", "alpha", "SKILL.md")
	before, err := os.Stat(skill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClaudeAdapter(home).Scan(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(skill)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("scan must not modify files")
	}
}

func TestScan_SymlinkedSkillDir(t *testing.T) {
	home := t.TempDir()
	real := t.TempDir()
	writeFile(t, filepath.Join(real, "SKILL.md"), "---\nname: linked\ndescription: d\n---\nx\n")
	skillsDir := filepath.Join(home, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(skillsDir, "linked")); err != nil {
		t.Fatal(err)
	}
	assets, err := NewAdapter(PlatformCodex, home).Scan()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, a := range assets {
		if a.Name == "linked" && a.Platform == PlatformCodex {
			found = true
		}
	}
	if !found {
		t.Error("symlinked skill dir not discovered (Codex installs are symlinks)")
	}
}

func TestReadHooks(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	writeFile(t, settings, `{
  "hooks": {
    "SessionStart": [
      {"matcher": "startup", "hooks": [{"type": "command", "command": "node \"$HOME/.claude/hooks/session-init.cjs\""}]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo custom"}]}
    ]
  }
}`)
	hooks, err := ReadHooks(settings)
	if err != nil {
		t.Fatalf("read hooks: %v", err)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
	var managed, custom bool
	for _, h := range hooks {
		m, _ := h.Frontmatter["managedByVd"].(bool)
		if h.Name == "SessionStart:startup" && m {
			managed = true
		}
		if h.Name == "PreToolUse:Bash" && !m {
			custom = true
		}
	}
	if !managed {
		t.Error("session-init hook should be flagged vd-managed")
	}
	if !custom {
		t.Error("custom echo hook should not be flagged vd-managed")
	}
}

func TestReadHooks_MissingFile(t *testing.T) {
	hooks, err := ReadHooks(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(hooks) != 0 {
		t.Errorf("expected empty, got %d", len(hooks))
	}
}
