package claudeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/vanducng/vd-cli/v2/internal/extension"
)

func cwExt() extension.Extension {
	return extension.Extension{
		Name: "codex-workflow", Transport: "stdio", Command: "uv",
		Args:    []string{"run", "--directory", "{dir}", "codex-workflow-server"},
		Dir:     "/abs/ext",
		Targets: []string{"codex", "claude"}, Scope: "project",
		StartupTimeoutSec: 120, Enabled: true,
	}
}

func parseTOMLFile(t *testing.T, p string) map[string]any {
	t.Helper()
	d, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := toml.Unmarshal(d, &m); err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	return m
}

func TestRegisterCodexMCP_PreservesAndResolves(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte("model = \"gpt-5\"\n\n[mcp_servers.other]\nurl = \"https://x.test\"\n\n[features]\nhooks = true\n"), 0o644)

	if err := RegisterCodexMCP(p, cwExt()); err != nil {
		t.Fatal(err)
	}
	m := parseTOMLFile(t, p)
	mcp := m["mcp_servers"].(map[string]any)
	if _, ok := mcp["other"]; !ok {
		t.Fatal("existing server lost")
	}
	if m["features"] == nil {
		t.Fatal("features table lost")
	}
	cw := mcp["codex-workflow"].(map[string]any)
	if cw["command"] != "uv" {
		t.Fatalf("command = %v", cw["command"])
	}
	args := cw["args"].([]any)
	if args[2] != "/abs/ext" {
		t.Fatalf("{dir} not resolved: %v", args)
	}

	before, _ := os.ReadFile(p)
	if err := RegisterCodexMCP(p, cwExt()); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("re-register not idempotent")
	}
}

func TestUnregisterCodexMCP_RemovesOnlyTarget(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte("[mcp_servers.other]\nurl = \"https://x.test\"\n"), 0o644)
	if err := RegisterCodexMCP(p, cwExt()); err != nil {
		t.Fatal(err)
	}
	if err := UnregisterCodexMCP(p, "codex-workflow"); err != nil {
		t.Fatal(err)
	}
	mcp := parseTOMLFile(t, p)["mcp_servers"].(map[string]any)
	if _, ok := mcp["codex-workflow"]; ok {
		t.Fatal("target not removed")
	}
	if _, ok := mcp["other"]; !ok {
		t.Fatal("removed the wrong server")
	}
}

func TestRegisterCodexMCP_HTTP(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte(""), 0o644)
	e := cwExt()
	e.Transport, e.URL, e.Command = "http", "https://h.test/mcp", ""
	if err := RegisterCodexMCP(p, e); err != nil {
		t.Fatal(err)
	}
	cw := parseTOMLFile(t, p)["mcp_servers"].(map[string]any)["codex-workflow"].(map[string]any)
	if cw["url"] != "https://h.test/mcp" {
		t.Fatalf("url = %v", cw["url"])
	}
	if _, has := cw["command"]; has {
		t.Fatal("http transport must not write command")
	}
}

func TestRegisterCodexMCP_CreatesMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.toml")
	if err := RegisterCodexMCP(p, cwExt()); err != nil {
		t.Fatal(err)
	}
	if parseTOMLFile(t, p)["mcp_servers"] == nil {
		t.Fatal("not created")
	}
}

func TestRegisterCodexMCP_NoEnvWritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte(""), 0o644)
	e := cwExt()
	e.Env = []string{"OPENAI_API_KEY"}
	RegisterCodexMCP(p, e)
	d, _ := os.ReadFile(p)
	if strings.Contains(string(d), "OPENAI_API_KEY") || strings.Contains(string(d), ".env") {
		t.Fatalf("env must never be written to config:\n%s", d)
	}
}

func TestWriteCodexConfigSafe_RestoresOnInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	orig := []byte("model = \"gpt-5\"\n")
	os.WriteFile(p, orig, 0o644)
	bad := []byte("model = \"x\"\n[broken table without close\nkey = ")
	if err := writeCodexConfigSafe(p, orig, bad); err == nil {
		t.Fatal("expected error writing invalid TOML")
	}
	got, _ := os.ReadFile(p)
	if string(got) != string(orig) {
		t.Fatalf("original not restored, got:\n%s", got)
	}
}
