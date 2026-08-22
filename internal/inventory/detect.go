package inventory

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstallAgent is a vd install target that can be detected from a local home dir.
type InstallAgent struct {
	Name string // claude, codex, cursor, droid, pi
	Home string
}

// DetectInstallAgents returns the install agents whose home directories exist
// on this machine. Missing homes are skipped. Order is Claude, Codex, Cursor,
// Droid, Pi — the same homes inventory already knows, plus Droid and Pi.
//
// Codex and Cursor honor $VD_CODEX_HOME / $VD_CURSOR_HOME the same way
// Inventory does.
func DetectInstallAgents() ([]InstallAgent, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return detectInstallAgents(home), nil
}

func detectInstallAgents(userHome string) []InstallAgent {
	var out []InstallAgent
	for _, a := range installAgentHomes(userHome) {
		if isDir(a.Home) {
			out = append(out, a)
		}
	}
	return out
}

func installAgentHomes(userHome string) []InstallAgent {
	return []InstallAgent{
		{Name: "claude", Home: filepath.Join(userHome, ".claude")},
		{Name: "codex", Home: envOr("VD_CODEX_HOME", filepath.Join(userHome, ".agents"))},
		{Name: "cursor", Home: envOr("VD_CURSOR_HOME", filepath.Join(userHome, ".cursor"))},
		{Name: "droid", Home: filepath.Join(userHome, ".factory")},
		{Name: "pi", Home: filepath.Join(userHome, ".pi")},
	}
}
