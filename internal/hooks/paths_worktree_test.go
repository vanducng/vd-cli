package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveUmbrellaRoot_WorktreeAnchorsToMain verifies that the umbrella root
// resolves to the MAIN worktree even when the baseDir is a linked worktree, so
// agent artifacts survive `git worktree remove`. The main checkout must stay
// byte-identical (umbrella under the local root).
func TestResolveUmbrellaRoot_WorktreeAnchorsToMain(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	pathsCJS, err := filepath.Abs(filepath.Join("assets", "lib", "paths.cjs"))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	git := func(workdir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	git(dir, "init", "-b", "main")
	git(dir, "config", "user.email", "t@t.t")
	git(dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, ".vd.json"), []byte(`{"paths":{"umbrella":".workbench"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(dir, "add", "-A")
	git(dir, "commit", "-m", "init")

	// Main root as git sees it (handles macOS /private symlink normalization).
	mainRoot := git(dir, "rev-parse", "--show-toplevel")
	wt := filepath.Join(dir, ".worktrees", "wt")
	git(dir, "worktree", "add", wt, "-b", "feat/x")

	resolve := func(baseDir string) string {
		t.Helper()
		script := `const p=require(process.env.PCJS);` +
			`process.stdout.write(p.resolveUmbrellaRoot({paths:{umbrella:'.workbench'}}, process.env.BASE) || 'NULL');`
		cmd := exec.Command("node", "-e", script)
		cmd.Env = append(os.Environ(), "PCJS="+pathsCJS, "BASE="+baseDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node resolve(%s): %v\n%s", baseDir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	want := filepath.Join(mainRoot, ".workbench")

	if got := resolve(mainRoot); got != want {
		t.Errorf("main checkout: umbrella = %q, want %q", got, want)
	}
	if got := resolve(wt); got != want {
		t.Errorf("from worktree: umbrella = %q, want %q (must anchor to MAIN, not the worktree)", got, want)
	}
}
