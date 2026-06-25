package claudeconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/sjson"

	"github.com/vanducng/vd-cli/v2/internal/extension"
)

// ClaudeUserConfigPath returns ~/.claude.json (user-scope MCP servers).
func ClaudeUserConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

// ClaudeProjectConfigPath returns <repoRoot>/.mcp.json (project-scope).
func ClaudeProjectConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".mcp.json")
}

// claudeMCPServer builds the mcpServers."<name>" object for e. env values are
// never written (AUDIT-C1); the process inherits the environment.
func claudeMCPServer(e extension.Extension) map[string]any {
	if e.Transport == "http" {
		return map[string]any{"type": "http", "url": e.URL}
	}
	return map[string]any{
		"type":    "stdio",
		"command": e.Command,
		"args":    e.ResolvedArgs(),
	}
}

// readOrInitJSON reads path, or returns "{}" when absent. A present-but-invalid
// file is an error (don't clobber a file we can't understand).
func readOrInitJSON(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []byte("{}"), false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	if !json.Valid(data) {
		return nil, true, fmt.Errorf("%s is not valid JSON", path)
	}
	return data, true, nil
}

// RegisterClaudeMCP sets mcpServers."<name>" in the file at path (surgical sjson
// patch — other keys/formatting preserved). Backs up an existing file first.
func RegisterClaudeMCP(path string, e extension.Extension) error {
	data, existed, err := readOrInitJSON(path)
	if err != nil {
		return err
	}
	out, err := sjson.SetBytes(data, "mcpServers."+e.Name, claudeMCPServer(e))
	if err != nil {
		return fmt.Errorf("patch mcpServers: %w", err)
	}
	return writeClaudeConfigSafe(path, data, out, existed)
}

// UnregisterClaudeMCP removes mcpServers."<name>" from path. No-op if absent.
func UnregisterClaudeMCP(path, name string) error {
	data, existed, err := readOrInitJSON(path)
	if err != nil {
		return err
	}
	if !existed {
		return nil
	}
	out, err := sjson.DeleteBytes(data, "mcpServers."+name)
	if err != nil {
		return fmt.Errorf("delete mcpServers.%s: %w", name, err)
	}
	if string(out) == string(data) {
		return nil
	}
	return writeClaudeConfigSafe(path, data, out, existed)
}

// writeClaudeConfigSafe backs up an existing file, writes data, then verifies it
// parses as JSON; on failure it restores the original (AUDIT-C1/R2).
func writeClaudeConfigSafe(path string, original, data []byte, existed bool) error {
	if existed {
		backup := codexBackupName(path)
		if err := os.WriteFile(backup, original, 0o644); err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	if err := atomicWrite(path, data); err != nil {
		return err
	}
	if !json.Valid(data) {
		if existed {
			if rErr := os.WriteFile(path, original, 0o644); rErr != nil {
				return fmt.Errorf("write produced invalid JSON AND restore failed: %v", rErr)
			}
		}
		return fmt.Errorf("write produced invalid JSON for %s (restored)", path)
	}
	return nil
}
