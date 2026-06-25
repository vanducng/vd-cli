package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/claudeconfig"
	"github.com/vanducng/vd-cli/v2/internal/extension"
)

const extensionsDirEnv = "VD_EXTENSIONS_DIR"

// resolveExtensionsDir locates the vd-cli root that holds extensions/.
// Precedence: --extensions-dir flag, VD_EXTENSIONS_DIR env, a cwd ancestor that
// contains an extensions/ directory.
func resolveExtensionsDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv(extensionsDirEnv); env != "" {
		return env, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; {
		if fi, statErr := os.Stat(filepath.Join(dir, "extensions")); statErr == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot locate extensions/ — set %s or run from the vd-cli repo", extensionsDirEnv)
		}
		dir = parent
	}
}

func newMcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp <subcommand>",
		Short: "Manage vd extensions (MCP servers/services) across Codex and Claude",
		Long: `Manage vd-cli extensions — self-contained MCP servers under extensions/<name>/.

vd is the manager: it registers extensions into Codex (~/.codex/config.toml
[mcp_servers]) and Claude (~/.claude.json user / .mcp.json project). MCP scope
{project,user,global} affects the Claude target only; Codex uses its single
user-level config.toml.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("unknown subcommand %q (valid: list, install, enable, disable, doctor, logs)", args[0])
		},
	}
	cmd.AddCommand(newMcpListCmd(), newMcpInstallCmd(), newMcpEnableCmd(), newMcpDisableCmd(), newMcpDoctorCmd(), newMcpLogsCmd())
	return cmd
}

// mcpLogDir is the shared log directory for vd extensions. Extensions log to
// <dir>/<name>.log so they're transparent and improvable; `vd mcp logs` reads it.
// Matches the convention used by the extension runtimes (env VD_LOG_DIR).
func mcpLogDir() string {
	if d := os.Getenv("VD_LOG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir() // never resolve to a relative path
	}
	return filepath.Join(home, ".vd", "logs")
}

func newMcpLogsCmd() *cobra.Command {
	var tail int
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs [name]",
		Short: "Show an extension's log (~/.vd/logs/<name>.log) — for inspection + continuous improvement",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return listExtensionLogs(cmd.OutOrStdout())
			}
			name := args[0]
			if name != filepath.Base(name) || strings.Contains(name, "..") {
				return fmt.Errorf("invalid extension name %q", name) // no path traversal
			}
			path := filepath.Join(mcpLogDir(), name+".log")
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no log yet at %s (the extension hasn't run, or doesn't log)", path)
				}
				return err
			}
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			if tail > 0 && len(lines) > tail {
				lines = lines[len(lines)-tail:]
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out, strings.Join(lines, "\n"))
			if !follow {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return followLog(ctx, out, path, int64(len(data)))
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 0, "Show only the last N lines (0 = all)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream new lines as they're written (Ctrl-C to stop)")
	return cmd
}

// listExtensionLogs prints the extensions that have a log, so `vd mcp logs`
// with no name tells the caller what they can inspect.
func listExtensionLogs(out io.Writer) error {
	dir := mcpLogDir()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no extension logs in %s yet — run an extension first", dir)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			names = append(names, strings.TrimSuffix(e.Name(), ".log"))
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("no extension logs in %s yet — run an extension first", dir)
	}
	sort.Strings(names)
	_, _ = fmt.Fprintf(out, "extensions with logs (pass one, e.g. `vd mcp logs %s`):\n  %s\n", names[0], strings.Join(names, "\n  "))
	return nil
}

// followLog streams bytes appended to path after offset until the context is
// canceled (Ctrl-C). Handles truncation/rotation by restarting from the top.
func followLog(ctx context.Context, out io.Writer, path string, offset int64) error {
	// The root command uses plain Execute() (no signal-aware context), so install
	// our own SIGINT handler here so Ctrl-C stops --follow cleanly.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open log %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek log %s: %w", path, err)
	}
	buf := make([]byte, 8192)
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		for {
			n, rerr := f.Read(buf)
			if n > 0 {
				_, _ = out.Write(buf[:n])
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				return fmt.Errorf("read log %s: %w", path, rerr)
			}
		}
		// If the file shrank below our cursor (rotation/truncation), restart.
		if fi, statErr := f.Stat(); statErr == nil {
			if pos, _ := f.Seek(0, io.SeekCurrent); fi.Size() < pos {
				_, _ = f.Seek(0, io.SeekStart)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

var flagExtensionsDir string

func discoverExtensions() ([]extension.Extension, error) {
	root, err := resolveExtensionsDir(flagExtensionsDir)
	if err != nil {
		return nil, err
	}
	return extension.Discover(root)
}

// claudePathForScope returns the Claude config file to patch for the scope.
func claudePathForScope(scope string) (string, error) {
	if scope == "project" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return claudeconfig.ClaudeProjectConfigPath(cwd), nil
	}
	return claudeconfig.ClaudeUserConfigPath() // user | global
}

func newMcpListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List extensions and their registration targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exts, err := discoverExtensions()
			if err != nil {
				return err
			}
			if len(exts) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no extensions found")
				return nil
			}
			for _, e := range exts {
				state := "enabled"
				if !e.Enabled {
					state = "disabled"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-8s %s  targets=%s scope=%s\n",
					e.Name, state, e.Transport, strings.Join(e.Targets, ","), e.Scope)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagExtensionsDir, "extensions-dir", "", "Override the dir containing extensions/ (else $VD_EXTENSIONS_DIR or cwd)")
	return cmd
}

func selectExtensions(names []string, includeDisabled bool) ([]extension.Extension, error) {
	all, err := discoverExtensions()
	if err != nil {
		return nil, err
	}
	byName := map[string]extension.Extension{}
	for _, e := range all {
		byName[e.Name] = e
	}
	if len(names) > 0 {
		var out []extension.Extension
		for _, n := range names {
			e, ok := byName[n]
			if !ok {
				return nil, fmt.Errorf("extension %q not found", n)
			}
			out = append(out, e)
		}
		return out, nil
	}
	var out []extension.Extension
	for _, e := range all {
		if e.Enabled || includeDisabled {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func registerExtension(e extension.Extension, scope string, w *strings.Builder) error {
	for _, target := range e.Targets {
		switch target {
		case "codex":
			path, err := claudeconfig.CodexConfigPath()
			if err != nil {
				return err
			}
			if err := claudeconfig.RegisterCodexMCP(path, e); err != nil {
				return fmt.Errorf("register %s in codex: %w", e.Name, err)
			}
			fmt.Fprintf(w, "  codex   → %s\n", path)
		case "claude":
			path, err := claudePathForScope(scope)
			if err != nil {
				return err
			}
			if err := claudeconfig.RegisterClaudeMCP(path, e); err != nil {
				return fmt.Errorf("register %s in claude: %w", e.Name, err)
			}
			fmt.Fprintf(w, "  claude  → %s (%s)\n", path, scope)
		}
	}
	return nil
}

func unregisterExtension(e extension.Extension, scope string, w *strings.Builder) error {
	for _, target := range e.Targets {
		switch target {
		case "codex":
			path, err := claudeconfig.CodexConfigPath()
			if err != nil {
				return err
			}
			if err := claudeconfig.UnregisterCodexMCP(path, e.Name); err != nil {
				return err
			}
			fmt.Fprintf(w, "  codex   ✕ %s\n", path)
		case "claude":
			path, err := claudePathForScope(scope)
			if err != nil {
				return err
			}
			if err := claudeconfig.UnregisterClaudeMCP(path, e.Name); err != nil {
				return err
			}
			fmt.Fprintf(w, "  claude  ✕ %s (%s)\n", path, scope)
		}
	}
	return nil
}

func newMcpInstallCmd() *cobra.Command {
	var scope string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "install [name...]",
		Short: "Register enabled extensions into Codex and Claude",
		RunE: func(cmd *cobra.Command, args []string) error {
			if scope != "project" && scope != "user" && scope != "global" {
				return fmt.Errorf("invalid --scope %q (project|user|global)", scope)
			}
			exts, err := selectExtensions(args, false)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, e := range exts {
				effScope := e.Scope
				if cmd.Flags().Changed("scope") {
					effScope = scope
				}
				var log strings.Builder
				if dryRun {
					for _, t := range e.Targets {
						fmt.Fprintf(&log, "  would register → %s\n", t)
					}
				} else if err := registerExtension(e, effScope, &log); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "%s:\n%s", e.Name, log.String())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "project", "Claude registration scope: project|user|global")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print planned registrations, write nothing")
	cmd.Flags().StringVar(&flagExtensionsDir, "extensions-dir", "", "Override the dir containing extensions/")
	return cmd
}

func newMcpEnableCmd() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Register an extension (alias for install <name>)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exts, err := selectExtensions(args, true)
			if err != nil {
				return err
			}
			var log strings.Builder
			if err := registerExtension(exts[0], scope, &log); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s enabled:\n%s", exts[0].Name, log.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "project", "Claude scope: project|user|global")
	cmd.Flags().StringVar(&flagExtensionsDir, "extensions-dir", "", "Override the dir containing extensions/")
	return cmd
}

func newMcpDisableCmd() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Unregister an extension from Codex and Claude",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exts, err := selectExtensions(args, true)
			if err != nil {
				return err
			}
			var log strings.Builder
			if err := unregisterExtension(exts[0], scope, &log); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s disabled:\n%s", exts[0].Name, log.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "project", "Claude scope: project|user|global")
	cmd.Flags().StringVar(&flagExtensionsDir, "extensions-dir", "", "Override the dir containing extensions/")
	return cmd
}

func newMcpDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check extensions: env preflight + launch reachability (basic)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exts, err := discoverExtensions()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, e := range exts {
				_, _ = fmt.Fprintf(out, "%s (%s):\n", e.Name, e.Transport)
				for _, name := range e.Env {
					if os.Getenv(name) == "" {
						_, _ = fmt.Fprintf(out, "  ⚠ env %s is not set\n", name)
					} else {
						_, _ = fmt.Fprintf(out, "  ✓ env %s set\n", name)
					}
				}
				switch e.Transport {
				case "stdio":
					if _, lookErr := exec.LookPath(e.Command); lookErr != nil {
						_, _ = fmt.Fprintf(out, "  ✕ command %q not found on PATH\n", e.Command)
					} else {
						_, _ = fmt.Fprintf(out, "  ✓ command %q found\n", e.Command)
					}
				case "http":
					if e.URL == "" {
						_, _ = fmt.Fprintln(out, "  ✕ url not set")
					} else {
						_, _ = fmt.Fprintf(out, "  ✓ url %s\n", e.URL)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagExtensionsDir, "extensions-dir", "", "Override the dir containing extensions/")
	return cmd
}
