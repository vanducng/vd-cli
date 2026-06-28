package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveUmbrellaRoot_NoGitAnchorsCwd verifies that when umbrella is set but the
// working dir is NOT inside any git repo (a brand-new project not yet `git init`'d, or
// a non-git tool session), the umbrella anchors at the working dir — returning
// <cwd>/.workbench — instead of returning null and silently scattering artifacts to
// the legacy plans/ layout at cwd.
func TestResolveUmbrellaRoot_NoGitAnchorsCwd(t *testing.T) {
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

	// A non-git working dir and an unrelated non-git HOME (so the stray-$HOME guard
	// can't interfere). Neither is inside a git repo.
	base := t.TempDir()
	home := t.TempDir()

	script := `const p=require(process.env.PCJS);` +
		`const c={paths:{umbrella:'.workbench'}};` +
		`process.stdout.write(p.resolveUmbrellaRoot(c, process.env.BASE) || 'NULL');`
	cmd := exec.Command("node", "-e", script)
	// HOME drives os.homedir() inside the resolver — point it away from base.
	cmd.Env = append(os.Environ(), "PCJS="+pathsCJS, "BASE="+base, "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node resolve: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))

	if got == "NULL" {
		t.Fatalf("umbrella disabled without git: got NULL, want a <cwd>/.workbench anchored at %q", base)
	}
	if realpath(t, filepath.Dir(got)) != realpath(t, base) || filepath.Base(got) != ".workbench" {
		t.Errorf("umbrella = %q, want cwd-anchored %q", got, filepath.Join(base, ".workbench"))
	}
}
