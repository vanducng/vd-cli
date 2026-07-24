package target

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const piManagedDir = ".vd-managed-skills"

type piEmitter struct{}

var renamePiPath = os.Rename

func (e *piEmitter) Name() string { return "pi" }

func (e *piEmitter) Emit(ctx Context) error {
	return emitPi(ctx, runtime.GOOS == "windows")
}

func emitPi(ctx Context, copyMode bool) error {
	skillsDir := filepath.Join(ctx.RepoRoot, "skills")
	piDir := filepath.Join(ctx.RepoRoot, ".pi")
	piSkillsDir := filepath.Join(piDir, "skills")
	managedDir := filepath.Join(piDir, piManagedDir)

	if err := rejectSymlinkPath(ctx.RepoRoot, piDir, piSkillsDir, managedDir); err != nil {
		return fmt.Errorf("pi emitter: %w", err)
	}
	desired, err := discoverSkillNames(skillsDir)
	if err != nil {
		return fmt.Errorf("pi emitter: discover skills: %w", err)
	}
	if err := os.MkdirAll(piSkillsDir, 0o755); err != nil {
		return fmt.Errorf("pi emitter: create skills dir: %w", err)
	}
	if err := os.MkdirAll(managedDir, 0o755); err != nil {
		return fmt.Errorf("pi emitter: create ownership dir: %w", err)
	}
	managed, err := readDroidManaged(managedDir)
	if err != nil {
		return fmt.Errorf("pi emitter: %w", err)
	}
	managedSet := make(map[string]bool, len(managed))
	for _, name := range managed {
		managedSet[name] = true
	}
	for _, name := range desired {
		dst := filepath.Join(piSkillsDir, name)
		if _, err := os.Lstat(dst); err == nil && !managedSet[name] {
			return fmt.Errorf("pi emitter: destination %s already exists and is not vd-managed", dst)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("pi emitter: inspect %s: %w", dst, err)
		}
	}

	desiredSet := make(map[string]bool, len(desired))
	for _, name := range desired {
		desiredSet[name] = true
		src := filepath.Join(skillsDir, name)
		dst := filepath.Join(piSkillsDir, name)
		markerCreated, err := ClaimPiSkill(ctx.RepoRoot, name)
		if err != nil {
			return fmt.Errorf("pi emitter: record ownership for %s: %w", name, err)
		}
		var publishErr error
		if copyMode {
			publishErr = replacePiCopy(piDir, src, dst)
		} else {
			rel, err := filepath.Rel(piSkillsDir, src)
			if err != nil {
				publishErr = fmt.Errorf("resolve link: %w", err)
			} else {
				publishErr = ensureSymlink(dst, rel)
			}
		}
		if publishErr != nil {
			if markerCreated {
				if unclaimErr := UnclaimPiSkill(ctx.RepoRoot, name); unclaimErr != nil {
					return fmt.Errorf("pi emitter: publish %s: %w; rollback ownership: %v", name, publishErr, unclaimErr)
				}
			}
			return fmt.Errorf("pi emitter: publish %s: %w", name, publishErr)
		}
	}

	for _, name := range managed {
		if desiredSet[name] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(piSkillsDir, name)); err != nil {
			return fmt.Errorf("pi emitter: remove stale %s: %w", name, err)
		}
		if err := os.Remove(filepath.Join(managedDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("pi emitter: remove ownership for %s: %w", name, err)
		}
	}
	return nil
}

// ClaimPiSkill keeps repo installs compatible with subsequent Pi builds.
func ClaimPiSkill(repoRoot, name string) (bool, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return false, fmt.Errorf("invalid skill name %q", name)
	}
	piDir := filepath.Join(repoRoot, ".pi")
	piSkillsDir := filepath.Join(piDir, "skills")
	managedDir := filepath.Join(piDir, piManagedDir)
	if err := rejectSymlinkPath(repoRoot, piDir, piSkillsDir, managedDir); err != nil {
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

// UnclaimPiSkill rolls back ownership when publishing a repo install fails.
func UnclaimPiSkill(repoRoot, name string) error {
	return os.Remove(filepath.Join(repoRoot, ".pi", piManagedDir, name))
}

func replacePiCopy(piDir, src, dst string) error {
	stageRoot, err := os.MkdirTemp(piDir, ".vd-pi-stage-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			_ = os.RemoveAll(stageRoot)
		}
	}()
	staged := filepath.Join(stageRoot, "new")
	if err := syncCopyDir(src, staged); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); os.IsNotExist(err) {
		return renamePiPath(staged, dst)
	} else if err != nil {
		return fmt.Errorf("inspect destination: %w", err)
	}
	backup := filepath.Join(stageRoot, "old")
	if err := renamePiPath(dst, backup); err != nil {
		return fmt.Errorf("stage old destination: %w", err)
	}
	if err := renamePiPath(staged, dst); err != nil {
		if restoreErr := renamePiPath(backup, dst); restoreErr != nil {
			cleanupStage = false
			return fmt.Errorf("replace destination: %w; restore failed, original preserved at %s: %v", err, backup, restoreErr)
		}
		return fmt.Errorf("replace destination: %w", err)
	}
	return nil
}
