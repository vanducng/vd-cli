package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vanducng/vd-cli/v2/internal/target"
)

// PiOptions keeps skill installation independent from Pi runtime configuration.
type PiOptions struct {
	Scope  string
	Dest   string
	Skills []string
	Copy   bool
	Force  bool
	DryRun bool
}

// Pi installs skills without requiring a running Pi session.
func Pi(repoRoot string, opts PiOptions) ([]Result, error) {
	scope := opts.Scope
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "repo" {
		return nil, fmt.Errorf("invalid pi scope %q (valid: user, repo)", scope)
	}

	destRoot := opts.Dest
	if destRoot == "" {
		var err error
		destRoot, err = piDest(repoRoot, scope)
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
	defaultRepoDest := filepath.Join(repoRoot, ".pi", "skills")
	managedRepoDest, err := sameCleanPath(destRoot, defaultRepoDest)
	if err != nil {
		return nil, err
	}
	if scope != "repo" || !managedRepoDest {
		return installSkillLinks(repoRoot, destRoot, opts.Skills, linkOpts)
	}
	claim := func(name string) (func() error, error) {
		created, err := target.ClaimPiSkill(repoRoot, name)
		if err != nil || !created {
			return func() error { return nil }, err
		}
		return func() error { return target.UnclaimPiSkill(repoRoot, name) }, nil
	}
	return installSkillLinksClaimed(repoRoot, destRoot, opts.Skills, linkOpts, claim)
}

// piDest resolves pi's skill directory, which unlike Droid differs by scope:
// user scope carries an extra "agent" segment (~/.pi/agent/skills), while repo
// scope is <repo>/.pi/skills.
func piDest(repoRoot, scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".pi", "agent", "skills"), nil
	case "repo":
		return filepath.Join(repoRoot, ".pi", "skills"), nil
	default:
		return "", fmt.Errorf("invalid pi scope %q (valid: user, repo)", scope)
	}
}
