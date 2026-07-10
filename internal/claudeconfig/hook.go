package claudeconfig

import (
	"path/filepath"
	"strings"
)

// Hook describes one entry from the hooks manifest. A non-lib hook is registered
// in settings.json under its Event; a lib hook is copied only (support file).
type Hook struct {
	File    string   // path relative to the hooks dir, e.g. "session-init.py" or "lib/config.py"
	Runtime string   // "node" | "python3" | "uv" | "" (direct exec via shebang)
	Event   string   // Claude event (SessionStart, Stop, ...), "statusLine", or "codex.*"
	Matcher string   // optional matcher
	Args    []string // extra argv appended after the file path
	Lib     bool     // true => support file only (copied, never registered)
}

// hooksPathMarker is the settings.json command substring identifying a command
// that targets a file under ~/.claude/hooks — i.e. one vd manages.
const hooksPathMarker = `$HOME/.claude/hooks/`

// runtimePrefix returns the shell tokens that precede the hook path for a
// runtime: "uv" expands to "uv run" (uv execs the script as a subcommand),
// "node"/"python3" are their own token, and "" yields no prefix (direct
// shebang exec).
func runtimePrefix(runtime string) string {
	switch runtime {
	case "":
		return ""
	case "uv":
		return "uv run"
	default:
		return runtime
	}
}

// runtimeArgv returns runtimePrefix split into argv tokens for exec-array
// builders (["uv","run"], ["node"], nil, ...).
func runtimeArgv(runtime string) []string {
	switch runtime {
	case "":
		return nil
	case "uv":
		return []string{"uv", "run"}
	default:
		return []string{runtime}
	}
}

// HookCommand builds the settings.json command string for h:
//
//	<runtime-prefix> "$HOME/.claude/hooks/<File>" <Args...>
//
// $HOME stays literal — the Claude hook runner shell-expands it, avoiding a
// personal absolute path in the user's config. The runtime prefix is omitted
// when h.Runtime is empty (direct shebang exec) and is "uv run" when h.Runtime
// is "uv".
func HookCommand(h Hook) string {
	cmd := `"$HOME/.claude/hooks/` + h.File + `"`
	if prefix := runtimePrefix(h.Runtime); prefix != "" {
		cmd = prefix + " " + cmd
	}
	for _, a := range h.Args {
		cmd += " " + shellQuote(a)
	}
	return cmd
}

// CodexNotifyCommand builds the exec array Codex runs for a codex.notify hook.
// Codex execs the program DIRECTLY (no shell), so the hook path must be absolute
// (joined under hooksDir, the real install dest — not the $HOME-literal form).
// The runtime prefix tokens are prepended when set ("uv" yields "uv","run");
// otherwise the absolute path is argv[0]. Args are appended.
func CodexNotifyCommand(h Hook, hooksDir string) []string {
	abs := filepath.Join(hooksDir, filepath.FromSlash(h.File))
	prefix := runtimeArgv(h.Runtime)
	cmd := make([]string, 0, len(prefix)+1+len(h.Args))
	cmd = append(cmd, prefix...)
	cmd = append(cmd, abs)
	cmd = append(cmd, h.Args...)
	return cmd
}

// CodexHookCommand builds the shell command string stored in ~/.codex/hooks.json.
func CodexHookCommand(h Hook, hooksDir string) string {
	abs := filepath.Join(hooksDir, filepath.FromSlash(h.File))
	cmd := singleQuote(abs)
	if prefix := runtimePrefix(h.Runtime); prefix != "" {
		cmd = prefix + " " + cmd
	}
	for _, a := range h.Args {
		cmd += " " + shellQuote(a)
	}
	return cmd
}

func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellMeta are characters that, unquoted, the shell would interpret.
const shellMeta = " \t\n\"'`$\\&|;<>()*?[]{}~#!="

// shellQuote returns a safe bare token unchanged, else POSIX single-quotes it
// (escaping embedded single quotes) so args with metacharacters cannot inject.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, shellMeta) {
		return s
	}
	return singleQuote(s)
}
