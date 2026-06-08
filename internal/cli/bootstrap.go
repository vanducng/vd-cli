package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/bootstrap"
)

func newBootstrapCmd() *cobra.Command {
	var (
		repoURL string
		ref     string
		update  bool
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Fetch the skills content repo into ~/.vd/skills",
		Long: `Clone the skills content repository (default vanducng/skills) into
$HOME/.vd/skills at the latest release tag, so 'vd install' has skills to
install from. Re-run with --update to move to the latest release.

Override the source with --repo or the VD_SKILLS_REPO env var; pin a specific
tag or branch with --ref.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := bootstrap.Run(cmd.Context(), bootstrap.Options{
				RepoURL: repoURL,
				Ref:     ref,
				Update:  update,
				DryRun:  dryRun,
			})
			if err != nil {
				return err
			}
			if flagQuiet {
				return nil
			}
			out := cmd.OutOrStdout()
			switch res.Action {
			case "already-present":
				_, _ = fmt.Fprintf(out, "skills already present at %s (%s); use --update to move to the latest release\n", res.Root, res.Ref)
			case "already-current":
				_, _ = fmt.Fprintf(out, "skills already at latest release %s (%s)\n", res.Ref, res.Root)
			case "would-clone":
				_, _ = fmt.Fprintf(out, "would clone %s@%s -> %s\n", bootstrap.RepoURL(), res.Ref, res.Root)
			case "would-update":
				_, _ = fmt.Fprintf(out, "would update %s -> %s\n", res.Root, res.Ref)
			default: // cloned | updated
				_, _ = fmt.Fprintf(out, "%s skills %s at %s\n", res.Action, res.Ref, res.Root)
				_, _ = fmt.Fprintln(out, "run 'vd install' to symlink skills into your agents")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Skills repo URL (default vanducng/skills or $VD_SKILLS_REPO)")
	cmd.Flags().StringVar(&ref, "ref", "", "Pin a specific tag or branch instead of the latest release")
	cmd.Flags().BoolVar(&update, "update", false, "Update an existing ~/.vd/skills to the target release")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print actions without changing files")

	return cmd
}
