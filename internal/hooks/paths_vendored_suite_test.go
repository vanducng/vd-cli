package hooks

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The control-plane hooks under hooks/ are VENDORED verbatim from the skills repo
// (the canonical source — see scripts/sync-hooks.sh and the hooks-drift CI check).
// Their behavior is owned and tested there in hooks/lib/paths.test.cjs, which travels
// with the vendored copy. Run that canonical suite here so vd-cli CI catches a vendored
// copy that misbehaves, without maintaining a divergent second set of assertions.
func TestVendoredHooksPathsSuite(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	suite, err := filepath.Abs(filepath.Join("..", "..", "hooks", "lib", "paths.test.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "--test", suite).CombinedOutput()
	if err != nil {
		t.Fatalf("vendored hooks/lib/paths.test.cjs failed:\n%s", out)
	}
}
