package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a worktree",
		Long: `Remove a worktree from the repository.

By default, this will fail if the worktree has uncommitted changes.
Use --force to remove it anyway.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := args[0]

			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			cfg, err := loadConfigForCommand()
			if err != nil {
				Fatal("Failed to load config: %v", err)
			}

			svc, err := worktree.NewService(gitClient, cfg)
			if err != nil {
				Fatal("Failed to create service: %v", err)
			}

			if err := svc.Remove(ctx, path, force); err != nil {
				Fatal("Failed to remove worktree: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", path)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force removal even with uncommitted changes")

	return cmd
}

func init() {
	RegisterCommand(NewRemoveCmd())
}
