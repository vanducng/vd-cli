package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
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

func fixtureService(t *testing.T) *inventory.Service {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "skills", "alpha", "SKILL.md"),
		"---\nname: alpha\ndescription: alpha skill\n---\nhello body\n")
	writeFile(t, filepath.Join(root, "skills.toml"),
		"[meta]\nversion = 1\n\n[skills.alpha]\nsource = \"up\"\nmode = \"tracked\"\n")

	claude := t.TempDir()
	writeFile(t, filepath.Join(claude, "skills", "discovered", "SKILL.md"),
		"---\nname: discovered\ndescription: a local skill\n---\nx\n")
	writeFile(t, filepath.Join(claude, "settings.json"),
		`{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"echo hi"}]}]}}`)
	svc := inventory.NewService(root, claude)
	svc.CodexHome, svc.CursorHome = t.TempDir(), t.TempDir() // hermetic
	return svc
}

func TestNewModel(t *testing.T) {
	m, err := newModel(fixtureService(t))
	if err != nil {
		t.Fatalf("newModel: %v", err)
	}
	if len(m.invRefs) < 2 {
		t.Errorf("expected managed + discovered rows, got %d", len(m.invRefs))
	}
	if m.invRefs[0].name != "alpha" {
		t.Errorf("first row = %q, want managed alpha", m.invRefs[0].name)
	}
}

func TestSwitchTab(t *testing.T) {
	m, _ := newModel(fixtureService(t))
	if m.tab != tabInventory {
		t.Fatalf("start tab = %v", m.tab)
	}
	m.switchTab(1)
	if m.tab != tabHooks {
		t.Errorf("after +1 = %v, want Hooks", m.tab)
	}
	m.switchTab(-1)
	if m.tab != tabInventory {
		t.Errorf("back = %v", m.tab)
	}
	m.switchTab(-1) // wrap
	if m.tab != tabDoctor {
		t.Errorf("wrap = %v, want Doctor", m.tab)
	}
}

func TestOpenSelectedSkill(t *testing.T) {
	m, _ := newModel(fixtureService(t))
	m.width, m.height = 100, 30
	m.resize()
	m.openSelected() // cursor at row 0 = alpha skill
	if m.detail == nil {
		t.Fatal("expected detail to open for a skill row")
	}
	if m.detail.Name != "alpha" {
		t.Errorf("detail = %q", m.detail.Name)
	}
}

func TestViewNoPanic(t *testing.T) {
	m, _ := newModel(fixtureService(t))
	m.width, m.height = 100, 30
	m.resize()
	if m.View() == "" {
		t.Error("list view empty")
	}
	m.openSelected()
	if m.View() == "" {
		t.Error("detail view empty")
	}
}

func TestUpdateQuitAndResize(t *testing.T) {
	m, _ := newModel(fixtureService(t))
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40}); cmd != nil {
		t.Errorf("resize should not emit a cmd")
	}
	// 'q' on the list quits (cmd non-nil); just assert no panic and a cmd returned.
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
		t.Error("q should return a quit cmd")
	}
}

func TestCyclePlatFilter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "skills", "alpha", "SKILL.md"),
		"---\nname: alpha\ndescription: a\n---\nx\n")
	writeFile(t, filepath.Join(root, "skills.toml"),
		"[meta]\nversion = 1\n\n[skills.alpha]\nsource = \"up\"\nmode = \"tracked\"\n")
	claude := t.TempDir()
	codex := t.TempDir()
	writeFile(t, filepath.Join(codex, "skills", "cdx", "SKILL.md"),
		"---\nname: cdx\ndescription: c\n---\nx\n")

	svc := inventory.NewService(root, claude)
	svc.CodexHome, svc.CursorHome = codex, t.TempDir()
	m, err := newModel(svc)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.invRefs) != 2 { // managed alpha + codex cdx
		t.Fatalf("all: refs = %d, want 2", len(m.invRefs))
	}
	m.cyclePlat() // -> claude (no claude assets)
	if m.plat != inventory.PlatformClaude || len(m.invRefs) != 0 {
		t.Errorf("claude: plat=%s refs=%d", m.plat, len(m.invRefs))
	}
	m.cyclePlat() // -> codex (cdx only)
	if m.plat != inventory.PlatformCodex || len(m.invRefs) != 1 || m.invRefs[0].name != "cdx" {
		t.Errorf("codex: plat=%s refs=%+v", m.plat, m.invRefs)
	}
}

func TestOneLine(t *testing.T) {
	if got := oneLine("a\n  b\tc\n"); got != "a b c" {
		t.Errorf("oneLine = %q", got)
	}
}
