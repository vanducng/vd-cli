package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/claudeconfig"
	"github.com/vanducng/vd-cli/v2/internal/config"
	"github.com/vanducng/vd-cli/v2/internal/hooks"
	agentinstall "github.com/vanducng/vd-cli/v2/internal/install"
)

type installOptions struct {
	scope  string
	dest   string
	copy   bool
	force  bool
	dryRun bool
	dev    bool
}

type installTarget struct {
	agent string
	opts  installOptions
}

func newInstallCmd() *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install [agent] [skill...]",
		Short: "Install local skills into an agent environment",
		Long: `Install skills from this repository into a local agent environment.

Run without an agent to select one or more install targets:
  1) Codex user skills            symlink to $HOME/.agents/skills
  2) Codex repo skills            symlink to .agents/skills
  3) Codex snapshot copy          copy to $HOME/.agents/skills
  4) Claude Code plugin           marketplace/plugin install
  5) Claude Code dev symlinks     symlink to $HOME/.claude/skills
  6) Droid user skills            symlink to $HOME/.factory/skills
  7) Droid repo skills            symlink to .factory/skills
  8) Droid snapshot copy          copy to $HOME/.factory/skills

Pick several at once with a comma-separated list (e.g. 1,5,7). Use 'all' for every non-conflicting agent environment.

Agents:
  codex          installs skills into Codex discovery paths
  droid          installs skills into Factory Droid discovery paths
  claude         registers and installs this repo as a Claude Code plugin
  claude --dev   per-skill symlink into $HOME/.claude/skills (mirrors codex)
  hooks          deploys hooks from hooks/hooks.toml to $HOME/.claude/hooks`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveInstallRoot(cmd, flagRoot, opts.dryRun)
			if err != nil {
				return err
			}
			return runInstall(cmd, root, args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.scope, "scope", "user", "Install scope: codex/droid user|repo, claude user|project|local")
	cmd.Flags().StringVar(&opts.dest, "dest", "", "Override destination directory")
	cmd.Flags().BoolVar(&opts.copy, "copy", false, "Copy skills instead of symlinking them")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Replace existing installed skill directories")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print actions without changing files")
	cmd.Flags().BoolVar(&opts.dev, "dev", false, "Claude only: per-skill symlink into $HOME/.claude/skills instead of marketplace plugin install")

	return cmd
}

func runInstall(cmd *cobra.Command, repoRoot string, args []string, opts installOptions) error {
	agent := ""
	skills := args
	if len(args) > 0 {
		if isKnownInstallAgent(args[0]) {
			agent = normalizeInstallAgent(args[0])
			skills = args[1:]
		} else if _, err := os.Stat(filepath.Join(repoRoot, "skills", args[0], "SKILL.md")); err != nil {
			// Not a known agent and not a real skill — most likely a
			// typo'd agent name. Fail fast with a clear hint instead of
			// silently treating it as a skill and confusing the user
			// downstream.
			return fmt.Errorf("unknown agent or skill %q (valid agents: codex, droid, claude, hooks)", args[0])
		}
	}

	if agent != "" {
		return runInstallTarget(cmd, repoRoot, agent, skills, opts)
	}

	targets, err := promptInstallSelection(cmd, opts)
	if err != nil {
		return err
	}
	for i, t := range targets {
		if len(targets) > 1 && !flagQuiet {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n==> [%d/%d] %s\n", i+1, len(targets), describeInstallTarget(t))
		}
		if err := runInstallTarget(cmd, repoRoot, t.agent, skills, t.opts); err != nil {
			return err
		}
	}
	return nil
}

func runInstallTarget(cmd *cobra.Command, repoRoot, agent string, skills []string, opts installOptions) error {
	switch agent {
	case "codex":
		return runInstallCodex(cmd, repoRoot, skills, opts)
	case "droid":
		return runInstallDroid(cmd, repoRoot, skills, opts)
	case "claude":
		if opts.dev {
			return runInstallClaudeDev(cmd, repoRoot, skills, opts)
		}
		if len(skills) > 0 {
			return fmt.Errorf("claude install does not accept skill names without --dev; it installs the configured plugin bundle")
		}
		return runInstallClaude(cmd, repoRoot, opts)
	case "hooks":
		if len(skills) > 0 {
			return fmt.Errorf("hooks install does not accept skill names; it deploys the hooks listed in hooks/hooks.toml")
		}
		return runInstallHooks(cmd, repoRoot, opts)
	default:
		return fmt.Errorf("unknown agent %q (valid: codex, droid, claude, hooks)", agent)
	}
}

