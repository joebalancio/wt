package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/joebalancio/wt/pkg/executor"
	"github.com/spf13/cobra"
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

The command ALWAYS creates a new branch with the specified name.
The new branch is created from the base branch (default: current HEAD).
If a branch with the same name already exists, the command will fail.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := args[0]

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

			// Run setup hooks
			if err := runSetupHooks(ctx, worktree.Path); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
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
	RegisterCommand(NewAddCmd())
}

// runSetupHooks executes post-create hooks for a worktree
func runSetupHooks(ctx context.Context, worktreePath string) error {
	cfg, err := loadConfigForCommand()
	if err != nil {
		return err
	}

	runner := executor.NewHookRunner(worktreePath)
	return runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate)
}
