// Package claudeconfig manages ~/.claude/.vd.json and ~/.claude/settings.json
// on behalf of vd install hooks.
package claudeconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/sjson"
)

const ckConfigFile = ".vd.json"

// Legacy file vd no longer reads. Detected only to raise a migration error
// (run the cktovd skill). Identifier rename CKConfig/ReadCKConfig → VD* is a
// separate gradual step.
const ckConfigFileLegacy = ".ck.json"

// CKConfig is a partial view of ~/.claude/.vd.json sufficient for vd's needs.
// rawOrig holds the complete original file bytes; writes patch only the keys
// vd owns rather than rewriting the whole file.
type CKConfig struct {
	Plan        *CKPlan        `json:"plan,omitempty"`
	Paths       *CKPaths       `json:"paths,omitempty"`
	CodingLevel *int           `json:"codingLevel,omitempty"`
	Locale      *CKLocale      `json:"locale,omitempty"`
	Hooks       map[string]any `json:"hooks,omitempty"`

	rawOrig []byte // original file bytes; nil for a new file
}

// CKPlan mirrors the plan block we care about.
type CKPlan struct {
	NamingFormat string `json:"namingFormat,omitempty"`
	DateFormat   string `json:"dateFormat,omitempty"`
	IssuePrefix  string `json:"issuePrefix,omitempty"`
	ReportsDir   string `json:"reportsDir,omitempty"`
}

// CKPaths mirrors the paths block, adding the umbrella slot.
type CKPaths struct {
	Plans    string  `json:"plans,omitempty"`
	Docs     string  `json:"docs,omitempty"`
	Umbrella *string `json:"umbrella,omitempty"` // nil encodes as JSON null
}

// CKLocale mirrors the locale block.
type CKLocale struct {
	ThinkingLanguage *string `json:"thinkingLanguage,omitempty"`
	ResponseLanguage *string `json:"responseLanguage,omitempty"`
}

// ckConfigPath returns the absolute path to ~/.claude/.vd.json.
func ckConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", ckConfigFile), nil
}

// ckConfigLegacyPath returns the absolute path to ~/.claude/.ck.json (legacy).
func ckConfigLegacyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", ckConfigFileLegacy), nil
}

// ReadCKConfig reads ~/.claude/.vd.json.
// vd no longer reads .ck.json: if .vd.json is absent but a legacy .ck.json
// lingers, an error is returned pointing at the cktovd migration skill.
// If neither exists, an empty config is returned (not an error).
func ReadCKConfig() (*CKConfig, error) {
	newPath, err := ckConfigPath()
	if err != nil {
		return nil, err
	}
	cfg, err := readCKConfigAt(newPath)
	if err != nil {
		return nil, err
	}
	if cfg.rawOrig != nil {
		return cfg, nil
	}
	// .vd.json absent — refuse to silently fall back to legacy .ck.json.
	legacyPath, err := ckConfigLegacyPath()
	if err == nil {
		if _, statErr := os.Stat(legacyPath); statErr == nil {
			return nil, fmt.Errorf(
				"legacy %s found at %s but no %s — run the cktovd skill (or rename it to %s); vd no longer reads %s",
				ckConfigFileLegacy, legacyPath, ckConfigFile, ckConfigFile, ckConfigFileLegacy)
		}
	}
	return cfg, nil
}

func readCKConfigAt(path string) (*CKConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &CKConfig{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("%s contains invalid JSON — refusing to proceed", path)
	}

	cfg := &CKConfig{rawOrig: data}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return cfg, nil
}

// EnsureUmbrellaSlot ensures the paths block exists so the umbrella key can be
// set later. This is the only structural mutation vd makes to .vd.json.
func EnsureUmbrellaSlot(cfg *CKConfig) {
	if cfg.Paths == nil {
		cfg.Paths = &CKPaths{}
	}
}

// WriteCKConfig atomically writes cfg back to ~/.claude/.vd.json.
// Only the keys vd owns (paths) are patched; all other keys stay
// byte-for-byte identical in their original positions.
func WriteCKConfig(cfg *CKConfig) error {
	path, err := ckConfigPath()
	if err != nil {
		return err
	}
	return writeCKConfigAt(path, cfg)
}

func writeCKConfigAt(path string, cfg *CKConfig) error {
	data, err := buildCKConfigBytes(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return atomicWrite(path, data)
}

// buildCKConfigBytes patches only the keys vd owns into the original raw bytes.
// For a new file it builds the minimal JSON from scratch.
func buildCKConfigBytes(cfg *CKConfig) ([]byte, error) {
	base := cfg.rawOrig
	if len(base) == 0 {
		base = []byte(`{}`)
	}

	// Patch paths if vd set it.
	if cfg.Paths != nil {
		pathsJSON, err := json.Marshal(cfg.Paths)
		if err != nil {
			return nil, fmt.Errorf("marshal paths: %w", err)
		}
		base, err = sjson.SetRawBytes(base, "paths", pathsJSON)
		if err != nil {
			return nil, fmt.Errorf("patch paths key: %w", err)
		}
	}

	// Ensure trailing newline.
	if len(base) > 0 && base[len(base)-1] != '\n' {
		base = append(base, '\n')
	}
	return base, nil
}
