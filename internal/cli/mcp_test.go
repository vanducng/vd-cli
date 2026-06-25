package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMcpLogDir(t *testing.T) {
	t.Setenv("VD_LOG_DIR", "/tmp/vd-logs-test")
	if got := mcpLogDir(); got != "/tmp/vd-logs-test" {
		t.Fatalf("VD_LOG_DIR not honored: %s", got)
	}
	t.Setenv("VD_LOG_DIR", "")
	if !strings.HasSuffix(mcpLogDir(), filepath.Join(".vd", "logs")) {
		t.Fatalf("default log dir wrong: %s", mcpLogDir())
	}
}

func TestMcpLogsCmd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VD_LOG_DIR", dir)
	os.WriteFile(filepath.Join(dir, "codex-workflow.log"), []byte("line1\nline2\nline3\n"), 0o644)

	// full
	cmd := newMcpLogsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"codex-workflow"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "line1") || !strings.Contains(out.String(), "line3") {
		t.Fatalf("missing lines: %q", out.String())
	}

	// tail
	cmd = newMcpLogsCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"codex-workflow", "--tail", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "line1") || !strings.Contains(out.String(), "line3") {
		t.Fatalf("tail=1 wrong: %q", out.String())
	}

	// missing
	cmd = newMcpLogsCmd()
	cmd.SetArgs([]string{"nope"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing log")
	}

	// path traversal rejected
	for _, bad := range []string{"../../etc/passwd", "a/b", ".."} {
		cmd = newMcpLogsCmd()
		cmd.SetArgs([]string{bad})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected rejection of %q", bad)
		}
	}
}

func TestFollowLog(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.log")
	os.WriteFile(p, []byte("a\nb\nc\n"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	var buf bytes.Buffer
	if err := followLog(ctx, &buf, p, 2); err != nil { // start after "a\n"
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "b") || !strings.Contains(buf.String(), "c") {
		t.Fatalf("did not stream appended lines: %q", buf.String())
	}
}

func TestResolveExtensionsDir_Env(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "extensions"), 0o755)
	t.Setenv("VD_EXTENSIONS_DIR", dir)
	got, err := resolveExtensionsDir("")
	if err != nil || got != dir {
		t.Fatalf("got %q err %v, want %q", got, err, dir)
	}
}
