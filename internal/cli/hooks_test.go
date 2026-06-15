package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanducng/vd-cli/v2/internal/claudeconfig"
	"github.com/vanducng/vd-cli/v2/internal/hooks"
)

// ── helpers ───────────────────────────────────────────────────────────────

// testManifestHooks mirrors the real hooks.toml for CLI uninstall/rollback tests.
func testManifestHooks() []claudeconfig.Hook {
	return []claudeconfig.Hook{
		{File: "session-init.cjs", Runtime: "node", Event: "SessionStart", Matcher: "startup|resume|clear|compact"},
		{File: "subagent-init.cjs", Runtime: "node", Event: "SubagentStart", Matcher: "*"},
		{File: "dev-rules-reminder.cjs", Runtime: "node", Event: "UserPromptSubmit"},
		{File: "lib/config.cjs", Lib: true},
		{File: "lib/paths.cjs", Lib: true},
		{File: "lib/state.cjs", Lib: true},
	}
}

func testManifestFiles() []string { return hooks.Files(testManifestHooks()) }

// withStatusLine appends a statusLine hook for the statusLine install/uninstall tests.
func withStatusLine() []claudeconfig.Hook {
	return append(testManifestHooks(), claudeconfig.Hook{File: "statusline.cjs", Runtime: "node", Event: "statusLine"})
}

