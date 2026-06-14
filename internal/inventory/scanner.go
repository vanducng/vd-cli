package inventory

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxAssetBytes caps a single asset file read to bound memory on hostile content.
const maxAssetBytes = 1 << 20 // 1 MiB

// ClaudeAdapter scans the path conventions of a ~/.claude directory.
type ClaudeAdapter struct {
	Home string // the .claude directory
}

// NewClaudeAdapter returns an adapter rooted at the given .claude dir.
func NewClaudeAdapter(claudeDir string) ClaudeAdapter {
	return ClaudeAdapter{Home: claudeDir}
}

// Scan enumerates skills, agents, commands, and rules under the .claude dir.
// Missing subdirectories are skipped, not errors. Read-only — never writes.
func (a ClaudeAdapter) Scan() ([]Asset, error) {
	out, err := a.scanSkills()
	if err != nil {
		return nil, err
	}
	for _, f := range []struct {
		dir string
		typ AssetType
	}{
		{"agents", Agent},
		{"commands", Command},
		{"rules", Rule},
	} {
		assets, err := a.scanFlat(f.dir, f.typ)
		if err != nil {
			return nil, err
		}
		out = append(out, assets...)
	}
	return out, nil
}

// scanSkills reads skills/<name>/SKILL.md(.disabled).
func (a ClaudeAdapter) scanSkills() ([]Asset, error) {
	dir := filepath.Join(a.Home, "skills")
	entries, err := readDirOrNil(dir)
	if err != nil {
		return nil, err
	}
	var out []Asset
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path, enabled := skillMarker(filepath.Join(dir, e.Name()))
		if path == "" {
			continue
		}
		fm, body, ok := parseFile(path)
		if !ok {
			continue
		}
		out = append(out, Asset{
			Type: Skill, Name: e.Name(), Description: Describe(fm),
			Enabled: enabled, Path: path, Frontmatter: fm, Body: body,
			Platform: platformClaude,
		})
	}
	return out, nil
}

// scanFlat reads <dir>/*.md(.disabled) / *.mdc as the given asset type.
func (a ClaudeAdapter) scanFlat(sub string, typ AssetType) ([]Asset, error) {
	dir := filepath.Join(a.Home, sub)
	entries, err := readDirOrNil(dir)
	if err != nil {
		return nil, err
	}
	var out []Asset
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base, enabled := enabledFromName(e.Name())
		if !strings.HasSuffix(base, ".md") && !strings.HasSuffix(base, ".mdc") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimSuffix(base, ".mdc"), ".md")
		path := filepath.Join(dir, e.Name())
		fm, body, ok := parseFile(path)
		if !ok {
			continue
		}
		out = append(out, Asset{
			Type: typ, Name: name, Description: Describe(fm),
			Enabled: enabled, Path: path, Frontmatter: fm, Body: body,
			Platform: platformClaude,
		})
	}
	return out, nil
}

// skillMarker returns the SKILL.md path and enablement, or "" if absent.
func skillMarker(skillDir string) (string, bool) {
	if p := filepath.Join(skillDir, "SKILL.md"); fileExists(p) {
		return p, true
	}
	if p := filepath.Join(skillDir, "SKILL.md.disabled"); fileExists(p) {
		return p, false
	}
	return "", false
}

// parseFile reads (size-capped) and frontmatter-parses a file. A read/parse
// error drops the asset rather than failing the whole scan.
func parseFile(path string) (map[string]any, string, bool) {
	data, err := readCapped(path, maxAssetBytes)
	if err != nil {
		return nil, "", false
	}
	fm, body, err := ParseFrontmatter(data)
	if err != nil {
		return nil, "", false
	}
	return fm, body, true
}

func readDirOrNil(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return entries, err
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func readCapped(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, max))
}
