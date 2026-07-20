package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vanducng/vd-cli/v2/internal/target"
)

// DroidOptions keeps skill installation independent from Droid runtime configuration.
type DroidOptions struct {
	Scope  string
	Dest   string
	Skills []string
	Copy   bool
	Force  bool
	DryRun bool
}

// Droid installs skills without requiring a running Droid session.
func Droid(repoRoot string, opts DroidOptions) ([]Result, error) {
	scope := opts.Scope
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "repo" {
		return nil, fmt.Errorf("invalid droid scope %q (valid: user, repo)", scope)
	}

	destRoot := opts.Dest
	if destRoot == "" {
		var err error
		destRoot, err = droidDest(repoRoot, scope)
		if err != nil {
			return nil, err
		}
	} else if err := assertSafeDest(destRoot); err != nil {
		return nil, err
	}

	linkOpts := LinkOptions{
		Copy:   opts.Copy,
		Force:  opts.Force,
		DryRun: opts.DryRun,
	}
	defaultRepoDest := filepath.Join(repoRoot, ".factory", "skills")
	managedRepoDest, err := sameCleanPath(destRoot, defaultRepoDest)
	if err != nil {
		return nil, err
	}
	if scope != "repo" || !managedRepoDest {
		return installSkillLinks(repoRoot, destRoot, opts.Skills, linkOpts)
	}
	claim := func(name string) (func(), error) {
		created, err := target.ClaimDroidSkill(repoRoot, name)
		if err != nil || !created {
			return func() {}, err
		}
		return func() { _ = target.UnclaimDroidSkill(repoRoot, name) }, nil
	}
	return installSkillLinksClaimed(repoRoot, destRoot, opts.Skills, linkOpts, claim)
}

func sameCleanPath(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, fmt.Errorf("resolve destination %s: %w", a, err)
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, fmt.Errorf("resolve destination %s: %w", b, err)
	}
	return filepath.Clean(absA) == filepath.Clean(absB), nil
}

func droidDest(repoRoot, scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".factory", "skills"), nil
	case "repo":
		return filepath.Join(repoRoot, ".factory", "skills"), nil
	default:
		return "", fmt.Errorf("invalid droid scope %q (valid: user, repo)", scope)
	}
}