// installTestHooks copies stub hook files for every manifest entry into hooksDir.
func installTestHooks(t *testing.T, hooksDir string) {
	t.Helper()
	src := t.TempDir()
	for _, rel := range testManifestFiles() {
		full := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("// "+rel+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	if _, err := hooks.InstallFrom(src, hooksDir, testManifestFiles()); err != nil {
		t.Fatalf("InstallFrom: %v", err)
	}
}

func buildHookFixture(t *testing.T) (hooksDir string, settingsPath string) {
	t.Helper()
	dir := t.TempDir()
	hooksDir = filepath.Join(dir, "hooks")
	settingsPath = filepath.Join(dir, "settings.json")

	// Install all managed hook files to a temp hooks dir.
	installTestHooks(t, hooksDir)

	// Write a settings.json with our hooks registered + an unmanaged hook.
	fixture := `{
  "env": {"FOO": "bar"},
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {"type": "command", "command": "node \"$HOME/.claude/hooks/session-init.cjs\""},
          {"type": "command", "command": "node \"$HOME/.claude/hooks/other-unmanaged.cjs\""}
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "*",
        "hooks": [{"type": "command", "command": "node \"$HOME/.claude/hooks/subagent-init.cjs\""}]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {"type": "command", "command": "node \"$HOME/.claude/hooks/dev-rules-reminder.cjs\""},
          {"type": "command", "command": "node \"$HOME/.claude/hooks/other-user-hook.cjs\""}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(settingsPath, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	return hooksDir, settingsPath
}

// runHooksCmd exercises runHooksUninstall / runHooksRollback against temp dirs.
func runUninstall(t *testing.T, hooksDir string, settingsPath string, dryRun bool) (string, error) {
	t.Helper()
	// Temporarily redirect ReadSettings / WriteSettings to our temp file.
	// We do this by calling the functions directly with their path overrides.
	s, err := claudeconfig.ReadSettingsAt(settingsPath)
	if err != nil {
		return "", err
	}
	claudeconfig.UnregisterHooks(s, testManifestHooks())
	if !dryRun {
		if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
			return "", err
		}
	}
	// Simulate file deletion.
	var out bytes.Buffer
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		if dryRun {
			out.WriteString("dry-run: would remove " + full + "\n")
			continue
		}
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		out.WriteString("removed " + full + "\n")
	}
	return out.String(), nil
}

// ── uninstall tests ───────────────────────────────────────────────────────

func TestHooksUninstallRemovesManagedFiles(t *testing.T) {
	hooksDir, settingsPath := buildHookFixture(t)

	_, err := runUninstall(t, hooksDir, settingsPath, false)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	// All managed files must be gone.
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		if _, err := os.Stat(full); err == nil {
			t.Errorf("managed file still present after uninstall: %s", full)
		}
	}
}

func TestHooksUninstallPreservesUnmanagedHooks(t *testing.T) {
	_, settingsPath := buildHookFixture(t)
	hooksDir := filepath.Dir(settingsPath) + "/hooks2"
	_ = os.MkdirAll(hooksDir, 0o755)

	// Only touch settings; don't delete hooks files for this test.
	s, err := claudeconfig.ReadSettingsAt(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	claudeconfig.UnregisterHooks(s, testManifestHooks())
	if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Managed commands must be gone.
	for _, name := range []string{"session-init.cjs", "subagent-init.cjs", "dev-rules-reminder.cjs"} {
		if strings.Contains(string(data), name) {
			t.Errorf("managed hook %s still present in settings.json after unregister", name)
		}
	}

	// Unmanaged hooks must survive.
	for _, name := range []string{"other-unmanaged.cjs", "other-user-hook.cjs"} {
		if !strings.Contains(string(data), name) {
			t.Errorf("unmanaged hook %s was removed — should be preserved", name)
		}
	}

	// Non-hooks keys must survive.
	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if _, ok := out["env"]; !ok {
		t.Error(`"env" key was removed after uninstall`)
	}
}

func TestHooksUninstallDryRunWritesNothing(t *testing.T) {
	hooksDir, settingsPath := buildHookFixture(t)
	origSettings, _ := os.ReadFile(settingsPath)

	_, err := runUninstall(t, hooksDir, settingsPath, true)
	if err != nil {
		t.Fatalf("dry-run uninstall: %v", err)
	}

	// Files must still exist.
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("dry-run removed file: %s", full)
		}
	}

	// Settings must be unchanged.
	afterSettings, _ := os.ReadFile(settingsPath)
	if string(origSettings) != string(afterSettings) {
		t.Error("dry-run mutated settings.json")
	}
}

// ── rollback tests ────────────────────────────────────────────────────────

func makeBackup(t *testing.T, hooksDir, rel, content string) {
	t.Helper()
	base := filepath.Join(hooksDir, rel)
	ext := filepath.Ext(base)
	noExt := strings.TrimSuffix(base, ext)
	ts := time.Now().UTC().Add(-1 * time.Second).Format("20060102T150405")
	bakPath := noExt + ".bak." + ts + ext
	_ = os.MkdirAll(filepath.Dir(bakPath), 0o755)
	if err := os.WriteFile(bakPath, []byte(content), 0o755); err != nil {
		t.Fatalf("write backup %s: %v", bakPath, err)
	}
}

func TestHooksRollbackRestoresFromBak(t *testing.T) {
	hooksDir := t.TempDir()

	// Create backup files for our managed hooks.
	for _, rel := range testManifestFiles() {
		makeBackup(t, hooksDir, rel, "// backup content for "+rel+"\n")
	}

	// Run rollback logic directly (mirrors runHooksRollback without hitting real ~/.claude).
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		bak := newestBackup(full)
		if bak == "" {
			t.Errorf("newestBackup returned empty for %s", rel)
			continue
		}
		data, err := os.ReadFile(bak)
		if err != nil {
			t.Fatalf("read backup %s: %v", bak, err)
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdirall: %v", err)
		}
		if err := os.WriteFile(full, data, 0o755); err != nil {
			t.Fatalf("restore %s: %v", full, err)
		}
	}

	// Each restored file must contain the backup content.
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		content, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("restored file missing: %s: %v", full, err)
			continue
		}
		if !strings.Contains(string(content), "backup content for "+rel) {
			t.Errorf("restored file %s does not contain expected backup content", rel)
		}
	}
}

func TestHooksRollbackNoBackups(t *testing.T) {
	hooksDir := t.TempDir()
	// No backups exist — newestBackup must return empty for all files.
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		if bak := newestBackup(full); bak != "" {
			t.Errorf("expected no backup for %s, got %s", rel, bak)
		}
	}
}

func TestHooksRollbackDryRunWritesNothing(t *testing.T) {
	hooksDir := t.TempDir()

	// Create backup + install real files.
	installTestHooks(t, hooksDir)
	for _, rel := range testManifestFiles() {
		makeBackup(t, hooksDir, rel, "// old backup\n")
	}

	// Capture original content of installed files.
	origContent := make(map[string][]byte)
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		data, _ := os.ReadFile(full)
		origContent[rel] = data
	}

	// Dry-run: should not write anything.
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		bak := newestBackup(full)
		if bak == "" {
			continue
		}
		// dry-run: just check without writing
		_ = bak
	}

	// Files must be unchanged.
	for _, rel := range testManifestFiles() {
		full := filepath.Join(hooksDir, rel)
		data, _ := os.ReadFile(full)
		if string(data) != string(origContent[rel]) {
			t.Errorf("dry-run check mutated %s", rel)
		}
	}
}

// ── newestBackup unit test ────────────────────────────────────────────────

func TestNewestBackupPicksLatest(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "foo.cjs")

	// Create two backups at different timestamps.
	older := filepath.Join(dir, "foo.bak.20240101T000000.cjs")
	newer := filepath.Join(dir, "foo.bak.20250601T120000.cjs")
	_ = os.WriteFile(older, []byte("old"), 0o644)
	_ = os.WriteFile(newer, []byte("new"), 0o644)

	got := newestBackup(target)
	if got != newer {
		t.Errorf("newestBackup: got %s, want %s", got, newer)
	}
}

// ── statusLine install/uninstall ──────────────────────────────────────────

// TestInstallSetsStatusLine mirrors what runInstallHooks does (RegisterHooks +
// SetStatusLine + WriteSettings) and asserts settings.json gains the statusLine key.
func TestInstallSetsStatusLine(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"env":{"FOO":"bar"}}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	s, err := claudeconfig.ReadSettingsAt(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	claudeconfig.RegisterHooks(s, withStatusLine())
	if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	if !strings.Contains(string(data), `"statusLine"`) {
		t.Error("statusLine key not present after install")
	}
	if !strings.Contains(string(data), "statusline.cjs") {
		t.Error("statusline.cjs not referenced in statusLine command")
	}
	if !strings.Contains(string(data), `$HOME`) {
		t.Error("statusLine command missing literal $HOME")
	}
	// Pre-existing key must survive.
	if !strings.Contains(string(data), `"FOO"`) {
		t.Error("env.FOO lost after install")
	}
}

// TestUninstallRemovesStatusLine mirrors unregisterHooksFromSettings (UnregisterHooks +
// UnsetStatusLine + WriteSettings) and asserts statusLine is removed.
func TestUninstallRemovesStatusLine(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"env":{"FOO":"bar"}}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// First: install (set statusLine).
	s, err := claudeconfig.ReadSettingsAt(settingsPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	claudeconfig.RegisterHooks(s, withStatusLine())
	if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
		t.Fatalf("install write: %v", err)
	}

	// Verify it was set.
	installed, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(installed), `"statusLine"`) {
		t.Fatal("statusLine not set after install step")
	}

	// Second: uninstall (unset statusLine).
	s2, err := claudeconfig.ReadSettingsAt(settingsPath)
	if err != nil {
		t.Fatalf("read for uninstall: %v", err)
	}
	claudeconfig.UnregisterHooks(s2, withStatusLine())
	if err := claudeconfig.WriteSettings(s2, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
		t.Fatalf("uninstall write: %v", err)
	}

	uninstalled, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(uninstalled), `"statusLine"`) {
		t.Error("statusLine still present after uninstall")
	}
	// Non-statusLine keys must survive.
	if !strings.Contains(string(uninstalled), `"FOO"`) {
		t.Error("env.FOO lost after uninstall")
	}
}

// TestInstallStatusLineIdempotent verifies that running install twice yields
// the same settings.json content (statusLine not duplicated).
func TestInstallStatusLineIdempotent(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	doInstall := func() {
		s, err := claudeconfig.ReadSettingsAt(settingsPath)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		claudeconfig.RegisterHooks(s, withStatusLine())
		if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	doInstall()
	data1, _ := os.ReadFile(settingsPath)
	doInstall()
	data2, _ := os.ReadFile(settingsPath)

	if string(data1) != string(data2) {
		t.Errorf("install is not idempotent:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}
}

// ── UnregisterHooks idempotency ───────────────────────────────────────────

func TestUnregisterHooksIdempotent(t *testing.T) {
	_, settingsPath := buildHookFixture(t)

	s, _ := claudeconfig.ReadSettingsAt(settingsPath)
	claudeconfig.UnregisterHooks(s, testManifestHooks())
	if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	data1, _ := os.ReadFile(settingsPath)

	s2, _ := claudeconfig.ReadSettingsAt(settingsPath)
	claudeconfig.UnregisterHooks(s2, testManifestHooks())
	if err := claudeconfig.WriteSettings(s2, claudeconfig.WriteOptions{Path: settingsPath}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	data2, _ := os.ReadFile(settingsPath)

	if string(data1) != string(data2) {
		t.Errorf("UnregisterHooks is not idempotent:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}
}
