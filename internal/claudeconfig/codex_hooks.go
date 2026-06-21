package claudeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const codexHooksFile = "hooks.json"

// CodexHooksPath returns the absolute path to ~/.codex/hooks.json.
func CodexHooksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex", codexHooksFile), nil
}

// CodexHooksDest returns the absolute path to ~/.codex/hooks.
func CodexHooksDest() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "hooks"), nil
}

// CodexEventName strips the codex. manifest prefix.
func CodexEventName(event string) (string, bool) {
	if !strings.HasPrefix(event, "codex.") || event == codexNotifyEvent {
		return "", false
	}
	name := strings.TrimPrefix(event, "codex.")
	return name, name != ""
}

// RegisterCodexHooks registers codex.* hooks in a Claude-compatible hooks.json.
func RegisterCodexHooks(s *Settings, hooks []Hook, hooksDir string) {
	if s.Hooks == nil {
		s.Hooks = make(map[string][]HookEntry)
	}

	for _, h := range hooks {
		event, ok := CodexEventName(h.Event)
		if h.Lib || !ok {
			continue
		}

		cmd := CodexHookCommand(h, hooksDir)
		cleanedEntries := removeHookCommand(s.Hooks[event], h.File)

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

		s.Hooks[event] = cleanedEntries
	}
}

// UnregisterCodexHooks removes codex.* hook commands from a hooks.json tree.
func UnregisterCodexHooks(s *Settings, hooks []Hook) {
	for _, h := range hooks {
		event, ok := CodexEventName(h.Event)
		if h.Lib || !ok || s.Hooks == nil {
			continue
		}
		if entries, exists := s.Hooks[event]; exists {
			s.Hooks[event] = removeHookCommand(entries, h.File)
			if len(s.Hooks[event]) == 0 {
				delete(s.Hooks, event)
			}
		}
	}
}