func describeInstallTarget(t installTarget) string {
	switch t.agent {
	case "codex":
		switch {
		case t.opts.copy:
			return "Codex snapshot copy"
		case t.opts.scope == "repo":
			return "Codex repo skills"
		default:
			return "Codex user skills"
		}
	case "claude":
		if t.opts.dev {
			return "Claude Code dev symlinks"
		}
		return "Claude Code plugin"
	case "droid":
		switch {
		case t.opts.copy:
			return "Droid snapshot copy"
		case t.opts.scope == "repo":
			return "Droid repo skills"
		default:
			return "Droid user skills"
		}
	default:
		return t.agent
	}
}

func runInstallDroid(cmd *cobra.Command, repoRoot string, skills []string, opts installOptions) error {
	results, err := agentinstall.Droid(repoRoot, agentinstall.DroidOptions{
		Scope:  opts.scope,
		Dest:   opts.dest,
		Skills: skills,
		Copy:   opts.copy,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	if flagQuiet {
		return nil
	}
	for _, result := range results {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s droid skill %s -> %s\n", result.Action, result.Name, result.Dest)
	}
	if !opts.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "restart Droid to pick up newly installed skills")
	}
	return nil
}

func runInstallCodex(cmd *cobra.Command, repoRoot string, skills []string, opts installOptions) error {
	results, err := agentinstall.Codex(repoRoot, agentinstall.CodexOptions{
		Scope:  opts.scope,
		Dest:   opts.dest,
		Skills: skills,
		Copy:   opts.copy,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	// Deploy Codex subagent definitions (agents/*.toml -> ~/.codex/agents).
	agents, agErr := agentinstall.DeployCodexAgents(repoRoot, agentinstall.CodexOptions{DryRun: opts.dryRun})
	if agErr != nil {
		return agErr
	}
	if flagQuiet {
		return nil
	}
	for _, result := range results {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s codex skill %s -> %s\n", result.Action, result.Name, result.Dest)
	}
	for _, a := range agents {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s -> %s\n", a.Action, a.Name, a.Dest)
	}
	if !opts.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "restart Codex to pick up newly installed skills")
	}
	return nil
}

func runInstallClaudeDev(cmd *cobra.Command, repoRoot string, skills []string, opts installOptions) error {
	results, err := agentinstall.ClaudeDev(repoRoot, agentinstall.ClaudeDevOptions{
		Dest:   opts.dest,
		Skills: skills,
		Copy:   opts.copy,
		Force:  opts.force,
		DryRun: opts.dryRun,
	})
	if err != nil {
		return err
	}
	if flagQuiet {
		return nil
	}
	for _, result := range results {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s claude skill %s -> %s\n", result.Action, result.Name, result.Dest)
	}
	if !opts.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "restart Claude Code to pick up newly installed skills")
	}
	return nil
}

func runInstallClaude(cmd *cobra.Command, repoRoot string, opts installOptions) error {
	if opts.dest != "" || opts.copy || opts.force {
		return fmt.Errorf("--dest, --copy, and --force only apply to codex or --dev installs")
	}
	switch opts.scope {
	case "user", "project", "local":
	default:
		return fmt.Errorf("invalid claude scope %q (valid: user, project, local)", opts.scope)
	}

	pluginName, marketplaceName, err := claudePluginSpec(repoRoot)
	if err != nil {
		return err
	}
	spec := pluginName + "@" + marketplaceName

	if opts.dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would run: vd build claude\n")
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would run: claude plugin marketplace add --scope %s %s\n", opts.scope, repoRoot)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would run: claude plugin install --scope %s %s\n", opts.scope, spec)
		return nil
	}

	if err := runBuild(cmd, repoRoot, []string{"claude"}); err != nil {
		return err
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude command not found in PATH")
	}

	if err := runExternal(cmd, "claude", "plugin", "marketplace", "add", "--scope", opts.scope, repoRoot); err != nil {
		return err
	}
	if err := runExternal(cmd, "claude", "plugin", "install", "--scope", opts.scope, spec); err != nil {
		return err
	}
	if !flagQuiet {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed Claude Code plugin %s\n", spec)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "restart Claude Code to pick up plugin changes")
	}
	return nil
}

