package claudeconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterClaudeMCP_PreservesOtherKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".claude.json")
	os.WriteFile(p, []byte(`{"numStartups":5,"mcpServers":{"other":{"type":"http","url":"https://x"}}}`), 0o644)
	if err := RegisterClaudeMCP(p, cwExt()); err != nil {
		t.Fatal(err)
	}
	d, _ := os.ReadFile(p)
	if !json.Valid(d) {
		t.Fatal("invalid JSON after write")
	}
	var m map[string]any
	json.Unmarshal(d, &m)
	if m["numStartups"] == nil {
		t.Fatal("top-level key lost")
	}
	servers := m["mcpServers"].(map[string]any)
	if servers["other"] == nil {
		t.Fatal("existing server lost")
	}
	cw := servers["codex-workflow"].(map[string]any)
	if cw["command"] != "uv" || cw["type"] != "stdio" {
		t.Fatalf("unexpected server obj: %v", cw)
	}
}

func TestRegisterClaudeMCP_ProjectCreate(t *testing.T) {
	p := ClaudeProjectConfigPath(t.TempDir())
	if err := RegisterClaudeMCP(p, cwExt()); err != nil {
		t.Fatal(err)
	}
	d, _ := os.ReadFile(p)
	if !json.Valid(d) {
		t.Fatal("invalid JSON")
	}
	var m map[string]any
	json.Unmarshal(d, &m)
	if m["mcpServers"].(map[string]any)["codex-workflow"] == nil {
		t.Fatal("not registered")
	}
}

func TestUnregisterClaudeMCP(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".mcp.json")
	RegisterClaudeMCP(p, cwExt())
	if err := UnregisterClaudeMCP(p, "codex-workflow"); err != nil {
		t.Fatal(err)
	}
	d, _ := os.ReadFile(p)
	var m map[string]any
	json.Unmarshal(d, &m)
	if s, ok := m["mcpServers"].(map[string]any); ok {
		if _, has := s["codex-workflow"]; has {
			t.Fatal("not removed")
		}
	}
}

func TestRegisterClaudeMCP_NoEnvLeak(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".mcp.json")
	e := cwExt()
	e.Env = []string{"OPENAI_API_KEY"}
	RegisterClaudeMCP(p, e)
	d, _ := os.ReadFile(p)
	if strings.Contains(string(d), "OPENAI_API_KEY") {
		t.Fatalf("env leaked into config:\n%s", d)
	}
}

func TestReadOrInitJSON_InvalidErrors(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".claude.json")
	os.WriteFile(p, []byte("{not json"), 0o644)
	if _, _, err := readOrInitJSON(p); err == nil {
		t.Fatal("expected error on invalid JSON file")
	}
}
