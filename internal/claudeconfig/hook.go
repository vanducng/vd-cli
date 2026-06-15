package claudeconfig

import "strings"

// Hook describes one entry from the hooks manifest. A non-lib hook is registered
// in settings.json under its Event; a lib hook is copied only (support file).
type Hook struct {
	File    string   // path relative to the hooks dir, e.g. "session-init.cjs" or "lib/config.cjs"
	Runtime string   // "node" | "python3" | "" (direct exec via shebang)
	Event   string   // Claude event (SessionStart, Stop, ...) or "statusLine"
	Matcher string   // optional matcher
	Args    []string // extra argv appended after the file path
	Lib     bool     // true => support file only (copied, never registered)
}

// hooksPathMarker is the settings.json command substring identifying a command
// that targets a file under ~/.claude/hooks — i.e. one vd manages.
const hooksPathMarker = `$HOME/.claude/hooks/`

// HookCommand builds the settings.json command string for h:
//
//	<runtime> "$HOME/.claude/hooks/<File>" <Args...>
//
// $HOME stays literal — the Claude hook runner shell-expands it, avoiding a
// personal absolute path in the user's config. The runtime prefix is omitted
// when h.Runtime is empty (direct shebang exec).
func HookCommand(h Hook) string {
	cmd := `"$HOME/.claude/hooks/` + h.File + `"`
	if h.Runtime != "" {
		cmd = h.Runtime + " " + cmd
	}
	for _, a := range h.Args {
		cmd += " " + shellQuote(a)
	}
	return cmd
}

// shellMeta are characters that, unquoted, the shell would interpret.
const shellMeta = " \t\n\"'`$\\&|;<>()*?[]{}~#!="

// shellQuote returns a safe bare token unchanged, else POSIX single-quotes it
// (escaping embedded single quotes) so args with metacharacters cannot inject.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, shellMeta) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
