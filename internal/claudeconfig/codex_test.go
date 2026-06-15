package claudeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func codexBackupCount(t *testing.T, path string) int {
	t.Helper()
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext) + ".bak."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

func TestWireCodexNotifyPreservesOtherLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := `# Codex config
model = "x"
notify = ["old-notify"]
approval_policy = "on-request"
`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cmd := []string{"python3", "/abs/.claude/hooks/agent-notify.py", "codex"}
	prev, err := WireCodexNotify(path, cmd)
	if err != nil {
		t.Fatalf("WireCodexNotify: %v", err)
	}
	if prev != `notify = ["old-notify"]` {
		t.Errorf("replacedPrev = %q, want the old notify line", prev)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	want := `# Codex config
model = "x"
notify = ["python3", "/abs/.claude/hooks/agent-notify.py", "codex"]
approval_policy = "on-request"
`
	if string(got) != want {
		t.Errorf("result mismatch:\n--- got\n%s\n--- want\n%s", got, want)
	}
	if codexBackupCount(t, path) != 1 {
		t.Errorf("expected exactly one backup file, got %d", codexBackupCount(t, path))
	}
}

func TestWireCodexNotifyAlreadyOurs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := `model = "x"
notify = ["python3", "/abs/.claude/hooks/agent-notify.py", "codex"]
`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cmd := []string{"python3", "/abs/.claude/hooks/agent-notify.py", "codex"}
	prev, err := WireCodexNotify(path, cmd)
	if err != nil {
		t.Fatalf("WireCodexNotify: %v", err)
	}
	if prev != "" {
		t.Errorf("replacedPrev = %q, want empty (line already ours)", prev)
	}
	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Errorf("line should be unchanged:\n%s", got)
	}
}

func TestWireCodexNotifyMissingFileCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	cmd := []string{"/abs/.claude/hooks/notify.sh"}
	prev, err := WireCodexNotify(path, cmd)
	if err != nil {
		t.Fatalf("WireCodexNotify: %v", err)
	}
	if prev != "" {
		t.Errorf("replacedPrev = %q, want empty for new file", prev)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(got) != `notify = ["/abs/.claude/hooks/notify.sh"]`+"\n" {
		t.Errorf("new file content = %q", got)
	}
}

func TestWireCodexNotifyAppendsWhenNoNotifyLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := `model = "x"` // no trailing newline, no notify
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	cmd := []string{"node", "/abs/hooks/n.cjs"}
	prev, err := WireCodexNotify(path, cmd)
	if err != nil {
		t.Fatalf("WireCodexNotify: %v", err)
	}
	if prev != "" {
		t.Errorf("replacedPrev = %q, want empty (appended)", prev)
	}
	got, _ := os.ReadFile(path)
	want := "model = \"x\"\nnotify = [\"node\", \"/abs/hooks/n.cjs\"]\n"
	if string(got) != want {
		t.Errorf("appended content mismatch:\n--- got\n%q\n--- want\n%q", got, want)
	}
}

func TestWireCodexNotifyQuotesSpecialChars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cmd := []string{`/path with "quote"\and-backslash`}
	if _, err := WireCodexNotify(path, cmd); err != nil {
		t.Fatalf("WireCodexNotify: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := `notify = ["/path with \"quote\"\\and-backslash"]` + "\n"
	if string(got) != want {
		t.Errorf("escaping mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestUnwireCodexNotifyRemovesOnlyOurs(t *testing.T) {
	progPath := "/abs/.claude/hooks/agent-notify.py"
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := `model = "x"
notify = ["python3", "` + progPath + `", "codex"]
approval_policy = "on-request"
`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := UnwireCodexNotify(path, progPath); err != nil {
		t.Fatalf("UnwireCodexNotify: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := `model = "x"
approval_policy = "on-request"
`
	if string(got) != want {
		t.Errorf("unwire mismatch:\n--- got\n%s\n--- want\n%s", got, want)
	}
	if codexBackupCount(t, path) != 1 {
		t.Errorf("expected one backup, got %d", codexBackupCount(t, path))
	}
}

func TestUnwireCodexNotifyLeavesForeignLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := `notify = ["/some/other/notifier"]` + "\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := UnwireCodexNotify(path, "/abs/.claude/hooks/agent-notify.py"); err != nil {
		t.Fatalf("UnwireCodexNotify: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Errorf("foreign notify line should be untouched, got:\n%s", got)
	}
	if codexBackupCount(t, path) != 0 {
		t.Errorf("expected no backup for no-op, got %d", codexBackupCount(t, path))
	}
}

func TestUnwireCodexNotifyMissingFileNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")
	if err := UnwireCodexNotify(path, "/abs/x"); err != nil {
		t.Fatalf("UnwireCodexNotify on missing file should be no-op, got: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("missing file should not be created")
	}
}

func TestCodexNotifyCommand(t *testing.T) {
	hooksDir := "/home/u/.claude/hooks"
	cases := []struct {
		name string
		hook Hook
		want []string
	}{
		{
			name: "python runtime with args yields absolute path",
			hook: Hook{File: "agent-notify.py", Runtime: "python3", Event: "codex.notify", Args: []string{"codex"}},
			want: []string{"python3", "/home/u/.claude/hooks/agent-notify.py", "codex"},
		},
		{
			name: "empty runtime puts abs path first",
			hook: Hook{File: "notify.sh", Event: "codex.notify"},
			want: []string{"/home/u/.claude/hooks/notify.sh"},
		},
		{
			name: "nested lib path joined",
			hook: Hook{File: "lib/notify.cjs", Runtime: "node", Event: "codex.notify", Args: []string{"a", "b"}},
			want: []string{"node", "/home/u/.claude/hooks/lib/notify.cjs", "a", "b"},
		},
	}
	for _, c := range cases {
		got := CodexNotifyCommand(c.hook, hooksDir)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}