func runInstallHooks(cmd *cobra.Command, repoRoot string, opts installOptions) error {
	dest := opts.dest
	if dest == "" {
		var err error
		dest, err = hooks.DefaultDest()
		if err != nil {
			return err
		}
	}

	srcDir := filepath.Join(repoRoot, "hooks")
	manifest := filepath.Join(srcDir, "hooks.toml")
	if _, err := os.Stat(manifest); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no hooks/hooks.toml found at %s — vd installs hooks from a local manifest", repoRoot)
		}
		return fmt.Errorf("stat %s: %w", manifest, err)
	}
	manifestHooks, err := hooks.LoadManifest(manifest)
	if err != nil {
		return err
	}

	if opts.dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would install hooks to %s\n", dest)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would register hooks in settings.json (dry-run):")
		s, err := claudeconfig.ReadSettings()
		if err != nil {
			return fmt.Errorf("read settings: %w", err)
		}
		claudeconfig.RegisterHooks(s, manifestHooks)
		if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{DryRun: true}); err != nil {
			return err
		}
		for _, h := range manifestHooks {
			if h.Event == "codex.notify" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would wire Codex notify: %s\n",
					strings.Join(claudeconfig.CodexNotifyCommand(h, dest), " "))
			}
		}
		if err := dryRunCodexHooks(cmd, manifestHooks); err != nil {
			return err
		}
		return nil
	}

	results, err := hooks.InstallFrom(srcDir, dest, hooks.Files(manifestHooks))
	if err != nil {
		return err
	}

	if !flagQuiet {
		for _, r := range results {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s hooks/%s -> %s\n", r.Action, r.RelPath, r.Dest)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "hooks installed to %s\n", dest)
	}

	s, err := claudeconfig.ReadSettings()
	if err != nil {
		return fmt.Errorf("read settings.json: %w", err)
	}
	claudeconfig.RegisterHooks(s, manifestHooks)
	if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{}); err != nil {
		return fmt.Errorf("register hooks in settings.json: %w", err)
	}

	if !flagQuiet {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "hooks registered in settings.json")
	}

	if err := wireCodexNotify(cmd, manifestHooks, dest); err != nil {
		return err
	}
	return installCodexHooks(cmd, srcDir, manifestHooks)
}

// wireCodexNotify registers each codex.notify hook into ~/.codex/config.toml.
func wireCodexNotify(cmd *cobra.Command, manifestHooks []claudeconfig.Hook, dest string) error {
	var codex []claudeconfig.Hook
	for _, h := range manifestHooks {
		if h.Event == "codex.notify" {
			codex = append(codex, h)
		}
	}
	if len(codex) == 0 {
		return nil
	}
	// Codex has a single top-level notify program — more than one would silently
	// clobber each other, so refuse rather than pick a winner.
	if len(codex) > 1 {
		return fmt.Errorf("manifest declares %d codex.notify hooks, but Codex supports only one notify program", len(codex))
	}

	codexPath, err := claudeconfig.CodexConfigPath()
	if err != nil {
		return fmt.Errorf("resolve codex config path: %w", err)
	}
	prev, err := claudeconfig.WireCodexNotify(codexPath, claudeconfig.CodexNotifyCommand(codex[0], dest))
	if err != nil {
		return fmt.Errorf("wire Codex notify: %w", err)
	}
	if !flagQuiet {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "wired Codex notify in %s\n", codexPath)
		if prev != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"note: replaced existing Codex notify (set CODEX_NOTIFY_FORWARD in your env to chain it): %s\n", prev)
		}
	}
	return nil
}

func dryRunCodexHooks(cmd *cobra.Command, manifestHooks []claudeconfig.Hook) error {
	files := codexHookFiles(manifestHooks)
	if len(files) == 0 {
		return nil
	}
	codexDest, err := claudeconfig.CodexHooksDest()
	if err != nil {
		return err
	}
	codexPath, err := claudeconfig.CodexHooksPath()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would install Codex hooks to %s\n", codexDest)
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would register Codex hooks in hooks.json (dry-run):")
	s, err := claudeconfig.ReadSettingsAt(codexPath)
	if err != nil {
		return fmt.Errorf("read codex hooks.json: %w", err)
	}
	claudeconfig.RegisterCodexHooks(s, manifestHooks, codexDest)
	return claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: codexPath, DryRun: true})
}

