package claudeconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── settings.go tests ──────────────────────────────────────────────────────

func TestRegisterHooksIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, `{"env":{"FOO":"bar"}}`)

	s1 := mustReadSettings(t, path)
	RegisterHooks(s1)
	if err := writeSettingsAt(path, s1, false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	data1 := mustReadFile(t, path)

	s2 := mustReadSettings(t, path)
	RegisterHooks(s2)
	if err := writeSettingsAt(path, s2, false); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data2 := mustReadFile(t, path)

	if string(data1) != string(data2) {
		t.Errorf("idempotency failed:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}

	// Both managed hooks must be present.
	if !IsRegistered(s2) {
		t.Error("IsRegistered returned false after two registrations")
	}
}

func TestRegisterHooksReplaceExisting(t *testing.T) {
	// Simulate ck's prior registration using the same file names.
	existing := `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {"type": "command", "command": "node \"$HOME/.claude/hooks/session-init.cjs\""},
          {"type": "command", "command": "node \"$HOME/.claude/hooks/other.cjs\""}
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "*",
        "hooks": [
          {"type": "command", "command": "node \"$HOME/.claude/hooks/subagent-init.cjs\""}
        ]
      }
    ]
  }
}`
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, existing)

	s := mustReadSettings(t, path)
	RegisterHooks(s)

	// Verify no duplication: session-init.cjs must appear exactly once.
	count := 0
	for _, entry := range s.Hooks["SessionStart"] {
		for _, item := range entry.Hooks {
			if strings.Contains(item.Command, "session-init.cjs") {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("session-init.cjs appears %d times in SessionStart (want 1)", count)
	}

	// other.cjs must be preserved.
	found := false
	for _, entry := range s.Hooks["SessionStart"] {
		for _, item := range entry.Hooks {
			if strings.Contains(item.Command, "other.cjs") {
				found = true
			}
		}
	}
	if !found {
		t.Error("other.cjs was removed — should have been preserved")
	}
}

func TestRegisterHooksLiteralDollarHOME(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, `{}`)

	s := mustReadSettings(t, path)
	RegisterHooks(s)
	if err := writeSettingsAt(path, s, false); err != nil {
		t.Fatalf("write: %v", err)
	}

	data := mustReadFile(t, path)

	// Must contain literal $HOME, not an expanded absolute path.
	if !strings.Contains(string(data), `$HOME`) {
		t.Error("settings.json does not contain literal $HOME")
	}
	if containsPersonalPath(data) {
		t.Error("settings.json contains a resolved absolute home path — $HOME must stay literal")
	}
}

func TestWriteSettingsBackupOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	writeFixture(t, path, `{"env":{}}`)

	s1 := mustReadSettings(t, path)
	RegisterHooks(s1)
	if err := writeSettingsAt(path, s1, false); err != nil {
		t.Fatalf("first write: %v", err)
	}

	backupPath := path + ".bak"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup not created after first write: %v", err)
	}
	backupContent := mustReadFile(t, backupPath)

	// Second write must not overwrite the backup.
	s2 := mustReadSettings(t, path)
	RegisterHooks(s2)
	if err := writeSettingsAt(path, s2, false); err != nil {
		t.Fatalf("second write: %v", err)
	}

	if string(mustReadFile(t, backupPath)) != string(backupContent) {
		t.Error("backup was overwritten on second run — backup must be created only once")
	}
}

func TestWriteSettingsMalformedRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, `{this is not valid json`)

	_, err := readSettingsAt(path)
	if err == nil {
		t.Fatal("expected error for malformed settings.json, got nil")
	}

	// Original must be intact.
	content := mustReadFile(t, path)
	if !strings.Contains(string(content), "not valid json") {
		t.Error("original file was modified despite malformed input")
	}
}

func TestWriteSettingsDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, `{"env":{}}`)
	originalContent := mustReadFile(t, path)

	s := mustReadSettings(t, path)
	RegisterHooks(s)
	if err := writeSettingsAt(path, s, true /* dryRun */); err != nil {
		t.Fatalf("dry-run write: %v", err)
	}

	// File must be unchanged after dry-run.
	if string(mustReadFile(t, path)) != string(originalContent) {
		t.Error("dry-run mutated the settings file")
	}
	// No backup created during dry-run.
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("dry-run created a backup file — it should not")
	}
}

func TestWriteSettingsAtomicNoPartial(t *testing.T) {
	// Write to a read-only directory to force a rename failure — the temp file
	// should be cleaned up and the original left untouched.
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	writeFixture(t, path, `{"env":{}}`)

	// Make the directory read-only so CreateTemp fails.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Skipf("cannot set dir read-only: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }()

	s := mustReadSettings(t, path)
	RegisterHooks(s)
	err := writeSettingsAt(path, s, false)
	if err == nil {
		t.Fatal("expected error writing to read-only dir, got nil")
	}

	// Original must still be the fixture content.
	if string(mustReadFile(t, path)) != `{"env":{}}` {
		t.Error("original file was modified despite write failure")
	}
}

