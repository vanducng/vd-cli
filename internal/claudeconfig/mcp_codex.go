package claudeconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/vanducng/vd-cli/v2/internal/extension"
)

// codexMCPBlock renders the `[mcp_servers.<name>]` table for e. env is never
// emitted — secrets stay out of config; the spawned process inherits the
// environment (see ADR-0001 / AUDIT-C1).
func codexMCPBlock(e extension.Extension) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", e.Name)
	if e.Transport == "http" {
		fmt.Fprintf(&b, "url = %s\n", tomlQuote(e.URL))
		return b.String()
	}
	fmt.Fprintf(&b, "command = %s\n", tomlQuote(e.Command))
	args := e.ResolvedArgs()
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = tomlQuote(a)
	}
	fmt.Fprintf(&b, "args = [%s]\n", strings.Join(parts, ", "))
	if e.StartupTimeoutSec > 0 {
		fmt.Fprintf(&b, "startup_timeout_sec = %d\n", e.StartupTimeoutSec)
	}
	return b.String()
}

// findCodexMCPBlock returns the [start,end) byte range of the
// `[mcp_servers.<name>]` table (and any of its sub-tables) in src, or (-1,-1)
// if absent. The block ends at the next table header that is not a sub-table of
// <name>, or at EOF. Byte offsets (not line indices) so trailing newlines are
// preserved exactly — replace and append stay idempotent.
func findCodexMCPBlock(src []byte, name string) (start, end int) {
	header := "[mcp_servers." + name + "]"
	subPrefix := "[mcp_servers." + name + "."
	start = -1
	for pos := 0; pos < len(src); {
		lineEnd := len(src)
		if nl := bytes.IndexByte(src[pos:], '\n'); nl >= 0 {
			lineEnd = pos + nl + 1
		}
		line := strings.TrimSpace(string(src[pos:lineEnd]))
		if start == -1 {
			if line == header {
				start = pos
			}
		} else if strings.HasPrefix(line, "[") && line != header && !strings.HasPrefix(line, subPrefix) {
			return start, pos
		}
		pos = lineEnd
	}
	if start == -1 {
		return -1, -1
	}
	return start, len(src)
}

// setCodexMCP returns src with the named server's block appended or replaced in
// place by block (which ends with a newline).
func setCodexMCP(src []byte, name, block string) []byte {
	start, end := findCodexMCPBlock(src, name)
	if start == -1 {
		base := strings.TrimRight(string(src), "\n")
		if base != "" {
			base += "\n\n"
		}
		return []byte(base + block)
	}
	out := make([]byte, 0, len(src)+len(block))
	out = append(out, src[:start]...)
	out = append(out, block...)
	out = append(out, src[end:]...)
	return out
}

// removeCodexMCP returns src with the named server's block removed (no-op if
// absent), swallowing the blank separator line left in front of it.
func removeCodexMCP(src []byte, name string) []byte {
	start, end := findCodexMCPBlock(src, name)
	if start == -1 {
		return src
	}
	pre := src[:start]
	if bytes.HasSuffix(pre, []byte("\n\n")) {
		pre = pre[:len(pre)-1]
	}
	out := make([]byte, 0, len(pre)+len(src)-end)
	out = append(out, pre...)
	out = append(out, src[end:]...)
	return out
}

// writeCodexConfigSafe backs up path, writes data, then re-parses it as TOML.
// If the result does not parse, it restores the backup and returns an error —
// the live config is never left broken (AUDIT-C1/R2).
func writeCodexConfigSafe(path string, original, data []byte) error {
	backup := codexBackupName(path)
	if err := os.WriteFile(backup, original, 0o644); err != nil {
		return fmt.Errorf("backup %s: %w", path, err)
	}
	if err := atomicWrite(path, data); err != nil {
		return err
	}
	if err := toml.Unmarshal(data, &map[string]any{}); err != nil {
		if rErr := os.WriteFile(path, original, 0o644); rErr != nil {
			return fmt.Errorf("write produced invalid TOML AND restore failed (%v); backup at %s: %w", rErr, backup, err)
		}
		return fmt.Errorf("write produced invalid TOML, restored original from %s: %w", backup, err)
	}
	return nil
}

// RegisterCodexMCP inserts or replaces the [mcp_servers.<name>] table for e in
// the Codex config at path. A missing file is created. Idempotent.
func RegisterCodexMCP(path string, e extension.Extension) error {
	block := codexMCPBlock(e)
	original, readErr := os.ReadFile(path)
	if readErr != nil {
		if !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create codex config dir: %w", err)
		}
		original = nil
	}
	out := setCodexMCP(original, e.Name, block)
	if bytes.Equal(out, original) {
		return nil
	}
	return writeCodexConfigSafe(path, original, out)
}

// UnregisterCodexMCP removes the [mcp_servers.<name>] table from path. No-op if
// the file or table is absent.
func UnregisterCodexMCP(path, name string) error {
	original, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, readErr)
	}
	out := removeCodexMCP(original, name)
	if bytes.Equal(out, original) {
		return nil
	}
	return writeCodexConfigSafe(path, original, out)
}
