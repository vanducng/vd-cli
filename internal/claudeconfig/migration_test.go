package claudeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A lingering legacy .ck.json (without .vd.json) must raise a migration error —
// vd no longer silently falls back to reading it.
func TestReadCKConfig_LegacyOnlyRaisesMigrationError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, ".ck.json"), []byte(`{"paths":{"plans":"plans"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadCKConfig()
	if err == nil {
		t.Fatal("expected a migration error when only legacy .ck.json is present, got nil")
	}
	if !strings.Contains(err.Error(), "cktovd") {
		t.Errorf("migration error should point at the cktovd skill, got: %v", err)
	}
}

// No config at all is not an error — an empty config is returned.
func TestReadCKConfig_NeitherPresentReturnsEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := ReadCKConfig()
	if err != nil {
		t.Fatalf("absent config must not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected an empty config, got nil")
	}
}

// When .vd.json exists, the legacy file is ignored entirely (no error).
func TestReadCKConfig_VDPresentIgnoresLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, ".vd.json"), []byte(`{"paths":{"plans":"plans"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, ".ck.json"), []byte(`{"paths":{"plans":"legacy"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadCKConfig()
	if err != nil {
		t.Fatalf(".vd.json present must not error: %v", err)
	}
	if cfg == nil || cfg.rawOrig == nil {
		t.Fatal("expected .vd.json to be read")
	}
}
