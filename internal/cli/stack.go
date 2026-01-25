package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/spice"
	"github.com/user/wt/internal/stack"
)

// NewStackCmd creates the stack command group
func NewStackCmd() *cobra.Command {
	var (
		stackBase  string
		stackForce bool
		noSetup    bool
	)

	cmd := &cobra.Command{
		Use:   "stack [name]",
		Short: "Create a stacked branch",
		Long: `Create a new stacked branch on top of the current branch.

If no name is provided, generates an auto-suffix (4 chars).
If a name is provided, appends it with a 4-char suffix.

Examples:
  wt stack              # Creates: currentBranch-xY7k
  wt stack api          # Creates: currentBranch-api-k9P2`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Check for main/master protection
			currentBranch, err := getCurrentBranchProtected(ctx)
			if err != nil {
				Fatal("Failed to get current branch: %v", err)
			}

			if !stackForce && isProtectedBranch(currentBranch) {
				Fatal("Cannot stack on '%s'. Stack on feature branches only.\nUse --force to override.", currentBranch)
			}

			// Create clients and service
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			spiceClient, err := spice.NewClient()
			if err != nil {
				Fatal("Failed to create git-spice client: %v", err)
			}

			cfg, err := loadConfigForCommand()
			if err != nil {
				Fatal("Failed to load config: %v", err)
			}

			stackService, err := stack.NewService(gitClient, spiceClient, cfg)
			if err != nil {
				Fatal("Failed to create stack service: %v", err)
			}

			// Get the optional name argument
			var name string
			if len(args) > 0 {
				name = args[0]
			}

			// Create the stack branch
			spec := stack.StackBranchSpec{
				Name: name,
				Base: stackBase,
			}

			stackBranch, err := stackService.CreateStackBranch(ctx, spec)
			if err != nil {
				Fatal("Failed to create stack branch: %v", err)
			}

			fmt.Fprintf(out, "Created stacked branch: %s\n", stackBranch.Name)

			// Create worktree
			if !noSetup {
				worktree, err := stackService.CreateWorktree(ctx, stackBranch.Name)
				if err != nil {
					Fatal("Failed to create worktree: %v", err)
				}
				fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path)

				// TODO: Run setup hooks (will be added in Phase 4)
			}
		},
	}

	cmd.Flags().StringVar(&stackBase, "base", "", "base branch for stack (default: current)")
	cmd.Flags().BoolVar(&stackForce, "force", false, "allow stacking on main/master")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "skip setup hooks and worktree creation")

	return cmd
}

// NewStackListCmd creates the stack list subcommand
func NewStackListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show stack hierarchy with paths",
		Long:  `Display the current stack as a tree with branch names and worktree paths.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			spiceClient, err := spice.NewClient()
			if err != nil {
				Fatal("Failed to create git-spice client: %v", err)
			}

			// Get stack from git-spice
			branches, err := spiceClient.GetStack(ctx)
			if err != nil {
				Fatal("Failed to get stack: %v", err)
			}

			// Get current branch for highlighting
			gitClient, err := git.NewClient()
			if err == nil {
				currentBranch, _ := gitClient.GetCurrentBranch(ctx)
				// TODO: Format tree display with current marker
				_ = currentBranch
			}

			// Simple display for now
			for _, branch := range branches {
				fmt.Fprintf(out, "%s\n", branch.Name)
			}
		},
	}

	return cmd
}

func getCurrentBranchProtected(ctx context.Context) (string, error) {
	gitClient, err := git.NewClient()
	if err != nil {
		return "", err
	}
	return gitClient.GetCurrentBranch(ctx)
}

func isProtectedBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

func loadConfigForCommand() (*config.Config, error) {
	configPath, err := config.FindConfig("")
	if err != nil {
		return config.DefaultConfig(), nil
	}
	return config.Load(configPath)
}

func init() {
	stackCmd := NewStackCmd()
	stackCmd.AddCommand(NewStackListCmd())
	RegisterCommand(stackCmd)
}
