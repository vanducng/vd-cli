package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Discover parses every extensions/<name>/extension.toml under repoRoot and
// returns them sorted by name. A missing extensions/ dir yields no extensions
// (not an error). Directories without an extension.toml are skipped.
func Discover(repoRoot string) ([]Extension, error) {
	root := filepath.Join(repoRoot, "extensions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read extensions dir %s: %w", root, err)
	}

	var exts []Extension
	seen := make(map[string]bool)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(root, ent.Name())
		manifest := filepath.Join(dir, "extension.toml")
		if _, statErr := os.Stat(manifest); statErr != nil {
			continue
		}
		e, loadErr := LoadExtension(manifest)
		if loadErr != nil {
			return nil, loadErr
		}
		if seen[e.Name] {
			return nil, fmt.Errorf("duplicate extension name %q (in %s)", e.Name, dir)
		}
		seen[e.Name] = true
		e.Dir = dir
		exts = append(exts, e)
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].Name < exts[j].Name })
	return exts, nil
}

// Find returns the extension with the given name, or an error if absent.
func Find(repoRoot, name string) (Extension, error) {
	exts, err := Discover(repoRoot)
	if err != nil {
		return Extension{}, err
	}
	for _, e := range exts {
		if e.Name == name {
			return e, nil
		}
	}
	return Extension{}, fmt.Errorf("extension %q not found under %s/extensions", name, repoRoot)
}
