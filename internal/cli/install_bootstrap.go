package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/bootstrap"
	"github.com/vanducng/vd-cli/v2/internal/config"
)

// resolveInstallRoot finds the skills repo to install from. Precedence:
// --root, VD_ROOT, a manifest-bearing repo in the CWD ancestry, then the
// bootstrapped home (~/.vd/skills). When nothing is set up it offers to fetch
// the skills repo (interactive) or errors with a hint (non-interactive).
func resolveInstallRoot(cmd *cobra.Command, override string, dryRun bool) (string, error) {
	if override != "" {
		return validateRootDir(override, "--root")
	}
	if env := os.Getenv(rootEnvVar); env != "" {
		return validateRootDir(env, rootEnvVar)
	}

	if cwd, err := os.Getwd(); err == nil {
		if root, ferr := config.FindRepoRoot(cwd); ferr == nil && hasManifest(root) {
			return root, nil
		}
	}

	home, _ := bootstrap.DefaultRoot()
	if bootstrap.IsBootstrapped(home) {
		return home, nil
	}

	return bootstrapInteractive(cmd, dryRun, home)
}

func bootstrapInteractive(cmd *cobra.Command, dryRun bool, home string) (string, error) {
	repoURL := bootstrap.RepoURL()
	if dryRun {
		return "", fmt.Errorf("no skills found; run 'vd bootstrap' to fetch %s into %s", repoURL, home)
	}
	if !isInteractive(cmd.InOrStdin()) {
		return "", fmt.Errorf("no skills found; run 'vd bootstrap' to fetch %s into %s", repoURL, home)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, "No skills found locally.")
	_, _ = fmt.Fprintf(out, "Fetch %s into %s now? [Y/n]: ", repoURL, home)

	reader := bufio.NewReader(cmd.InOrStdin())
	ans, err := reader.ReadString('\n')
	if err != nil && ans == "" {
		return "", fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "", "y", "yes":
	default:
		return "", fmt.Errorf("aborted; run 'vd bootstrap' when ready")
	}

	res, err := bootstrap.Run(context.Background(), bootstrap.Options{})
	if err != nil {
		return "", err
	}
	if !flagQuiet {
		_, _ = fmt.Fprintf(out, "%s skills %s at %s\n", res.Action, res.Ref, res.Root)
	}
	return res.Root, nil
}

func hasManifest(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "skills.toml"))
	return err == nil
}

func isInteractive(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
