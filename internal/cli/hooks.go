package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/claudeconfig"
	"github.com/vanducng/vd-cli/v2/internal/hooks"
)

// managedHookFiles lists the flat files (relative to the hooks dir) that vd
// installs. Keep in sync with hooks/assets and claudeconfig.managedHooks.
var managedHookFiles = []string{
	"session-init.cjs",
	"subagent-init.cjs",
	"dev-rules-reminder.cjs",
	filepath.Join("lib", "config.cjs"),
	filepath.Join("lib", "paths.cjs"),
	filepath.Join("lib", "state.cjs"),
}

func newHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks <subcommand>",
		Short: "Manage vd-installed Claude hooks",
		Long: `Manage the vd-cli clean-room Claude hooks installed in ~/.claude/hooks.

Subcommands:
  uninstall  Remove managed hook files and unregister them from settings.json
  rollback   Restore the most recent backup created by 'vd install hooks'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("unknown subcommand %q (valid: uninstall, rollback)", args[0])
		},
	}
	cmd.AddCommand(newHooksUninstallCmd())
	cmd.AddCommand(newHooksRollbackCmd())
	return cmd
}

// ── uninstall ─────────────────────────────────────────────────────────────

func newHooksUninstallCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove vd-managed hooks and unregister them from settings.json",
		Long: `Remove managed hook files from ~/.claude/hooks and unregister their
commands from ~/.claude/settings.json. Only files installed by vd are
removed; any other hooks in settings.json are left untouched.

A backup of settings.json is created before the first modification.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := hooks.DefaultDest()
			if err != nil {
				return err
			}
			return runHooksUninstall(cmd, dest, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print actions without changing files")
	return cmd
}

func runHooksUninstall(cmd *cobra.Command, hooksDir string, dryRun bool) error {
	out := cmd.OutOrStdout()

	// Collect managed files that exist.
	type fileOp struct {
		rel  string
		full string
	}
	var toDelete []fileOp
	for _, rel := range managedHookFiles {
		full := filepath.Join(hooksDir, rel)
		if _, err := os.Stat(full); err == nil {
			toDelete = append(toDelete, fileOp{rel: rel, full: full})
		}
	}

	if dryRun {
		if len(toDelete) == 0 {
			_, _ = fmt.Fprintln(out, "dry-run: no managed hook files found to remove")
		}
		for _, f := range toDelete {
			_, _ = fmt.Fprintf(out, "dry-run: would remove %s\n", f.full)
		}
		_, _ = fmt.Fprintln(out, "dry-run: would unregister managed hooks from settings.json")
		return nil
	}

	// Backup + unregister from settings.json first (surgical sjson removal).
	if err := unregisterHooksFromSettings(); err != nil {
		return fmt.Errorf("unregister hooks: %w", err)
	}
	if !flagQuiet {
		_, _ = fmt.Fprintln(out, "unregistered managed hooks from settings.json")
	}

	// Delete managed hook files.
	for _, f := range toDelete {
		if err := os.Remove(f.full); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", f.full, err)
		}
		// Prune now-empty lib/ dir if nothing else remains.
		dir := filepath.Dir(f.full)
		if dir != hooksDir {
			entries, _ := os.ReadDir(dir)
			if len(entries) == 0 {
				_ = os.Remove(dir)
			}
		}
		if !flagQuiet {
			_, _ = fmt.Fprintf(out, "removed %s\n", f.full)
		}
	}

	if !flagQuiet {
		_, _ = fmt.Fprintln(out, "hooks uninstalled")
	}
	return nil
}

// unregisterHooksFromSettings removes only our managed commands from settings.json.
func unregisterHooksFromSettings() error {
	s, err := claudeconfig.ReadSettings()
	if err != nil {
		return err
	}
	claudeconfig.UnregisterHooks(s)
	return claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{})
}

// ── rollback ──────────────────────────────────────────────────────────────

func newHooksRollbackCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the most recent hook backup created by 'vd install hooks'",
		Long: `Restore each managed hook file from its most recent .bak.<ts>.cjs backup,
and restore settings.json from settings.json.bak if present.

This undoes the last 'vd install hooks' run.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := hooks.DefaultDest()
			if err != nil {
				return err
			}
			return runHooksRollback(cmd, dest, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print actions without changing files")
	return cmd
}

func runHooksRollback(cmd *cobra.Command, hooksDir string, dryRun bool) error {
	out := cmd.OutOrStdout()

	type rollbackOp struct {
		src string
		dst string
	}
	var ops []rollbackOp

	// For each managed hook file, find newest .bak.<ts> backup.
	for _, rel := range managedHookFiles {
		full := filepath.Join(hooksDir, rel)
		bak := newestBackup(full)
		if bak == "" {
			continue
		}
		ops = append(ops, rollbackOp{src: bak, dst: full})
	}

	// Check for settings.json backup.
	settingsBackup := ""
	home, err := os.UserHomeDir()
	if err == nil {
		settingsPath := filepath.Join(home, ".claude", "settings.json")
		bakPath := settingsPath + ".bak"
		if _, err := os.Stat(bakPath); err == nil {
			settingsBackup = bakPath
		}
	}

	if len(ops) == 0 && settingsBackup == "" {
		_, _ = fmt.Fprintln(out, "no backups found to restore")
		return nil
	}

	if dryRun {
		for _, op := range ops {
			_, _ = fmt.Fprintf(out, "dry-run: would restore %s -> %s\n", op.src, op.dst)
		}
		if settingsBackup != "" {
			_, _ = fmt.Fprintf(out, "dry-run: would restore settings.json from %s\n", settingsBackup)
		}
		return nil
	}

	// Restore hook files.
	for _, op := range ops {
		if err := os.MkdirAll(filepath.Dir(op.dst), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", op.dst, err)
		}
		data, err := os.ReadFile(op.src)
		if err != nil {
			return fmt.Errorf("read backup %s: %w", op.src, err)
		}
		if err := os.WriteFile(op.dst, data, 0o755); err != nil {
			return fmt.Errorf("restore %s: %w", op.dst, err)
		}
		if !flagQuiet {
			_, _ = fmt.Fprintf(out, "restored %s\n", op.dst)
		}
	}

	// Restore settings.json.
	if settingsBackup != "" {
		settingsPath := filepath.Join(home, ".claude", "settings.json")
		data, err := os.ReadFile(settingsBackup)
		if err != nil {
			return fmt.Errorf("read settings backup: %w", err)
		}
		if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
			return fmt.Errorf("restore settings.json: %w", err)
		}
		if !flagQuiet {
			_, _ = fmt.Fprintf(out, "restored settings.json from %s\n", settingsBackup)
		}
	}

	if !flagQuiet {
		_, _ = fmt.Fprintln(out, "rollback complete")
	}
	return nil
}

// newestBackup finds the most recent .bak.<ts>.<ext> file for target.
// Returns empty string if none found.
func newestBackup(target string) string {
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(target, ext)
	dir := filepath.Dir(target)
	name := filepath.Base(base)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	prefix := name + ".bak."
	var candidates []struct {
		ts   time.Time
		path string
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, prefix) {
			continue
		}
		// Strip prefix and extension to get the timestamp portion.
		ts := strings.TrimPrefix(n, prefix)
		ts = strings.TrimSuffix(ts, ext)
		t, err := time.Parse("20060102T150405", ts)
		if err != nil {
			continue
		}
		candidates = append(candidates, struct {
			ts   time.Time
			path string
		}{ts: t, path: filepath.Join(dir, n)})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ts.After(candidates[j].ts)
	})
	return candidates[0].path
}
