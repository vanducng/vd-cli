package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployCodexAgents(t *testing.T) {
	repo := t.TempDir()
	agents := filepath.Join(repo, "agents")
	os.MkdirAll(agents, 0o755)
	os.WriteFile(filepath.Join(agents, "planner.toml"), []byte("name = \"planner\"\n"), 0o644)
	os.WriteFile(filepath.Join(agents, "tester.toml"), []byte("name = \"tester\"\n"), 0o644)
	os.WriteFile(filepath.Join(agents, "README.md"), []byte("# not a toml"), 0o644)

	dest := filepath.Join(t.TempDir(), "agents")

	// dry-run: enumerates, writes nothing
	res, err := DeployCodexAgents(repo, CodexOptions{Dest: dest, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("dry-run got %d, want 2 (.toml only)", len(res))
	}
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("dry-run should not create dest")
	}

	// real run: copies both
	res, err = DeployCodexAgents(repo, CodexOptions{Dest: dest})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d, want 2", len(res))
	}
	for _, n := range []string{"planner.toml", "tester.toml"} {
		if _, err := os.Stat(filepath.Join(dest, n)); err != nil {
			t.Fatalf("missing %s: %v", n, err)
		}
	}

	// idempotent re-run
	if _, err := DeployCodexAgents(repo, CodexOptions{Dest: dest}); err != nil {
		t.Fatalf("re-run: %v", err)
	}
}

func TestDeployCodexAgents_NoDir(t *testing.T) {
	res, err := DeployCodexAgents(t.TempDir(), CodexOptions{})
	if err != nil {
		t.Fatalf("missing agents dir should not error: %v", err)
	}
	if res != nil {
		t.Fatalf("want nil, got %v", res)
	}
}
