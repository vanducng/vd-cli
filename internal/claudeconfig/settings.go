package claudeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/sjson"
)

const settingsFile = "settings.json"

// hookCommand returns the literal settings.json command string for a hook file.
// $HOME is kept literal — the Claude hook runner shell-expands it, and using
// a literal $HOME avoids baking a personal absolute path into the user's config.
func hookCommand(hookFile string) string {
	return `node "$HOME/.claude/hooks/` + hookFile + `"`
}

// managedHooks maps each Claude event to the hook .cjs file we register.
var managedHooks = []struct {
	event   string
	matcher string
	file    string
}{
	{event: "SessionStart", matcher: "startup|resume|clear|compact", file: "session-init.cjs"},
	{event: "SubagentStart", matcher: "*", file: "subagent-init.cjs"},
	{event: "UserPromptSubmit", matcher: "", file: "dev-rules-reminder.cjs"},
	{event: "PreToolUse", matcher: "Bash|Glob|Grep|Read|Edit|Write", file: "scout-block.cjs"},
	{event: "SubagentStart", matcher: "*", file: "team-context-inject.cjs"},
}

// HookEntry is one entry in a hooks event array in settings.json.
type HookEntry struct {
	Matcher string     `json:"matcher,omitempty"`
	Hooks   []HookItem `json:"hooks"`
}

// HookItem is one item in a HookEntry's hooks array.
type HookItem struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// Settings holds the parsed hooks subtree plus the original raw bytes.
// The raw bytes are the authoritative representation of everything we don't touch.
type Settings struct {
	Hooks   map[string][]HookEntry `json:"hooks,omitempty"`
	rawOrig []byte                 // original file bytes; nil for a new file
}

// settingsPath returns the absolute path to ~/.claude/settings.json.
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", settingsFile), nil
}

// ReadSettings reads ~/.claude/settings.json.
// Missing file → empty Settings with nil rawOrig (not an error).
func ReadSettings() (*Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}
	return readSettingsAt(path)
}

// ReadSettingsAt reads settings from an explicit path (used in tests).
func ReadSettingsAt(path string) (*Settings, error) {
	return readSettingsAt(path)
}

