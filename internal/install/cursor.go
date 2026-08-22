package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// CursorOptions keeps skill installation independent from Cursor runtime configuration.
type CursorOptions struct {
	Scope  string
	Dest   string
	Skills []string
	Copy   bool
	Force  bool
	DryRun bool
}

// Cursor installs local skills into Cursor discovery paths without requiring a
// running Cursor session. User scope writes $HOME/.cursor/skills (or
// $VD_CURSOR_HOME/skills when that inventory override is set). Repo scope
// writes .cursor/skills in the current repo.
func Cursor(repoRoot string, opts CursorOptions) ([]Result, error) {
	scope := opts.Scope
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "repo" {
		return nil, fmt.Errorf("invalid cursor scope %q (valid: user, repo)", scope)
	}

	destRoot := opts.Dest
	if destRoot == "" {
		var err error
		destRoot, err = cursorDest(repoRoot, scope)
		if err != nil {
			return nil, err
		}
	} else if err := assertSafeDest(destRoot); err != nil {
		return nil, err
	}

	return installSkillLinks(repoRoot, destRoot, opts.Skills, LinkOptions{
		Copy:   opts.Copy,
		Force:  opts.Force,
		DryRun: opts.DryRun,
	})
}

func cursorDest(repoRoot, scope string) (string, error) {
	switch scope {
	case "user":
		home, err := cursorHome()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "skills"), nil
	case "repo":
		return filepath.Join(repoRoot, ".cursor", "skills"), nil
	default:
		return "", fmt.Errorf("invalid cursor scope %q (valid: user, repo)", scope)
	}
}

// cursorHome matches inventory discovery: $VD_CURSOR_HOME if set, else ~/.cursor.
func cursorHome() (string, error) {
	if home := os.Getenv("VD_CURSOR_HOME"); home != "" {
		return home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cursor"), nil
}