func TestWriteSettingsPreservesUnknownKeys(t *testing.T) {
	fixture := `{"env":{"FOO":"bar"},"cleanupPeriodDays":30,"permissions":{"defaultMode":"auto"}}`
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, fixture)

	s := mustReadSettings(t, path)
	RegisterHooks(s)
	if err := writeSettingsAt(path, s, false); err != nil {
		t.Fatalf("write: %v", err)
	}

	data := mustReadFile(t, path)
	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	for _, key := range []string{"env", "cleanupPeriodDays", "permissions"} {
		if _, ok := out[key]; !ok {
			t.Errorf("key %q was lost after write", key)
		}
	}
}

// TestWriteSettingsKeyOrderPreserved is the key regression test: a settings.json
// with many top-level keys in a specific order must have EXACTLY that order
// preserved after RegisterHooks + WriteSettings. Only the "hooks" value should
// differ; every other key must be byte-identical to the original.
func TestWriteSettingsKeyOrderPreserved(t *testing.T) {
	// Deliberately non-alphabetical key order with an existing hooks block.
	fixture := `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "cleanupPeriodDays": 30,
  "env": {
    "FOO": "bar"
  },
  "permissions": {
    "defaultMode": "auto"
  },
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "node \"$HOME/.claude/hooks/session-init.cjs\""
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "node \"$HOME/.claude/hooks/subagent-init.cjs\""
          }
        ]
      }
    ]
  },
  "attribution": {
    "commit": ""
  }
}`
	path := filepath.Join(t.TempDir(), "settings.json")
	writeFixture(t, path, fixture)

	s := mustReadSettings(t, path)
	RegisterHooks(s)
	if err := writeSettingsAt(path, s, false); err != nil {
		t.Fatalf("write: %v", err)
	}

	result := string(mustReadFile(t, path))

	// Verify top-level key ORDER is preserved by checking line positions.
	keyLines := map[string]int{}
	for i, line := range strings.Split(result, "\n") {
		for _, k := range []string{"$schema", "cleanupPeriodDays", "env", "permissions", "hooks", "attribution"} {
			if strings.Contains(line, `"`+k+`"`) {
				keyLines[k] = i
			}
		}
	}
	orderChecks := []struct{ first, second string }{
		{"$schema", "cleanupPeriodDays"},
		{"cleanupPeriodDays", "env"},
		{"env", "permissions"},
		{"permissions", "hooks"},
		{"hooks", "attribution"},
	}
	for _, oc := range orderChecks {
		f, fok := keyLines[oc.first]
		s2, sok := keyLines[oc.second]
		if !fok || !sok {
			t.Errorf("key %q or %q not found in output", oc.first, oc.second)
			continue
		}
		if f >= s2 {
			t.Errorf("key order wrong: %q (line %d) should come before %q (line %d)", oc.first, f, oc.second, s2)
		}
	}

	// Non-hooks keys must be byte-identical to the fixture.
	for _, segment := range []string{
		`"$schema": "https://json.schemastore.org/claude-code-settings.json"`,
		`"cleanupPeriodDays": 30`,
		`"FOO": "bar"`,
		`"defaultMode": "auto"`,
		`"commit": ""`,
	} {
		if !strings.Contains(result, segment) {
			t.Errorf("non-hooks segment %q was mutated or lost", segment)
		}
	}

	// Our hooks must be registered (idempotent re-registration of already-present hooks).
	if !IsRegistered(s) {
		t.Error("IsRegistered returned false after write")
	}
}

// ── config.go tests ────────────────────────────────────────────────────────

func TestCKConfigPreservesCustomKeys(t *testing.T) {
	fixture := `{
  "codingLevel": -1,
  "statusline": "minimal",
  "privacyBlock": true,
  "plan": {"namingFormat": "{date}-{issue}-{slug}", "reportsDir": "reports"},
  "paths": {"plans": "plans", "docs": "docs"},
  "hooks": {"privacy-block": false},
  "kits": {"SomeKit": {"installedSettings": {}}}
}`
	path := filepath.Join(t.TempDir(), ".ck.json")
	writeFixture(t, path, fixture)

	cfg, err := readCKConfigAt(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	EnsureUmbrellaSlot(cfg)

	if err := writeCKConfigAt(path, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}

	data := mustReadFile(t, path)
	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	// All custom keys must be preserved.
	for _, key := range []string{"statusline", "privacyBlock", "kits", "codingLevel", "plan", "paths", "hooks"} {
		if _, ok := out[key]; !ok {
			t.Errorf("key %q lost after write", key)
		}
	}

	// paths block must exist (EnsureUmbrellaSlot).
	if cfg.Paths == nil {
		t.Error("paths block is nil after EnsureUmbrellaSlot")
	}
}

func TestCKConfigNoDollarHOMEInOutput(t *testing.T) {
	fixture := `{"plan":{"namingFormat":"{date}-{slug}"}}`
	path := filepath.Join(t.TempDir(), ".ck.json")
	writeFixture(t, path, fixture)

	cfg, err := readCKConfigAt(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	EnsureUmbrellaSlot(cfg)
	if err := writeCKConfigAt(path, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}

	data := mustReadFile(t, path)
	if containsPersonalPath(data) {
		t.Error(".ck.json contains a resolved absolute home path")
	}
}

func TestCKConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	cfg, err := readCKConfigAt(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("returned nil config for missing file")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func mustReadSettings(t *testing.T, path string) *Settings {
	t.Helper()
	s, err := readSettingsAt(path)
	if err != nil {
		t.Fatalf("readSettingsAt %s: %v", path, err)
	}
	return s
}