func readSettingsAt(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Settings{Hooks: make(map[string][]HookEntry)}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Validate JSON before anything else — refuse malformed input.
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s contains invalid JSON — refusing to proceed to avoid corruption", path)
	}

	s := &Settings{
		Hooks:   make(map[string][]HookEntry),
		rawOrig: data,
	}

	// Parse only the hooks subtree.
	var wrapper struct {
		Hooks map[string][]HookEntry `json:"hooks"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse hooks in %s: %w", path, err)
	}
	if wrapper.Hooks != nil {
		s.Hooks = wrapper.Hooks
	}

	return s, nil
}

// RegisterHooks ensures each managed hook has an entry in s.Hooks, using
// match-and-replace so an existing registration of the same hook file is
// replaced rather than duplicated.
func RegisterHooks(s *Settings) {
	if s.Hooks == nil {
		s.Hooks = make(map[string][]HookEntry)
	}

	for _, mh := range managedHooks {
		cmd := hookCommand(mh.file)
		entries := s.Hooks[mh.event]

		// Pass 1: remove any existing HookItem whose command targets our file.
		cleanedEntries := removeHookCommand(entries, mh.file)

		// Pass 2: find or create the entry block matching our matcher.
		blockIdx := -1
		for i, e := range cleanedEntries {
			if e.Matcher == mh.matcher {
				blockIdx = i
				break
			}
		}

		item := HookItem{Type: "command", Command: cmd}

		if blockIdx >= 0 {
			cleanedEntries[blockIdx].Hooks = append([]HookItem{item}, cleanedEntries[blockIdx].Hooks...)
		} else {
			cleanedEntries = append([]HookEntry{
				{Matcher: mh.matcher, Hooks: []HookItem{item}},
			}, cleanedEntries...)
		}

		s.Hooks[mh.event] = cleanedEntries
	}
}

// removeHookCommand removes any HookItem whose command references hookFile
// from every entry in entries. Empty entry blocks are pruned.
func removeHookCommand(entries []HookEntry, hookFile string) []HookEntry {
	out := make([]HookEntry, 0, len(entries))
	for _, e := range entries {
		filtered := make([]HookItem, 0, len(e.Hooks))
		for _, item := range e.Hooks {
			if !strings.Contains(item.Command, hookFile) {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			e.Hooks = filtered
			out = append(out, e)
		}
	}
	return out
}

// WriteOptions controls the write behavior.
type WriteOptions struct {
	// Path overrides the default ~/.claude/settings.json (used in tests).
	Path string
	// DryRun prints the diff without writing.
	DryRun bool
}

// WriteSettings atomically persists s to ~/.claude/settings.json (or opts.Path).
// It patches ONLY the hooks key in the raw file — all other keys stay
// byte-for-byte identical in their original positions.
// Backs up the original file once before the first mutation.
// On DryRun it prints the diff to stdout and returns nil.
func WriteSettings(s *Settings, opts WriteOptions) error {
	path := opts.Path
	if path == "" {
		var err error
		path, err = settingsPath()
		if err != nil {
			return err
		}
	}
	return writeSettingsAt(path, s, opts.DryRun)
}

func writeSettingsAt(path string, s *Settings, dryRun bool) error {
	newData, err := buildSettingsBytes(s)
	if err != nil {
		return err
	}

	existing, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", path, readErr)
	}

	if dryRun {
		printDiff(path, existing, newData)
		return nil
	}

	// Backup the original once (skip if backup already exists).
	if readErr == nil {
		backupPath := path + ".bak"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
				return fmt.Errorf("backup %s: %w", path, err)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	return atomicWrite(path, newData)
}

// buildSettingsBytes produces the final file bytes.
//
// Strategy: use sjson to splice only the "hooks" key into the original raw
// bytes, leaving every other key byte-for-byte in its original position.
// The injected hooks value is formatted to match the original file's
// indentation so the result stays readable. Indentation is detected once
// from the original bytes and applied consistently on every write, so
// repeated write-read-write cycles produce identical output (idempotent).
func buildSettingsBytes(s *Settings) ([]byte, error) {
	base := s.rawOrig
	if len(base) == 0 {
		base = []byte(`{}`)
	}

	// Detect the file's indentation unit from the first indented line.
	// Falls back to 2 spaces (Claude Code's default) if detection fails.
	indent := detectIndent(base)

	var hooksJSON []byte
	var err error
	if indent != "" {
		// Pretty-print hooks value using the file's own indent unit.
		// The prefix equals one level of indent (top-level value depth).
		hooksJSON, err = json.MarshalIndent(s.Hooks, indent, indent)
	} else {
		hooksJSON, err = json.Marshal(s.Hooks)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal hooks: %w", err)
	}

	// sjson.SetRawBytes patches only "hooks", leaving every other byte intact.
	patched, err := sjson.SetRawBytes(base, "hooks", hooksJSON)
	if err != nil {
		return nil, fmt.Errorf("patch hooks key: %w", err)
	}

	if !bytes.HasSuffix(patched, []byte("\n")) {
		patched = append(patched, '\n')
	}
	return patched, nil
}

// detectIndent returns the indentation unit (e.g. "  " for 2-space) used by
// data, by scanning the first line that starts with whitespace.
// Returns "" if the file appears to be single-line / compact JSON.
func detectIndent(data []byte) string {
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		n := 0
		for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
			n++
		}
		if n > 0 {
			return string(line[:n])
		}
	}
	return ""
}

// printDiff prints a simple line-level diff of old vs new to stdout.
func printDiff(path string, old, new []byte) {
	oldLines := splitLines(old)
	newLines := splitLines(new)

	fmt.Printf("--- %s (current)\n", path)
	fmt.Printf("+++ %s (proposed)\n", path)

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}
	for i := 0; i < maxLen; i++ {
		var ol, nl string
		if i < len(oldLines) {
			ol = oldLines[i]
		}
		if i < len(newLines) {
			nl = newLines[i]
		}
		if ol != nl {
			if ol != "" {
				fmt.Printf("- %s\n", ol)
			}
			if nl != "" {
				fmt.Printf("+ %s\n", nl)
			}
		}
	}
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// atomicWrite writes data to path via a temp file + rename.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".vd-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp -> %s: %w", path, err)
	}

	ok = true
	return nil
}

// statusLineCommand is the literal command written to the statusLine key.
const statusLineCommand = `node "$HOME/.claude/hooks/statusline.cjs"`

// statusLineEntry is the JSON object written to settings.json "statusLine".
type statusLineEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// SetStatusLine patches the "statusLine" key in s using the raw bytes so that
// repeated calls are idempotent (same bytes in → same bytes out).
func SetStatusLine(s *Settings) {
	if s.rawOrig == nil {
		s.rawOrig = []byte(`{}`)
	}
	entry := statusLineEntry{Type: "command", Command: statusLineCommand}
	entryJSON, _ := json.Marshal(entry)
	patched, err := sjson.SetRawBytes(s.rawOrig, "statusLine", entryJSON)
	if err == nil {
		s.rawOrig = patched
	}
}

// UnsetStatusLine removes the "statusLine" key from s using the raw bytes.
func UnsetStatusLine(s *Settings) {
	if s.rawOrig == nil {
		return
	}
	patched, err := sjson.DeleteBytes(s.rawOrig, "statusLine")
	if err == nil {
		s.rawOrig = patched
	}
}

// UnregisterHooks removes only our managed hook commands from s.Hooks.
// Matcher blocks that become empty are pruned; unmanaged hooks are untouched.
func UnregisterHooks(s *Settings) {
	if s.Hooks == nil {
		return
	}
	for _, mh := range managedHooks {
		if entries, ok := s.Hooks[mh.event]; ok {
			s.Hooks[mh.event] = removeHookCommand(entries, mh.file)
			if len(s.Hooks[mh.event]) == 0 {
				delete(s.Hooks, mh.event)
			}
		}
	}
}

// IsManagedCommand reports whether cmd references one of vd's managed hook files.
func IsManagedCommand(cmd string) bool {
	for _, mh := range managedHooks {
		if strings.Contains(cmd, mh.file) {
			return true
		}
	}
	return false
}

// IsRegistered reports whether all managed hooks are present in s.
func IsRegistered(s *Settings) bool {
	for _, mh := range managedHooks {
		cmd := hookCommand(mh.file)
		found := false
		for _, entry := range s.Hooks[mh.event] {
			for _, item := range entry.Hooks {
				if item.Command == cmd {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// DiffSettings returns the proposed serialized bytes without writing.
func DiffSettings(s *Settings) ([]byte, error) {
	return buildSettingsBytes(s)
}

// containsPersonalPath returns true if data contains a hardcoded absolute home
// path — used as a safety assertion in tests.
func containsPersonalPath(data []byte) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(home))
}
