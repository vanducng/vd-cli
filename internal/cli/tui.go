package cli

import (
	"github.com/spf13/cobra"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
	"github.com/vanducng/vd-cli/v2/internal/ui/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Browse skills, assets, and hooks in a terminal UI",
		Long: `Open an interactive terminal UI over the same inventory backend as 'vd web':
tracked skills (with drift), assets discovered under ~/.claude, and hooks.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := resolveRepoRoot(flagRoot)
			if err != nil {
				return err
			}
			claudeHome, err := claudeDir()
			if err != nil {
				return err
			}
			return tui.Run(inventory.NewService(root, claudeHome))
		},
	}
}
