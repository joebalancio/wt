package cli

import (
	"context"
	"fmt"
	"io"

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
		run        string
	)

	cmd := &cobra.Command{
		Use:   "add <branch>",
		Short: "Add a new worktree",
		Long: `Add a new worktree for the specified branch.

If the branch does not exist, it will be created from the base branch (default: current HEAD).
If the branch already exists, a worktree will be added for that existing branch.

Use --run to execute a command after hooks complete:
  wt add feat/auth --run "claude"

Template variables:
  {worktree_path} - Path to the new worktree
  {branch} - Branch name`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := ""
			if len(args) > 0 {
				branch = args[0]
			}
			runAddCommand(cmd, branch, base, path, force, track, noCheckout, run)
		},
	}

	cmd.Flags().StringVar(&base, "base", "", "base branch for new branch")
	cmd.Flags().StringVar(&path, "path", "", "custom path for worktree")
	cmd.Flags().BoolVar(&force, "force", false, "force creation even if path exists")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")
	cmd.Flags().StringVar(&run, "run", "", "command to run after hooks (e.g., 'claude')")

	return cmd
}

func runAddCommand(cmd *cobra.Command, branch, base, path string, force bool, track string, noCheckout bool, run string) {
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

	// Get or create worktree
	wt := getOrCreateWorktree(ctx, svc, branch, base, path, force, track, noCheckout, cmd.OutOrStdout())

	// Setup tmux window and run hooks
	setupWorktreeWithTmux(ctx, cmd, wt.Branch, wt.Path, run)
}

// getOrCreateWorktree returns an existing worktree for the branch or creates a new one
func getOrCreateWorktree(ctx context.Context, svc *worktree.Service, branch, base, path string, force bool, track string, noCheckout bool, stdout interface{ io.Writer }) *domain.Worktree {
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

	wt, created, err := svc.GetOrCreate(ctx, spec)
	if err != nil {
		Fatal("Failed to get or create worktree: %v", err)
	}

	// Output appropriate message
	if created {
		if _, err := fmt.Fprintf(stdout, "Created worktree: %s [%s]\n", wt.Path, wt.Branch); err != nil {
			Fatal("Failed to write output: %v", err)
		}
	} else {
		if _, err := fmt.Fprintf(stdout, "Selected worktree: %s [%s]\n", wt.Path, wt.Branch); err != nil {
			Fatal("Failed to write output: %v", err)
		}
	}
	return wt
}

// setupWorktreeWithTmux creates tmux window before running hooks
func setupWorktreeWithTmux(ctx context.Context, cmd *cobra.Command, branch, worktreePath, runCmd string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		runSetupHooksWithWarning(ctx, cmd, worktreePath)
		runCommandLocallyOrFatal(branch, worktreePath, runCmd)
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		runSetupHooksWithWarning(ctx, cmd, worktreePath)
		runCommandLocallyOrFatal(branch, worktreePath, runCmd)
		return
	}

	windowName := tmux.GenerateWindowName(branch)
	windowExisted, _ := tmuxClient.WindowExists(windowName)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
		runSetupHooksWithWarning(ctx, cmd, worktreePath)
		return
	}

	// Select the window so user sees it
	_ = tmuxClient.SelectWindow(windowName)

	// Run hooks INSIDE the new window
	if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}
	if runCmd != "" {
		_ = runCommandAfterHooks(RunCommandOpts{
			Command:       runCmd,
			WorktreePath:  worktreePath,
			Branch:        branch,
			WindowName:    windowName,
			TmuxClient:    tmuxClient,
			WindowExisted: windowExisted,
			InTmux:        true,
		})
	}
}

func init() {
	RegisterCommand(NewAddCmd())
}
