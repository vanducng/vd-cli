package claudeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const codexConfigFile = "config.toml"

// notifyLineRe matches the single-line top-level `notify = [...]` assignment.
// The value lives on one line; we replace it surgically and leave every other
// byte untouched.
var notifyLineRe = regexp.MustCompile(`(?m)^\s*notify\s*=.*$`)

// CodexConfigPath returns the absolute path to ~/.codex/config.toml.
func CodexConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex", codexConfigFile), nil
}

// codexBackupName builds a one-shot backup path with a UTC timestamp, matching
// the hooks installer's backupName style.
func codexBackupName(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + ".bak." + time.Now().UTC().Format("20060102T150405") + ext
}

// tomlQuote returns a TOML basic string for s (double-quoted, backslash and
// double-quote escaped).
func tomlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// notifyArray renders command as a TOML array literal: ["a", "b", ...].
func notifyArray(command []string) string {
	parts := make([]string, len(command))
	for i, c := range command {
		parts[i] = tomlQuote(c)
	}
	return "notify = [" + strings.Join(parts, ", ") + "]"
}

// WireCodexNotify surgically sets the top-level notify key in the Codex config
// at path to command. A missing file is created with just the notify line.
// Every other line/comment is preserved byte-for-byte (no go-toml round-trip).
//
// If an existing notify line was present and does not already reference our
// command's program path, its full text is returned as replacedPrev so the
// caller can warn the user (they can chain it via CODEX_NOTIFY_FORWARD). If the
// existing line already targets our program, replacedPrev is "".
//
// The file is backed up once (path.bak.<UTC-ts>) before the atomic write.
func WireCodexNotify(path string, command []string) (replacedPrev string, err error) {
	if len(command) == 0 {
		return "", fmt.Errorf("codex notify command is empty")
	}
	programPath := codexProgramPath(command)
	newLine := notifyArray(command)

	existing, readErr := os.ReadFile(path)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return "", fmt.Errorf("read %s: %w", path, readErr)
		}
		// Missing file: create with just the notify line.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("create codex config dir: %w", err)
		}
		return "", atomicWrite(path, []byte(newLine+"\n"))
	}

	var out []byte
	if loc := notifyLineRe.FindIndex(existing); loc != nil {
		prev := strings.TrimRight(string(existing[loc[0]:loc[1]]), "\r")
		if !strings.Contains(prev, programPath) {
			replacedPrev = prev
		}
		out = append(out, existing[:loc[0]]...)
		out = append(out, []byte(newLine)...)
		out = append(out, existing[loc[1]:]...)
	} else {
		// No notify line — append one (keep a trailing newline tidy).
		out = append(out, existing...)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
		out = append(out, []byte(newLine+"\n")...)
	}

	backupPath := codexBackupName(path)
	if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
		return "", fmt.Errorf("backup %s: %w", path, err)
	}
	if err := atomicWrite(path, out); err != nil {
		return "", err
	}
	return replacedPrev, nil
}

// UnwireCodexNotify removes the managed notify line from path if it references
// programPath. No-op when the file is absent, has no notify line, or the line is
// not ours. The file is backed up once before any change.
func UnwireCodexNotify(path, programPath string) error {
	existing, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, readErr)
	}

	loc := notifyLineRe.FindIndex(existing)
	if loc == nil {
		return nil
	}
	line := string(existing[loc[0]:loc[1]])
	if !strings.Contains(line, programPath) {
		return nil // not ours — leave it
	}

	// Drop the whole line including its trailing newline (if any).
	end := loc[1]
	if end < len(existing) && existing[end] == '\n' {
		end++
	}
	out := append([]byte{}, existing[:loc[0]]...)
	out = append(out, existing[end:]...)

	backupPath := codexBackupName(path)
	if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
		return fmt.Errorf("backup %s: %w", path, err)
	}
	return atomicWrite(path, out)
}

// codexProgramPath returns the program element of a notify command — the runtime
// when one is present (argv[0] is python3/node, argv[1] is the script), else the
// first element. Used to detect whether an existing notify line is ours.
func codexProgramPath(command []string) string {
	if len(command) == 0 {
		return ""
	}
	switch command[0] {
	case "python3", "node":
		if len(command) > 1 {
			return command[1]
		}
	}
	return command[0]
}
