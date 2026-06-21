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
	cmd.Flags().StringVar(&opts.hookPath, "hook-path", "", "Override dev-rules-reminder.cjs path")
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
		sessionID = os.Getenv("VD_SESSION_ID")
	}

	hookPath, err := resolveContextHookPath(opts.hookPath)
	if err != nil {
		return err
	}

	raw, err := runContextHook(cwd, sessionID, hookPath)
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

func resolveContextHookPath(override string) (string, error) {
	if override != "" {
		return validateHookPath(override)
	}
	home, err := os.UserHomeDir()
	if err == nil {
		for _, candidate := range []string{
			filepath.Join(home, ".codex", "hooks", "dev-rules-reminder.cjs"),
			filepath.Join(home, ".claude", "hooks", "dev-rules-reminder.cjs"),
		} {
			if path, err := validateHookPath(candidate); err == nil {
				return path, nil
			}
		}
	}
	if root, err := resolveRepoRoot(flagRoot); err == nil {
		if path, err := validateHookPath(filepath.Join(root, "hooks", "dev-rules-reminder.cjs")); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("dev-rules-reminder.cjs not found in ~/.codex/hooks, ~/.claude/hooks, or the vd repo hooks dir")
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

func runContextHook(cwd, sessionID, hookPath string) ([]byte, error) {
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

	ext := exec.Command("node", hookPath)
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
