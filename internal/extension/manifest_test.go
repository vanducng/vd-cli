package extension

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "extension.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return p
}

func TestLoadExtension(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
		check   func(t *testing.T, e Extension)
	}{
		{
			name: "valid stdio",
			body: `name = "codex-workflow"
description = "orchestrator"
transport = "stdio"
command = "uv"
args = ["run", "{dir}", "server"]
env = ["OPENAI_API_KEY"]
targets = ["codex", "claude"]
scope = "project"
startup_timeout_sec = 120
enabled = true`,
			check: func(t *testing.T, e Extension) {
				if e.Name != "codex-workflow" || e.Transport != "stdio" || !e.Enabled {
					t.Fatalf("unexpected: %+v", e)
				}
				if len(e.Targets) != 2 {
					t.Fatalf("targets = %v", e.Targets)
				}
			},
		},
		{
			name: "valid http",
			body: `name = "remote"
transport = "http"
url = "https://example.test/mcp"
targets = ["codex"]`,
			check: func(t *testing.T, e Extension) {
				if e.URL == "" || e.Scope != "project" { // scope defaults to project
					t.Fatalf("unexpected: %+v", e)
				}
			},
		},
		{name: "missing name", body: `transport = "stdio"
command = "x"
targets = ["codex"]`, wantErr: true},
		{name: "bad transport", body: `name = "x"
transport = "grpc"
targets = ["codex"]`, wantErr: true},
		{name: "stdio without command", body: `name = "x"
transport = "stdio"
targets = ["codex"]`, wantErr: true},
		{name: "http without url", body: `name = "x"
transport = "http"
targets = ["codex"]`, wantErr: true},
		{name: "no targets", body: `name = "x"
transport = "stdio"
command = "uv"`, wantErr: true},
		{name: "unknown target", body: `name = "x"
transport = "stdio"
command = "uv"
targets = ["gemini"]`, wantErr: true},
		{name: "bad scope", body: `name = "x"
transport = "stdio"
command = "uv"
targets = ["codex"]
scope = "everywhere"`, wantErr: true},
		{name: "unknown field rejected", body: `name = "x"
transport = "stdio"
command = "uv"
targets = ["codex"]
transprot = "typo"`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := LoadExtension(writeManifest(t, tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err == nil && tc.check != nil {
				tc.check(t, e)
			}
		})
	}
}

func TestResolvedArgs(t *testing.T) {
	e := Extension{Args: []string{"run", "{dir}", "server"}, Dir: "/abs/ext"}
	got := e.ResolvedArgs()
	want := []string{"run", "/abs/ext", "server"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ResolvedArgs = %v, want %v", got, want)
		}
	}
}
