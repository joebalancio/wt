package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/picker"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
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

If the branch does not exist, it will be created from the base branch (default: current HEAD).
If the branch already exists, a worktree will be added for that existing branch.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := ""
			if len(args) > 0 {
				branch = args[0]
			}
			runAddCommand(cmd, branch, base, path, force, track, noCheckout)
		},
	}

	cmd.Flags().StringVar(&base, "base", "", "base branch for new branch")
	cmd.Flags().StringVar(&path, "path", "", "custom path for worktree")
	cmd.Flags().BoolVar(&force, "force", false, "force creation even if path exists")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")

	return cmd
}

func runAddCommand(cmd *cobra.Command, branch, base, path string, force bool, track string, noCheckout bool) {
	ctx := context.Background()

	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	// Check if we're inside a worktree - this is not allowed
	inWorktree, mainRepoRoot, err := gitClient.IsInWorktree(ctx)
	if err != nil {
		Fatal("Failed to check worktree context: %v", err)
	}

	if inWorktree {
		// Get current toplevel for the error message
		repoInfo, err := gitClient.GetRepoInfo(ctx)
		currentPath := "unknown"
		if err == nil {
			currentPath = repoInfo.RootPath
		}

		Fatal(`cannot add worktree from inside another worktree

Current location: %s
Main repository:  %s

Run this command from the main repository instead:
  cd %s && wt add %s`,
			currentPath,
			mainRepoRoot,
			mainRepoRoot,
			branch)
	}

	if branch == "" {
		if !picker.IsTerminal() {
			Fatal("branch argument required when not in interactive terminal")
		}
		p := picker.NewPicker(gitClient)
		result, err := p.SelectBranch(ctx)
		if err != nil {
			Fatal("Branch selection failed: %v", err)
		}
		branch = result.Branch
		if result.IsNew && base == "" {
			base = result.BaseBranch
		}
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

	wt, err := svc.Add(ctx, spec)
	if err != nil {
		Fatal("Failed to add worktree: %v", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s [%s]\n", wt.Path, wt.Branch); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Setup tmux window and run hooks
	setupWorktreeWithTmux(ctx, cmd, wt.Branch, wt.Path)
}

// setupWorktreeWithTmux creates tmux window before running hooks
func setupWorktreeWithTmux(ctx context.Context, cmd *cobra.Command, branch, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		// Not in tmux or --no-tmux: run hooks locally
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		// Fall back to local hooks if tmux unavailable
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
		return
	}

	windowName := tmux.GenerateWindowName(branch)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
		// Still try to run hooks locally
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
		return
	}

	// Select the window so user sees it
	_ = tmuxClient.SelectWindow(windowName)

	// Run hooks INSIDE the new window
	if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}
}

func init() {
	RegisterCommand(NewAddCmd())
}