func installCodexHooks(cmd *cobra.Command, srcDir string, manifestHooks []claudeconfig.Hook) error {
	files := codexHookFiles(manifestHooks)
	if len(files) == 0 {
		return nil
	}
	codexDest, err := claudeconfig.CodexHooksDest()
	if err != nil {
		return err
	}
	results, err := hooks.InstallFrom(srcDir, codexDest, files)
	if err != nil {
		return fmt.Errorf("install Codex hooks: %w", err)
	}
	if !flagQuiet {
		for _, r := range results {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s hooks/%s -> %s\n", r.Action, r.RelPath, r.Dest)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Codex hooks installed to %s\n", codexDest)
	}

	codexPath, err := claudeconfig.CodexHooksPath()
	if err != nil {
		return err
	}
	s, err := claudeconfig.ReadSettingsAt(codexPath)
	if err != nil {
		return fmt.Errorf("read codex hooks.json: %w", err)
	}
	claudeconfig.RegisterCodexHooks(s, manifestHooks, codexDest)
	if err := claudeconfig.WriteSettings(s, claudeconfig.WriteOptions{Path: codexPath}); err != nil {
		return fmt.Errorf("register Codex hooks in hooks.json: %w", err)
	}
	if !flagQuiet {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Codex hooks registered in %s\n", codexPath)
	}
	return nil
}

func codexHookFiles(manifestHooks []claudeconfig.Hook) []string {
	needed := make(map[string]bool)
	for _, h := range manifestHooks {
		if _, ok := claudeconfig.CodexEventName(h.Event); ok {
			needed[h.File] = true
		}
	}
	if len(needed) == 0 {
		return nil
	}
	for _, h := range manifestHooks {
		if h.Lib {
			needed[h.File] = true
		}
	}

	files := make([]string, 0, len(needed))
	seen := make(map[string]bool, len(needed))
	for _, h := range manifestHooks {
		if needed[h.File] && !seen[h.File] {
			seen[h.File] = true
			files = append(files, h.File)
		}
	}
	return files
}

func claudePluginSpec(repoRoot string) (pluginName, marketplaceName string, err error) {
	manifestPath := filepath.Join(repoRoot, "skills.toml")
	manifest, err := config.Load(manifestPath)
	if err != nil {
		return "", "", fmt.Errorf("load skills.toml: %w", err)
	}
	config.ApplyDefaults(manifest, filepath.Join(repoRoot, ".claude-plugin", "marketplace.json"))
	pluginName = manifest.Targets.Claude.Bundle.Name
	marketplaceName = manifest.Meta.Name
	if pluginName == "" {
		return "", "", fmt.Errorf("missing [targets.claude.bundle].name")
	}
	if marketplaceName == "" {
		return "", "", fmt.Errorf("missing [meta].name")
	}
	return pluginName, marketplaceName, nil
}

func runExternal(cmd *cobra.Command, name string, args ...string) error {
	ext := exec.Command(name, args...)
	ext.Stdout = cmd.OutOrStdout()
	ext.Stderr = cmd.ErrOrStderr()
	ext.Stdin = cmd.InOrStdin()
	if err := ext.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func isKnownInstallAgent(s string) bool {
	switch normalizeInstallAgent(s) {
	case "codex", "droid", "claude", "hooks":
		return true
	default:
		return false
	}
}

func normalizeInstallAgent(s string) string {
	switch strings.ToLower(s) {
	case "codex":
		return "codex"
	case "droid":
		return "droid"
	case "claude", "claude-code", "claudecode":
		return "claude"
	case "hooks":
		return "hooks"
	default:
		return strings.ToLower(s)
	}
}

func promptInstallSelection(cmd *cobra.Command, opts installOptions) ([]installTarget, error) {
	in := cmd.InOrStdin()
	if !isInteractive(in) {
		return nil, fmt.Errorf("agent argument required when stdin is not interactive")
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, "Install skills for:")
	_, _ = fmt.Fprintln(out, "  1) Codex user skills            symlink to $HOME/.agents/skills")
	_, _ = fmt.Fprintln(out, "  2) Codex repo skills            symlink to .agents/skills")
	_, _ = fmt.Fprintln(out, "  3) Codex snapshot copy          copy to $HOME/.agents/skills")
	_, _ = fmt.Fprintln(out, "  4) Claude Code plugin           marketplace/plugin install")
	_, _ = fmt.Fprintln(out, "  5) Claude Code dev symlinks     symlink to $HOME/.claude/skills")
	_, _ = fmt.Fprintln(out, "  6) Droid user skills            symlink to $HOME/.factory/skills")
	_, _ = fmt.Fprintln(out, "  7) Droid repo skills            symlink to .factory/skills")
	_, _ = fmt.Fprintln(out, "  8) Droid snapshot copy          copy to $HOME/.factory/skills")
	_, _ = fmt.Fprint(out, "Select install target(s) [1-8, comma-separated, or 'all' environments]: ")

	reader := bufio.NewReader(in)
	text, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read selection: %w", err)
	}
	return resolveInstallSelections(text, opts)
}

