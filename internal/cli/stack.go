package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
	"github.com/joebalancio/wt/internal/stack"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/spf13/cobra"
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
			runStackCommand(cmd, args, stackBase, stackForce, noSetup)
		},
	}

	cmd.Flags().StringVar(&stackBase, "base", "", "base branch for stack (default: current)")
	cmd.Flags().BoolVar(&stackForce, "force", false, "allow stacking on main/master")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "skip setup hooks and worktree creation")

	return cmd
}

func runStackCommand(cmd *cobra.Command, args []string, stackBase string, stackForce bool, noSetup bool) {
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

	stackService := initStackService()

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
		createStackWorktree(ctx, cmd, stackService, stackBranch.Name)
	}
}

// initStackService initializes git client, config, and stack service
func initStackService() *stack.Service {
	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

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

	stackService, err := stack.NewService(gitClient, spiceClient, cfg)
	if err != nil {
		Fatal("Failed to create stack service: %v", err)
	}

	return stackService
}

// createStackWorktree creates a worktree for the stack branch and sets up hooks and tmux
func createStackWorktree(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string) {
	out := cmd.OutOrStdout()

	worktree, err := stackService.CreateWorktree(ctx, branchName)
	if err != nil {
		Fatal("Failed to create worktree: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Run setup hooks
	if err := runSetupHooks(ctx, worktree.Path); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}

	// Create tmux window if in tmux and not disabled
	createStackTmuxWindow(ctx, cmd, stackService, branchName, worktree.Path)
}

// createStackTmuxWindow creates a tmux window for the stack branch
func createStackTmuxWindow(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	// Get stack level for window naming
	stackLevel := getStackLevel(ctx, stackService, branchName)

	windowName := tmux.GenerateStackWindowName(branchName, stackLevel)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
}

// getStackLevel returns the stack level for a given branch name
func getStackLevel(ctx context.Context, stackService *stack.Service, branchName string) int {
	stackBranches, _ := stackService.GetStack(ctx)
	for i, sb := range stackBranches {
		if sb.Name == branchName {
			return i
		}
	}
	return 0
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

			// Create clients
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			// Load config
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

			// Create stack service
			stackService, err := stack.NewService(gitClient, spiceClient, cfg)
			if err != nil {
				Fatal("Failed to create stack service: %v", err)
			}

			// Get stack with worktree paths
			branches, err := stackService.GetStack(ctx)
			if err != nil {
				Fatal("Failed to get stack: %v", err)
			}

			// Get current branch for highlighting
			currentBranch, _ := gitClient.GetCurrentBranch(ctx)
			for _, branch := range branches {
				if branch.Name == currentBranch {
					branch.IsHead = true
				}
			}

			// Format and display tree
			treeOutput := stack.FormatStackTree(branches)
			if _, err := fmt.Fprint(out, treeOutput); err != nil {
				Fatal("Failed to write output: %v", err)
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
