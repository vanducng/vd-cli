package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveUmbrellaRoot_HomeAncestorNoHijack verifies the stray-ancestor guard:
// when a coincidental git repo is rooted at $HOME (e.g. an accidental `git init ~`),
// a project working dir nested below it must anchor its .workbench umbrella to the
// PROJECT dir, not to $HOME — otherwise every repo-less subdir scatters artifacts
// into the home directory (the miu-cr regression).
func TestResolveUmbrellaRoot_HomeAncestorNoHijack(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	pathsCJS, err := filepath.Abs(filepath.Join("..", "..", "hooks", "lib", "paths.cjs"))
	if err != nil {
		t.Fatal(err)
	}

	// A throwaway HOME that is itself a git repo (the stray ancestor).
	fakeHome := t.TempDir()
	git := func(workdir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = workdir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git(fakeHome, "init", "-b", "main")
	git(fakeHome, "config", "user.email", "t@t.t")
	git(fakeHome, "config", "user.name", "t")
	git(fakeHome, "commit", "--allow-empty", "-m", "stray home repo")

	// A project dir nested below the stray home repo, with NO git of its own.
	project := filepath.Join(fakeHome, "git", "personal", "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	resolve := func(baseDir string) string {
		t.Helper()
		script := `const p=require(process.env.PCJS);` +
			`process.stdout.write(p.resolveUmbrellaRoot({paths:{umbrella:'.workbench'}}, process.env.BASE) || 'NULL');`
		cmd := exec.Command("node", "-e", script)
		// HOME drives os.homedir() inside the resolver — point it at the stray repo.
		cmd.Env = append(os.Environ(), "PCJS="+pathsCJS, "BASE="+project, "HOME="+fakeHome)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node resolve(%s): %v\n%s", project, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	got := resolve(project)
	wantProject := filepath.Join(project, ".workbench")
	homeUmbrella := filepath.Join(fakeHome, ".workbench")

	// realpath both sides (macOS /var → /private/var) before comparing.
	if realpath(t, got) == realpath(t, homeUmbrella) {
		t.Fatalf("umbrella hijacked to $HOME: got %q, must anchor to project %q", got, wantProject)
	}
	if realpath(t, got) != realpath(t, wantProject) {
		t.Errorf("umbrella = %q, want project-anchored %q", got, wantProject)
	}
}

func realpath(t *testing.T, p string) string {
	t.Helper()
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