func resolveInstallSelections(selection string, opts installOptions) ([]installTarget, error) {
	tokens := splitInstallSelections(selection)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no install target selected")
	}

	expanded := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if normalizeInstallSelection(tok) == "all" {
			expanded = append(expanded, "1", "2", "4", "5", "6", "7")
			continue
		}
		expanded = append(expanded, tok)
	}

	seen := make(map[string]bool, len(expanded))
	targets := make([]installTarget, 0, len(expanded))
	for _, tok := range expanded {
		key := normalizeInstallSelection(tok)
		if seen[key] {
			continue
		}
		agent, resolved, err := resolveInstallSelection(tok, opts)
		if err != nil {
			return nil, err
		}
		seen[key] = true
		targets = append(targets, installTarget{agent: agent, opts: resolved})
	}
	if err := rejectInstallTargetConflicts(targets); err != nil {
		return nil, err
	}
	if len(targets) > 1 && opts.dest != "" {
		return nil, fmt.Errorf("--dest requires a single install target")
	}
	return targets, nil
}

func rejectInstallTargetConflicts(targets []installTarget) error {
	types := make(map[string]bool)
	for _, target := range targets {
		if target.agent != "codex" && target.agent != "droid" {
			continue
		}
		key := target.agent + ":" + target.opts.scope
		if copyMode, ok := types[key]; ok && copyMode != target.opts.copy {
			return fmt.Errorf("conflicting %s %s install variants: choose symlink or snapshot copy", target.agent, target.opts.scope)
		}
		types[key] = target.opts.copy
	}
	return nil
}

func splitInstallSelections(selection string) []string {
	tokens := make([]string, 0, 8)
	for _, part := range strings.Split(selection, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "[]")
		part = strings.TrimSpace(part)
		if part != "" {
			tokens = append(tokens, part)
		}
	}
	return tokens
}

func resolveInstallSelection(selection string, opts installOptions) (string, installOptions, error) {
	switch normalizeInstallSelection(selection) {
	case "codex-user":
		opts.scope = "user"
		opts.copy = false
		return "codex", opts, nil
	case "codex-repo":
		opts.scope = "repo"
		opts.copy = false
		return "codex", opts, nil
	case "codex-copy":
		opts.scope = "user"
		opts.copy = true
		return "codex", opts, nil
	case "claude":
		// Clear codex-only flags so a user who passed `--copy` / `--dest`
		// / `--force` and then picked Claude in the picker doesn't trip
		// the "only apply to codex installs" guard.
		opts.copy = false
		opts.dest = ""
		opts.force = false
		opts.dev = false
		if opts.scope == "user" || opts.scope == "project" || opts.scope == "local" {
			return "claude", opts, nil
		}
		opts.scope = "user"
		return "claude", opts, nil
	case "claude-dev":
		opts.dev = true
		return "claude", opts, nil
	case "droid-user":
		opts.scope = "user"
		opts.copy = false
		opts.dev = false
		return "droid", opts, nil
	case "droid-repo":
		opts.scope = "repo"
		opts.copy = false
		opts.dev = false
		return "droid", opts, nil
	case "droid-copy":
		opts.scope = "user"
		opts.copy = true
		opts.dev = false
		return "droid", opts, nil
	default:
		return "", opts, fmt.Errorf("invalid selection %q", strings.TrimSpace(selection))
	}
}

func normalizeInstallSelection(selection string) string {
	s := strings.ToLower(strings.TrimSpace(selection))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Join(strings.Fields(s), "-")
	switch s {
	case "1", "codex", "codex-user", "codex-user-skills", "user":
		return "codex-user"
	case "2", "codex-repo", "codex-repo-skills", "repo":
		return "codex-repo"
	case "3", "codex-copy", "codex-snapshot", "codex-snapshot-copy", "copy", "snapshot":
		return "codex-copy"
	case "4", "claude", "claude-code", "claudecode":
		return "claude"
	case "5", "claude-dev", "claude-code-dev", "claudedev", "dev":
		return "claude-dev"
	case "6", "droid", "droid-user", "droid-user-skills":
		return "droid-user"
	case "7", "droid-repo", "droid-repo-skills":
		return "droid-repo"
	case "8", "droid-copy", "droid-snapshot", "droid-snapshot-copy":
		return "droid-copy"
	case "all", "a", "*":
		return "all"
	default:
		return s
	}
}
