package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
	"github.com/joebalancio/wt/internal/stack"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/spf13/cobra"
)

// NewStackCmd creates the stack command group
func NewStackCmd() *cobra.Command {
	var (
		stackBase  string
		stackForce bool
		noSetup    bool
		path       string
		track      string
		noCheckout bool
	)

	cmd := &cobra.Command{
		Use:   "stack [name]",
		Short: "Create a stacked branch",
		Long: `Create a new stacked branch on top of the current branch.

If no name is provided, generates an auto-suffix (4 chars).
If a name is provided, appends it with a 4-char suffix.

Examples:
  wt stack              # Creates: currentBranch-xY7k
  wt stack api          # Creates: currentBranch-api-k9P2
  wt stack api --path /custom/path
  wt stack api --track origin/api`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runStackCommand(cmd, args, stackBase, stackForce, noSetup, path, track, noCheckout)
		},
	}

	cmd.Flags().StringVar(&stackBase, "base", "", "base branch for stack (default: current)")
	cmd.Flags().BoolVar(&stackForce, "force", false, "allow stacking on main/master")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "skip setup hooks and worktree creation")
	cmd.Flags().StringVar(&path, "path", "", "custom path for worktree")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")

	return cmd
}

func runStackCommand(cmd *cobra.Command, args []string, stackBase string, stackForce bool, noSetup bool, path string, track string, noCheckout bool) {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	inWorktree, mainRepoRoot, err := gitClient.IsInWorktree(ctx)
	if err != nil {
		Fatal("Failed to check worktree context: %v", err)
	}

	if inWorktree {
		repoInfo, err := gitClient.GetRepoInfo(ctx)
		currentPath := "unknown"
		if err == nil && repoInfo != nil {
			currentPath = repoInfo.RootPath
		}

		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		Fatal(`cannot stack from inside another worktree

Current location: %s
Main repository:  %s

Run this command from the main repository instead:
  cd %s && wt stack %s`,
			currentPath,
			mainRepoRoot,
			mainRepoRoot,
			name)
	}

	// Check for main/master protection
	currentBranch, err := gitClient.GetCurrentBranch(ctx)
	if err != nil {
		Fatal("Failed to get current branch: %v", err)
	}

	if !stackForce && isProtectedBranch(currentBranch) {
		Fatal("Cannot stack on '%s'. Stack on feature branches only.\nUse --force to override.", currentBranch)
	}

	stackService := initStackServiceWithWorktree(gitClient)

	// Get the optional name argument
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// Create the stack branch
	spec := stack.BranchSpec{
		Name: name,
		Base: stackBase,
	}

	stackBranch, err := stackService.CreateStackBranch(ctx, spec)
	if err != nil {
		Fatal("Failed to create stack branch: %v", err)
	}

	if _, err := fmt.Fprintf(out, "Created stacked branch: %s\n", stackBranch.Name); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Create worktree
	if !noSetup {
		wtSpec := stack.WorktreeSpec{
			Path:       path,
			Track:      track,
			NoCheckout: noCheckout,
		}
		createStackWorktreeWithSpec(ctx, cmd, stackService, stackBranch.Name, wtSpec)
	}
}

// initStackServiceWithWorktree initializes stack service with shared git client.
func initStackServiceWithWorktree(gitClient *git.Client) *stack.Service {
	cfg, err := loadConfigForCommand()
	if err != nil {
		Fatal("Failed to load config: %v", err)
	}

	// Validate git-spice configuration early
	if err := validateGitSpiceConfig(cfg); err != nil {
		Fatal("%v", err)
	}

	spiceClient, err := spice.NewClient(cfg)
	if err != nil {
		Fatal("Failed to create spice client: %v", err)
	}

	worktreeSvc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		Fatal("Failed to create worktree service: %v", err)
	}

	stackService, err := stack.NewService(gitClient, spiceClient, cfg, worktreeSvc)
	if err != nil {
		Fatal("Failed to create stack service: %v", err)
	}

	return stackService
}

// createStackWorktreeWithSpec creates a worktree for the stack branch and sets up hooks and tmux.
func createStackWorktreeWithSpec(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string, spec stack.WorktreeSpec) {
	out := cmd.OutOrStdout()

	worktree, err := stackService.CreateWorktreeWithSpec(ctx, branchName, spec)
	if err != nil {
		Fatal("Failed to create worktree: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	setupStackWorktree(ctx, cmd, stackService, branchName, worktree.Path)
}

func setupStackWorktree(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string, worktreePath string) {
	// NEW ORDER: Create tmux window BEFORE running hooks
	if shouldCreateTmuxWindow(NoTmux()) {
		tmuxClient, err := tmux.NewClient()
		if err != nil {
			// Fall back to local hooks
			if err := runSetupHooks(ctx, worktreePath); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
			return
		}

		// Get stack level for window naming
		stackLevel := getStackLevel(ctx, stackService, branchName)
		windowName := tmux.GenerateStackWindowName(branchName, stackLevel)

		if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
			if err := runSetupHooks(ctx, worktreePath); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
			return
		}

		// Select the window
		_ = tmuxClient.SelectWindow(windowName)

		// Run hooks INSIDE the new window
		if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
	} else {
		// Not in tmux or --no-tmux: run hooks locally
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
	}
}

// NewStackListCmd creates the stack list subcommand
func NewStackListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show stack hierarchy with paths",
		Long:  `Display the current stack as a tree with branch names and worktree paths.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			runStackListCommand(cmd)
		},
	}

	return cmd
}

func runStackListCommand(cmd *cobra.Command) {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	cfg, err := loadConfigForCommand()
	if err != nil {
		Fatal("Failed to load config: %v", err)
	}
	if err := validateGitSpiceConfig(cfg); err != nil {
		Fatal("%v", err)
	}

	spiceClient, err := spice.NewClient(cfg)
	if err != nil {
		Fatal("Failed to create spice client: %v", err)
	}

	worktreeSvc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		Fatal("Failed to create worktree service: %v", err)
	}

	stackService, err := stack.NewService(gitClient, spiceClient, cfg, worktreeSvc)
	if err != nil {
		Fatal("Failed to create stack service: %v", err)
	}

	branches, err := stackService.GetStack(ctx)
	if err != nil {
		Fatal("Failed to get stack: %v", err)
	}

	currentBranch, _ := gitClient.GetCurrentBranch(ctx)
	for _, branch := range branches {
		if branch.Name == currentBranch {
			branch.IsHead = true
		}
	}

	treeOutput := stack.FormatStackTree(branches)
	if _, err := fmt.Fprint(out, treeOutput); err != nil {
		Fatal("Failed to write output: %v", err)
	}
}

func isProtectedBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

// validateGitSpiceConfig validates git-spice configuration
// Returns an error if git-spice is not configured
func validateGitSpiceConfig(cfg *config.Config) error {
	if cfg.Spice.BinaryPath == "" {
		return fmt.Errorf("git-spice not configured.\n\n" +
			"Run 'wt init' to configure git-spice.\n\n" +
			"Install git-spice first if needed:\n" +
			"  cargo install git-spice\n" +
			"  brew install git-spice")
	}
	return nil
}

func init() {
	stackCmd := NewStackCmd()
	stackCmd.AddCommand(NewStackListCmd())
	RegisterCommand(stackCmd)
}
