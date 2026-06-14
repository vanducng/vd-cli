package inventory

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxAssetBytes caps a single asset file read to bound memory on hostile content.
const maxAssetBytes = 1 << 20 // 1 MiB

// Adapter scans one agent's home directory for assets, tagging them with a
// platform. The same path conventions (skills/, agents/, commands/, rules/)
// cover Claude Code, Codex, and Cursor; missing dirs are simply skipped.
type Adapter struct {
	Platform string
	Home     string // the agent home dir (e.g. ~/.claude, ~/.agents, ~/.cursor)
}

// NewAdapter returns an adapter for a platform rooted at home.
func NewAdapter(platform, home string) Adapter {
	return Adapter{Platform: platform, Home: home}
}

// NewClaudeAdapter is a convenience constructor for the Claude Code platform.
func NewClaudeAdapter(claudeDir string) Adapter {
	return Adapter{Platform: platformClaude, Home: claudeDir}
}

// Scan enumerates skills, agents, commands, and rules under the home dir.
// Read-only — never writes. Symlinked skill dirs (Codex) are followed.
func (a Adapter) Scan() ([]Asset, error) {
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

// scanSkills reads skills/<name>/SKILL.md(.disabled). Entry may be a symlink.
func (a Adapter) scanSkills() ([]Asset, error) {
	dir := filepath.Join(a.Home, "skills")
	entries, err := readDirOrNil(dir)
	if err != nil {
		return nil, err
	}
	var out []Asset
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if !isDir(full) { // follows symlinks (Codex installs are symlinked dirs)
			continue
		}
		path, enabled := skillMarker(full)
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
			Platform: a.Platform,
		})
	}
	return out, nil
}

// scanFlat reads <dir>/*.md(.disabled) / *.mdc as the given asset type.
func (a Adapter) scanFlat(sub string, typ AssetType) ([]Asset, error) {
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
			Platform: a.Platform,
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

func isDir(p string) bool {
	info, err := os.Stat(p) // follows symlinks
	return err == nil && info.IsDir()
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
