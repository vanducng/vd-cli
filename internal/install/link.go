package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// LinkOptions controls per-skill symlink/copy operations shared by the
// Codex installer and the Claude Code dev installer.
type LinkOptions struct {
	Copy   bool
	Force  bool
	DryRun bool
}

// LinkSkill places src at dst (under destRoot) as a relative symlink, or as a
// recursive copy when Copy is set / running on Windows. Returns a short action
// label ("symlinked", "copied", "unchanged", "would symlink", "would copy").
func LinkSkill(src, dst, destRoot string, opts LinkOptions) (string, error) {
	copyMode := opts.Copy || runtime.GOOS == "windows"
	if opts.DryRun {
		if copyMode {
			return "would copy", nil
		}
		return "would symlink", nil
	}

	if !copyMode {
		rel, err := filepath.Rel(destRoot, src)
		if err != nil {
			return "", fmt.Errorf("resolve relative symlink target: %w", err)
		}
		if existing, err := os.Readlink(dst); err == nil && existing == rel {
			return "unchanged", nil
		}
	}

	if _, err := os.Lstat(dst); err == nil {
		if !opts.Force {
			return "", fmt.Errorf("destination %s already exists; use --force to replace it", dst)
		}
		if err := os.RemoveAll(dst); err != nil {
			return "", fmt.Errorf("remove existing destination: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if copyMode {
		if err := copyDir(src, dst); err != nil {
			return "", err
		}
		return "copied", nil
	}

	rel, err := filepath.Rel(destRoot, src)
	if err != nil {
		return "", fmt.Errorf("resolve relative symlink target: %w", err)
	}
	if err := os.Symlink(rel, dst); err != nil {
		return "", fmt.Errorf("create symlink: %w", err)
	}
	return "symlinked", nil
}
