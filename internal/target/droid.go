package target

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const droidManagedDir = ".vd-managed-skills"

type droidEmitter struct{}

func (e *droidEmitter) Name() string { return "droid" }

func (e *droidEmitter) Emit(ctx Context) error {
	return emitDroid(ctx, runtime.GOOS == "windows")
}

func emitDroid(ctx Context, copyMode bool) error {
	skillsDir := filepath.Join(ctx.RepoRoot, "skills")
	factoryDir := filepath.Join(ctx.RepoRoot, ".factory")
	droidDir := filepath.Join(factoryDir, "skills")
	managedDir := filepath.Join(factoryDir, droidManagedDir)

	if err := rejectSymlinkPath(ctx.RepoRoot, factoryDir, droidDir, managedDir); err != nil {
		return fmt.Errorf("droid emitter: %w", err)
	}
	desired, err := discoverSkillNames(skillsDir)
	if err != nil {
		return fmt.Errorf("droid emitter: discover skills: %w", err)
	}
	if err := os.MkdirAll(droidDir, 0o755); err != nil {
		return fmt.Errorf("droid emitter: create skills dir: %w", err)
	}
	if err := os.MkdirAll(managedDir, 0o755); err != nil {
		return fmt.Errorf("droid emitter: create ownership dir: %w", err)
	}
	managed, err := readDroidManaged(managedDir)
	if err != nil {
		return fmt.Errorf("droid emitter: %w", err)
	}
	managedSet := make(map[string]bool, len(managed))
	for _, name := range managed {
		managedSet[name] = true
	}
	for _, name := range desired {
		dst := filepath.Join(droidDir, name)
		if _, err := os.Lstat(dst); err == nil && !managedSet[name] {
			return fmt.Errorf("droid emitter: destination %s already exists and is not vd-managed", dst)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("droid emitter: inspect %s: %w", dst, err)
		}
	}

	desiredSet := make(map[string]bool, len(desired))
	for _, name := range desired {
		desiredSet[name] = true
		src := filepath.Join(skillsDir, name)
		dst := filepath.Join(droidDir, name)
		markerCreated, err := ClaimDroidSkill(ctx.RepoRoot, name)
		if err != nil {
			return fmt.Errorf("droid emitter: record ownership for %s: %w", name, err)
		}
		var publishErr error
		if copyMode {
			publishErr = replaceDroidCopy(factoryDir, src, dst)
		} else {
			rel, err := filepath.Rel(droidDir, src)
			if err != nil {
				publishErr = fmt.Errorf("resolve link: %w", err)
			} else {
				publishErr = ensureSymlink(dst, rel)
			}
		}
		if publishErr != nil {
			if markerCreated {
				_ = UnclaimDroidSkill(ctx.RepoRoot, name)
			}
			return fmt.Errorf("droid emitter: publish %s: %w", name, publishErr)
		}
	}

	for _, name := range managed {
		if desiredSet[name] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(droidDir, name)); err != nil {
			return fmt.Errorf("droid emitter: remove stale %s: %w", name, err)
		}
		if err := os.Remove(filepath.Join(managedDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("droid emitter: remove ownership for %s: %w", name, err)
		}
	}
	return nil
}

// ClaimDroidSkill keeps repo installs compatible with subsequent Droid builds.
func ClaimDroidSkill(repoRoot, name string) (bool, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return false, fmt.Errorf("invalid skill name %q", name)
	}
	factoryDir := filepath.Join(repoRoot, ".factory")
	droidDir := filepath.Join(factoryDir, "skills")
	managedDir := filepath.Join(factoryDir, droidManagedDir)
	if err := rejectSymlinkPath(repoRoot, factoryDir, droidDir, managedDir); err != nil {
		return false, err
	}
	if err := os.MkdirAll(managedDir, 0o755); err != nil {
		return false, fmt.Errorf("create ownership dir: %w", err)
	}
	marker := filepath.Join(managedDir, name)
	file, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if os.IsExist(err) {
		info, statErr := os.Lstat(marker)
		if statErr != nil {
			return false, statErr
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("ownership marker %s is not a regular file", name)
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(marker)
		return false, err
	}
	return true, nil
}

// UnclaimDroidSkill rolls back ownership when publishing a repo install fails.
func UnclaimDroidSkill(repoRoot, name string) error {
	return os.Remove(filepath.Join(repoRoot, ".factory", droidManagedDir, name))
}

func rejectSymlinkPath(repoRoot string, paths ...string) error {
	for _, path := range paths {
		if err := assertInsideRoot(path, repoRoot); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlinked destination path %s", path)
		}
	}
	return nil
}

func readDroidManaged(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read ownership dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect ownership marker %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("ownership marker %s is not a regular file", entry.Name())
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

func replaceDroidCopy(factoryDir, src, dst string) error {
	stageRoot, err := os.MkdirTemp(factoryDir, ".vd-droid-stage-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stageRoot) }()
	staged := filepath.Join(stageRoot, "new")
	if err := syncCopyDir(src, staged); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); os.IsNotExist(err) {
		return os.Rename(staged, dst)
	} else if err != nil {
		return fmt.Errorf("inspect destination: %w", err)
	}
	backup := filepath.Join(stageRoot, "old")
	if err := os.Rename(dst, backup); err != nil {
		return fmt.Errorf("stage old destination: %w", err)
	}
	if err := os.Rename(staged, dst); err != nil {
		if restoreErr := os.Rename(backup, dst); restoreErr != nil {
			return fmt.Errorf("replace destination: %w; restore failed: %v", err, restoreErr)
		}
		return fmt.Errorf("replace destination: %w", err)
	}
	return nil
}
