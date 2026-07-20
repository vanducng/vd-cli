// Package install contains local agent installation helpers.
package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CodexOptions controls how Codex installs skills into a Codex agent directory.
type CodexOptions struct {
	Scope  string
	Dest   string
	Skills []string
	Copy   bool
	Force  bool
	DryRun bool
}

// Result describes a single skill install action performed by Codex.
type Result struct {
	Name   string
	Source string
	Dest   string
	Action string
}

// Codex installs the requested skills into the Codex agent skills directory
// for the given repo, returning per-skill results in name order.
func Codex(repoRoot string, opts CodexOptions) ([]Result, error) {
	scope := opts.Scope
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "repo" {
		return nil, fmt.Errorf("invalid codex scope %q (valid: user, repo)", scope)
	}

	destRoot := opts.Dest
	if destRoot == "" {
		var err error
		destRoot, err = codexDest(repoRoot, scope)
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

type skillClaim func(name string) (rollback func() error, err error)

func installSkillLinks(repoRoot, destRoot string, skills []string, opts LinkOptions) ([]Result, error) {
	return installSkillLinksClaimed(repoRoot, destRoot, skills, opts, nil)
}

func installSkillLinksClaimed(repoRoot, destRoot string, skills []string, opts LinkOptions, claim skillClaim) ([]Result, error) {
	skillsDir := filepath.Join(repoRoot, "skills")
	names, err := resolveSkillNames(skillsDir, skills)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no local skills found in %s", skillsDir)
	}
	if !opts.DryRun {
		if err := os.MkdirAll(destRoot, 0o755); err != nil {
			return nil, fmt.Errorf("create skills dir %s: %w", destRoot, err)
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
		rollback := func() error { return nil }
		if claim != nil && !opts.DryRun {
			rollback, err = claim(name)
			if err != nil {
				return nil, fmt.Errorf("install %s: record ownership: %w", name, err)
			}
		}
		action, err := LinkSkill(src, dst, destRoot, opts)
		if err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, fmt.Errorf("install %s: %w; rollback ownership: %v", name, err, rollbackErr)
			}
			return nil, fmt.Errorf("install %s: %w", name, err)
		}
		results = append(results, Result{Name: name, Source: src, Dest: dst, Action: action})
	}
	return results, nil
}

func codexDest(repoRoot, scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".agents", "skills"), nil
	case "repo":
		return filepath.Join(repoRoot, ".agents", "skills"), nil
	default:
		return "", fmt.Errorf("invalid codex scope %q (valid: user, repo)", scope)
	}
}

func resolveSkillNames(skillsDir string, requested []string) ([]string, error) {
	if len(requested) > 0 {
		for _, name := range requested {
			if err := assertSimpleName(name); err != nil {
				return nil, err
			}
			if _, err := os.Stat(filepath.Join(skillsDir, name, "SKILL.md")); err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("skill %q not found under %s", name, skillsDir)
				}
				return nil, err
			}
		}
		return requested, nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsDir, entry.Name(), "SKILL.md")); err == nil {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// dangerousDestPrefixes are absolute paths (and their subtrees) we refuse to
// install into, even with --force, to catch finger-fumbles like
// `--dest /etc`. Paths under $HOME or os.TempDir() are always allowed and
// skip this check, so this list focuses on system roots.
var dangerousDestPrefixes = []string{
	"/etc",
	"/usr",
	"/bin",
	"/sbin",
	"/boot",
	"/proc",
	"/sys",
	"/dev",
	"/System",
	"/Library",
	"/private/etc",
}

func assertSafeDest(dest string) error {
	abs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve dest %s: %w", dest, err)
	}
	abs = filepath.Clean(abs)
	if abs == "/" {
		return fmt.Errorf("refusing to install into filesystem root %q", abs)
	}
	// Always allow $HOME and the OS temp dir (covers `t.TempDir()` on macOS,
	// which lives under /var/folders).
	for _, allow := range allowedDestRoots() {
		if isWithin(abs, allow) {
			return nil
		}
	}
	for _, root := range dangerousDestPrefixes {
		if isWithin(abs, root) {
			return fmt.Errorf("refusing to install into system path %s; choose a path under $HOME or a temp dir", abs)
		}
	}
	return nil
}

func allowedDestRoots() []string {
	var roots []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if abs, err := filepath.Abs(home); err == nil {
			roots = append(roots, filepath.Clean(abs))
		}
	}
	if tmp := os.TempDir(); tmp != "" {
		if abs, err := filepath.Abs(tmp); err == nil {
			roots = append(roots, filepath.Clean(abs))
		}
	}
	return roots
}

func isWithin(abs, root string) bool {
	if abs == root {
		return true
	}
	return strings.HasPrefix(abs+string(filepath.Separator), root+string(filepath.Separator))
}

func assertSimpleName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid skill name %q", name)
	}
	return nil
}

func assertInsideRoot(path, repoRoot string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path %s: %w", path, err)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolve root %s: %w", repoRoot, err)
	}
	if abs != root && !strings.HasPrefix(abs+string(filepath.Separator), root+string(filepath.Separator)) {
		return fmt.Errorf("path %s escapes repo root %s", path, repoRoot)
	}
	return nil
}

// copyDir produces a self-contained snapshot of src under dst. Symlinks inside
// src are dereferenced (their target contents are copied) — the snapshot mode
// is meant to materialize a portable copy, so we trade link fidelity for
// portability. Broken symlinks surface as Open errors from copyFile.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

// copyFile copies src to dst, preserving the source's permission bits so
// executable scripts inside skills remain executable after a snapshot copy.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat src %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create dst %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	// Re-apply mode: O_CREATE honors umask, so executables can still lose
	// bits on systems with restrictive umask. Chmod is the belt-and-suspenders.
	return os.Chmod(dst, info.Mode().Perm())
}
