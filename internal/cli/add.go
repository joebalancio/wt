package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
	"github.com/user/wt/pkg/domain"
)

// NewAddCmd creates the add command
func NewAddCmd() *cobra.Command {
	var (
		base       string
		path       string
		force      bool
		track      string
		noCheckout bool
	)

	cmd := &cobra.Command{
		Use:   "add <branch>",
		Short: "Add a new worktree",
		Long: `Add a new worktree for the specified branch.

If the branch already exists, it will be checked out in the new worktree.
If the branch doesn't exist, it will be created from the specified base branch.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := args[0]

			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			svc, err := worktree.NewService(gitClient)
			if err != nil {
				Fatal("Failed to create service: %v", err)
			}

			spec := domain.WorktreeCreateSpec{
				Branch:   branch,
				Base:     base,
				Path:     path,
				Force:    force,
				Checkout: !noCheckout,
			}

			if track != "" {
				spec.Track = &track
			}

			worktree, err := svc.Add(ctx, spec)
			if err != nil {
				Fatal("Failed to add worktree: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s [%s]\n", worktree.Path, worktree.Branch)
		},
	}

	cmd.Flags().StringVar(&base, "base", "", "base branch for new branch")
	cmd.Flags().StringVar(&path, "path", "", "custom path for worktree")
	cmd.Flags().BoolVar(&force, "force", false, "force creation even if path exists")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")

	return cmd
}

func init() {
	// Register as a child of worktreeCmd
	worktreeCmd.AddCommand(NewAddCmd())
}
