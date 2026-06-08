package hooks

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const assetsPrefix = "assets"

// DefaultDest returns the default install destination (~/.claude/hooks).
func DefaultDest() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "hooks"), nil
}

// FileResult describes one file written (or skipped) by Install.
type FileResult struct {
	RelPath string
	Dest    string
	Action  string // "wrote", "unchanged", "backed-up+wrote"
}

// Install writes all embedded hook assets to dest, preserving the lib/
// subdirectory layout (assets/ prefix is stripped). Files are written with
// 0755 perms. Pre-existing files owned by us are overwritten; if the content
// differs, the original is backed up once before overwrite. Unknown files
// already in dest are never touched.
func Install(dest string) ([]FileResult, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("create hooks dir %s: %w", dest, err)
	}

	var results []FileResult

	err := fs.WalkDir(FS, assetsPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(assetsPrefix, path)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", path, err)
		}
		// Always use OS path separators for the destination.
		rel = filepath.FromSlash(rel)

		dstPath := filepath.Join(dest, rel)

		data, err := FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		action, err := writeFile(dstPath, data)
		if err != nil {
			return err
		}
		results = append(results, FileResult{RelPath: rel, Dest: dstPath, Action: action})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// writeFile writes data to dst. Returns "unchanged" if the existing content
// already matches, "backed-up+wrote" if we backed up a different existing file,
// or "wrote" for a clean new write.
func writeFile(dst string, data []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("create parent dir for %s: %w", dst, err)
	}

	existing, readErr := os.ReadFile(dst)
	if readErr == nil {
		if bytesEqual(existing, data) {
			return "unchanged", nil
		}
		// Back up the differing file once (no double-backup).
		backupPath := backupName(dst)
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := os.Rename(dst, backupPath); err != nil {
				return "", fmt.Errorf("backup %s -> %s: %w", dst, backupPath, err)
			}
		}
	} else if !os.IsNotExist(readErr) {
		return "", fmt.Errorf("stat %s: %w", dst, readErr)
	}

	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return "", fmt.Errorf("write %s: %w", dst, err)
	}

	if readErr == nil {
		return "backed-up+wrote", nil
	}
	return "wrote", nil
}

func backupName(dst string) string {
	ext := filepath.Ext(dst)
	base := strings.TrimSuffix(dst, ext)
	return base + ".bak." + time.Now().UTC().Format("20060102T150405") + ext
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

