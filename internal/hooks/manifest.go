package hooks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/vanducng/vd-cli/v2/internal/claudeconfig"
)

// knownEvents are the hook events a manifest may target.
var knownEvents = map[string]bool{
	"SessionStart": true, "SessionEnd": true, "UserPromptSubmit": true,
	"PreToolUse": true, "PostToolUse": true, "Stop": true,
	"SubagentStart": true, "SubagentStop": true, "Notification": true,
	"PreCompact": true, "PermissionRequest": true, "statusLine": true,
	"codex.notify": true, "codex.UserPromptSubmit": true,
}

// tomlHook mirrors one [[hook]] table in hooks.toml.
type tomlHook struct {
	File    string   `toml:"file"`
	Runtime string   `toml:"runtime"`
	Event   string   `toml:"event"`
	Matcher string   `toml:"matcher"`
	Args    []string `toml:"args"`
	Lib     bool     `toml:"lib"`
}

type tomlManifest struct {
	Hook []tomlHook `toml:"hook"`
}

// LoadManifest parses the hooks manifest TOML at path into a hook list.
// A missing file yields a clear error. Each non-lib hook requires a non-empty
// event; runtime must be one of "", "node", "python3".
func LoadManifest(path string) ([]claudeconfig.Hook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("hooks manifest not found: %s", path)
		}
		return nil, fmt.Errorf("read hooks manifest %s: %w", path, err)
	}

	// Strict decode so a typo'd field (e.g. `runtimes`) errors instead of
	// silently producing an empty value.
	var m tomlManifest
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse hooks manifest %s: %w", path, err)
	}

	hooks := make([]claudeconfig.Hook, 0, len(m.Hook))
	for i, h := range m.Hook {
		if h.File == "" {
			return nil, fmt.Errorf("%s: hook #%d has empty file", path, i+1)
		}
		// File is joined under the hooks dir on install — reject path traversal
		// and absolute paths (fs.ValidPath rejects "..", leading "/", and ".").
		if !fs.ValidPath(filepath.ToSlash(h.File)) {
			return nil, fmt.Errorf("%s: hook %q has an unsafe file path", path, h.File)
		}
		switch h.Runtime {
		case "", "node", "python3":
		default:
			return nil, fmt.Errorf("%s: hook %q has invalid runtime %q (valid: node, python3, or empty)", path, h.File, h.Runtime)
		}
		if !h.Lib {
			if h.Event == "" {
				return nil, fmt.Errorf("%s: hook %q needs an event (or set lib = true)", path, h.File)
			}
			if !knownEvents[h.Event] {
				return nil, fmt.Errorf("%s: hook %q has unknown event %q", path, h.File, h.Event)
			}
		}
		hooks = append(hooks, claudeconfig.Hook{
			File:    h.File,
			Runtime: h.Runtime,
			Event:   h.Event,
			Matcher: h.Matcher,
			Args:    h.Args,
			Lib:     h.Lib,
		})
	}
	return hooks, nil
}

// Files returns each unique hook file (lib and non-lib), suitable for
// InstallFrom. The same file may be registered for multiple events; it is
// copied once.
func Files(hooks []claudeconfig.Hook) []string {
	seen := make(map[string]bool, len(hooks))
	files := make([]string, 0, len(hooks))
	for _, h := range hooks {
		if seen[h.File] {
			continue
		}
		seen[h.File] = true
		files = append(files, h.File)
	}
	return files
}
