package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AgentsResult describes one agent-TOML deploy action.
type AgentsResult struct {
	Name   string
	Source string
	Dest   string
	Action string
}

// DeployCodexAgents copies <repoRoot>/agents/*.toml into ~/.codex/agents/.
//
// Distinct from Codex(): agent TOMLs are single files at a fixed personal path
// (~/.codex/agents), not skill directories under the user|repo scope model.
// A missing agents/ dir yields no results (not an error). Idempotent: existing
// files are overwritten (they are vd-managed copies).
func DeployCodexAgents(repoRoot string, opts CodexOptions) ([]AgentsResult, error) {
	agentsDir := filepath.Join(repoRoot, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents dir %s: %w", agentsDir, err)
	}

	destRoot := opts.Dest
	if destRoot == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return nil, fmt.Errorf("resolve home directory: %w", herr)
		}
		destRoot = filepath.Join(home, ".codex", "agents")
	} else if err := assertSafeDest(destRoot); err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, nil
	}

	if !opts.DryRun {
		if err := os.MkdirAll(destRoot, 0o755); err != nil {
			return nil, fmt.Errorf("create codex agents dir %s: %w", destRoot, err)
		}
	}

	results := make([]AgentsResult, 0, len(names))
	for _, name := range names {
		if err := assertSimpleName(name); err != nil {
			return nil, err
		}
		src := filepath.Join(agentsDir, name)
		dst := filepath.Join(destRoot, name)
		action := "agent-toml"
		if opts.DryRun {
			action = "agent-toml (dry-run)"
		} else if err := copyFile(src, dst); err != nil {
			return nil, fmt.Errorf("deploy agent %s: %w", name, err)
		}
		results = append(results, AgentsResult{Name: name, Source: src, Dest: dst, Action: action})
	}
	return results, nil
}
