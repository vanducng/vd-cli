package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type contextPrintOptions struct {
	cwd       string
	sessionID string
	jsonOut   bool
	hookPath  string
}

const (
	contextHookFilePy  = "dev-rules-reminder.py"
	contextHookFileCjs = "dev-rules-reminder.cjs"
	sessionIDEnvVar    = "VD_SESSION_ID"
)

// contextHookRuntime returns the interpreter for a resolved context hook: the
// Python hook runs via python3, the legacy Node hook (or any other override)
// via node.
func contextHookRuntime(path string) string {
	if strings.HasSuffix(path, ".py") {
		return "python3"
	}
	return "node"
}

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context <subcommand>",
		Short: "Print vd runtime context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("unknown subcommand %q (valid: print)", args[0])
		},
	}
	cmd.AddCommand(newContextPrintCmd())
	return cmd
}

func newContextPrintCmd() *cobra.Command {
	var opts contextPrintOptions
	cmd := &cobra.Command{
		Use:   "print",
		Short: "Print the injected path and naming context",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runContextPrint(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.cwd, "cwd", "", "Working directory to resolve project config from")
	cmd.Flags().StringVar(&opts.sessionID, "session-id", "", "Session id used for plan path resolution")
	cmd.Flags().BoolVar(&opts.jsonOut, "json", false, "Print hook JSON instead of plain context")
	cmd.Flags().StringVar(&opts.hookPath, "hook-path", "", "Override the dev-rules-reminder hook path (.py runs via python3, else node)")
	return cmd
}

func runContextPrint(cmd *cobra.Command, opts contextPrintOptions) error {
	cwd := opts.cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("determine cwd: %w", err)
		}
	}
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("resolve cwd: %w", err)
	}
	if info, err := os.Stat(cwd); err != nil {
		return fmt.Errorf("stat cwd %s: %w", cwd, err)
	} else if !info.IsDir() {
		return fmt.Errorf("cwd %s is not a directory", cwd)
	}

	sessionID := opts.sessionID
	if sessionID == "" {
		sessionID = os.Getenv(sessionIDEnvVar)
	}

	hookPath, runtime, err := resolveContextHook(opts.hookPath)
	if err != nil {
		return err
	}

	raw, err := runContextHook(cwd, sessionID, hookPath, runtime)
	if err != nil {
		return err
	}
	if opts.jsonOut {
		_, _ = cmd.OutOrStdout().Write(raw)
		if len(raw) == 0 || raw[len(raw)-1] != '\n' {
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}
		return nil
	}

	text, err := extractAdditionalContext(raw)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), text)
	return nil
}

// resolveContextHook finds the dev-rules-reminder hook and the interpreter to
// run it with. An explicit override wins; otherwise each hooks dir (in
// precedence order) is scanned preferring the Python hook over the legacy Node
// one, so a machine that upgrades vd before re-running `vd install hooks` still
// resolves the .cjs it already has.
func resolveContextHook(override string) (path, runtime string, err error) {
	if override != "" {
		p, verr := validateHookPath(override)
		if verr != nil {
			return "", "", verr
		}
		return p, contextHookRuntime(p), nil
	}

	var dirs []string
	if home, herr := os.UserHomeDir(); herr == nil {
		dirs = append(dirs,
			filepath.Join(home, ".codex", "hooks"),
			filepath.Join(home, ".claude", "hooks"),
		)
	}
	if root, rerr := resolveRepoRoot(flagRoot); rerr == nil {
		dirs = append(dirs, filepath.Join(root, "hooks"))
	}

	if p, rt, ok := findContextHookIn(dirs); ok {
		return p, rt, nil
	}
	return "", "", fmt.Errorf("dev-rules-reminder.{py,cjs} not found in ~/.codex/hooks, ~/.claude/hooks, or the vd repo hooks dir")
}

// findContextHookIn returns the first dev-rules-reminder hook found across dirs
// (in order) with its runtime, preferring the Python hook over the Node one
// within each dir.
func findContextHookIn(dirs []string) (path, runtime string, ok bool) {
	for _, dir := range dirs {
		for _, name := range []string{contextHookFilePy, contextHookFileCjs} {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, contextHookRuntime(candidate), true
			}
		}
	}
	return "", "", false
}

func validateHookPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", abs)
	}
	return abs, nil
}

func runContextHook(cwd, sessionID, hookPath, runtime string) ([]byte, error) {
	payload := map[string]string{
		"cwd":             cwd,
		"hook_event_name": "UserPromptSubmit",
	}
	if sessionID != "" {
		payload["session_id"] = sessionID
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	bin, err := exec.LookPath(runtime)
	if err != nil {
		return nil, fmt.Errorf("%s not found in PATH: %w", runtime, err)
	}
	ext := exec.Command(bin, hookPath)
	ext.Dir = cwd
	ext.Stdin = bytes.NewReader(append(data, '\n'))
	var stdout, stderr bytes.Buffer
	ext.Stdout = &stdout
	ext.Stderr = &stderr
	if err := ext.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("run %s: %w: %s", hookPath, err, detail)
		}
		return nil, fmt.Errorf("run %s: %w", hookPath, err)
	}
	if stdout.Len() == 0 {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("context hook produced no output: %s", detail)
		}
		return nil, fmt.Errorf("context hook produced no output")
	}
	return stdout.Bytes(), nil
}

func extractAdditionalContext(raw []byte) (string, error) {
	var parsed struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse hook JSON: %w", err)
	}
	if parsed.HookSpecificOutput.AdditionalContext == "" {
		return "", fmt.Errorf("hook JSON did not include additionalContext")
	}
	return parsed.HookSpecificOutput.AdditionalContext, nil
}
