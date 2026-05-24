package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeDevOptions controls Claude Code dev-symlink installs into the local
// user-level skills directory ($HOME/.claude/skills/<name>).
type ClaudeDevOptions struct {
	Dest   string
	Skills []string
	Copy   bool
	Force  bool
	DryRun bool
}

// ClaudeDev symlinks (or copies) each requested skill into the Claude Code
// user-level skills directory so edits in the repo are picked up live by
// Claude Code without bumping a plugin version. Returns per-skill results.
func ClaudeDev(repoRoot string, opts ClaudeDevOptions) ([]Result, error) {
	destRoot := opts.Dest
	if destRoot == "" {
		var err error
		destRoot, err = claudeUserSkillsDir()
		if err != nil {
			return nil, err
		}
	} else if err := assertSafeDest(destRoot); err != nil {
		return nil, err
	}

	skillsDir := filepath.Join(repoRoot, "skills")
	names, err := resolveSkillNames(skillsDir, opts.Skills)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no local skills found in %s", skillsDir)
	}

	if !opts.DryRun {
		if err := os.MkdirAll(destRoot, 0o755); err != nil {
			return nil, fmt.Errorf("create claude skills dir %s: %w", destRoot, err)
		}
	}

	results := make([]Result, 0, len(names))
	for _, name := range names {
		src := filepath.Join(skillsDir, name)
		dst := filepath.Join(destRoot, name)
		if err := assertSimpleName(name); err != nil {
			return nil, err
		}
		if err := assertInsideRoot(src, repoRoot); err != nil {
			return nil, err
		}
		action, err := LinkSkill(src, dst, destRoot, LinkOptions{
			Copy:   opts.Copy,
			Force:  opts.Force,
			DryRun: opts.DryRun,
		})
		if err != nil {
			return nil, fmt.Errorf("install %s: %w", name, err)
		}
		results = append(results, Result{Name: name, Source: src, Dest: dst, Action: action})
	}
	return results, nil
}

func claudeUserSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "skills"), nil
}
