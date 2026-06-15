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

// statusLineEvent is the sentinel Event value routing a Hook to the statusLine
// key instead of the hooks{} map.
const statusLineEvent = "statusLine"

// codexNotifyEvent is the sentinel Event value for a Hook that targets Codex's
// ~/.codex/config.toml notify key — it never goes into settings.json.
const codexNotifyEvent = "codex.notify"

// skipsSettingsJSON reports whether a hook event is registered somewhere other
// than settings.json's hooks{} map (statusLine key, or Codex config).
func skipsSettingsJSON(event string) bool {
	return event == statusLineEvent || event == codexNotifyEvent
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

// RegisterHooks registers each non-lib hook in s, using match-and-replace so an
// existing registration of the same hook file is replaced rather than
// duplicated. A hook with Event=="statusLine" patches the statusLine key.
func RegisterHooks(s *Settings, hooks []Hook) {
	if s.Hooks == nil {
		s.Hooks = make(map[string][]HookEntry)
	}

	for _, h := range hooks {
		if h.Lib || h.Event == "" {
			continue // lib files are copied only; empty event has nothing to register
		}
		if h.Event == codexNotifyEvent {
			continue // wired into ~/.codex/config.toml, not settings.json
		}
		if h.Event == statusLineEvent {
			SetStatusLine(s, HookCommand(h))
			continue
		}

		cmd := HookCommand(h)
		// Pass 1: remove any existing HookItem whose command targets our file.
		cleanedEntries := removeHookCommand(s.Hooks[h.Event], h.File)

		// Pass 2: find or create the entry block matching our matcher.
		blockIdx := -1
		for i, e := range cleanedEntries {
			if e.Matcher == h.Matcher {
				blockIdx = i
				break
			}
		}

		item := HookItem{Type: "command", Command: cmd}
		if blockIdx >= 0 {
			cleanedEntries[blockIdx].Hooks = append([]HookItem{item}, cleanedEntries[blockIdx].Hooks...)
		} else {
			cleanedEntries = append([]HookEntry{
				{Matcher: h.Matcher, Hooks: []HookItem{item}},
			}, cleanedEntries...)
		}

		s.Hooks[h.Event] = cleanedEntries
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

// statusLineEntry is the JSON object written to settings.json "statusLine".
type statusLineEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// SetStatusLine patches the "statusLine" key in s with cmd using the raw bytes
// so that repeated calls are idempotent (same bytes in → same bytes out).
func SetStatusLine(s *Settings, cmd string) {
	if s.rawOrig == nil {
		s.rawOrig = []byte(`{}`)
	}
	entry := statusLineEntry{Type: "command", Command: cmd}
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

// UnregisterHooks removes the given hooks' commands from s. Matcher blocks that
// become empty are pruned; unmanaged hooks are untouched. A statusLine hook
// clears the statusLine key.
func UnregisterHooks(s *Settings, hooks []Hook) {
	for _, h := range hooks {
		if h.Lib {
			continue
		}
		if h.Event == codexNotifyEvent {
			continue // unwired from ~/.codex/config.toml, not settings.json
		}
		if h.Event == statusLineEvent {
			UnsetStatusLine(s)
			continue
		}
		if s.Hooks == nil {
			continue
		}
		if entries, ok := s.Hooks[h.Event]; ok {
			s.Hooks[h.Event] = removeHookCommand(entries, h.File)
			if len(s.Hooks[h.Event]) == 0 {
				delete(s.Hooks, h.Event)
			}
		}
	}
}

// IsManagedCommand reports whether cmd targets a file under ~/.claude/hooks.
// Best-effort, path-based heuristic (needs no manifest): a hook a user hand-
// placed in that directory would also match. Used only for an advisory "vd"
// badge in the inventory UI, so a false positive is cosmetic.
func IsManagedCommand(cmd string) bool {
	return strings.Contains(cmd, hooksPathMarker)
}

// IsRegistered reports whether all non-lib, non-statusLine hooks are present in s.
func IsRegistered(s *Settings, hooks []Hook) bool {
	for _, h := range hooks {
		if h.Lib || skipsSettingsJSON(h.Event) {
			continue
		}
		cmd := HookCommand(h)
		found := false
		for _, entry := range s.Hooks[h.Event] {
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
