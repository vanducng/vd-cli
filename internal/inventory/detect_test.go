package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstallAgents_None(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VD_CODEX_HOME", "")
	t.Setenv("VD_CURSOR_HOME", "")

	got, err := DetectInstallAgents()
	if err != nil {
		t.Fatalf("DetectInstallAgents: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want none", got)
	}
}

func TestDetectInstallAgents_CursorOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VD_CODEX_HOME", "")
	t.Setenv("VD_CURSOR_HOME", "")
	if err := os.Mkdir(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectInstallAgents()
	if err != nil {
		t.Fatalf("DetectInstallAgents: %v", err)
	}
	if len(got) != 1 || got[0].Name != "cursor" {
		t.Fatalf("got %+v, want cursor only", got)
	}
}

func TestDetectInstallAgents_SeveralAndEnvOverrides(t *testing.T) {
	home := t.TempDir()
	codex := t.TempDir()
	cursor := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("VD_CODEX_HOME", codex)
	t.Setenv("VD_CURSOR_HOME", cursor)
	for _, dir := range []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".factory"),
		filepath.Join(home, ".pi"),
	} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := DetectInstallAgents()
	if err != nil {
		t.Fatalf("DetectInstallAgents: %v", err)
	}
	want := []string{"claude", "codex", "cursor", "droid", "pi"}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %v", got, want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("got[%d] = %q, want %q", i, got[i].Name, name)
		}
	}
	if got[1].Home != codex || got[2].Home != cursor {
		t.Fatalf("env homes = %+v", got)
	}
}

func TestDetectInstallAgents_SkipsMissingHomes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("VD_CODEX_HOME", "")
	t.Setenv("VD_CURSOR_HOME", "")
	if err := os.Mkdir(filepath.Join(home, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := detectInstallAgents(home)
	if len(got) != 1 || got[0].Name != "codex" {
		t.Fatalf("got %+v, want codex only", got)
	}
}
