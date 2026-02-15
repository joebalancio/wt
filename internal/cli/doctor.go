package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
	"github.com/spf13/cobra"
)

// NewDoctorCmd creates the doctor command
func NewDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks on wt installation and environment",
		Long: `Check wt installation, dependencies, configuration, and repository status.

This command verifies that:
- wt binary is correctly installed
- git and git worktree support are available
- git-spice is installed (required for stacking)
- configuration files are valid
- current repository is ready for stacking operations`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			runDoctor(cmd)
		},
	}
}

func runDoctor(cmd *cobra.Command) {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	allPass := true

	// Check wt installation
	if _, err := fmt.Fprintln(out, "Checking wt installation..."); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if !checkWTBinary(ctx, out) {
		allPass = false
	}

	// Check dependencies
	if _, err := fmt.Fprintln(out, "\nChecking dependencies..."); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	gitOK, gitSpiceOK := checkDependencies(ctx, out)
	if !gitOK {
		allPass = false
	}

	// Check configuration
	if _, err := fmt.Fprintln(out, "\nChecking configuration..."); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if !checkConfiguration(out) {
		allPass = false
	}

	// Check repository (only if git is available)
	if gitOK {
		if _, err := fmt.Fprintln(out, "\nChecking current repository..."); err != nil {
			Fatal("Failed to write output: %v", err)
		}
		checkRepoDoctor(ctx, out, gitSpiceOK)
		// Git-spice missing is not critical for basic operations
	}

	// Summary
	if _, err := fmt.Fprintln(out); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if allPass {
		if _, err := fmt.Fprintln(out, "All checks passed!"); err != nil {
			Fatal("Failed to write output: %v", err)
		}
		os.Exit(0)
	}

	// Some checks failed
	if gitOK {
		// Git is OK but other checks failed
		os.Exit(2) // Warning
	}
	// Critical failure - git not available
	os.Exit(1)
}

func checkWTBinary(_ context.Context, out io.Writer) bool {
	// Get wt binary path
	execPath, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintf(out, "! wt binary path unknown: %v\n", err)
		return false
	}
	_, _ = fmt.Fprintf(out, "✓ wt binary: %s\n", execPath)

	// Get version
	_, _ = fmt.Fprintf(out, "✓ Version: %s\n", "v2.0.0")
	_, _ = fmt.Fprintf(out, "✓ OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	return true
}

func checkDependencies(ctx context.Context, out io.Writer) (gitOK bool, gitSpiceOK bool) {
	// Check git
	_, err := git.NewClient()
	if err != nil {
		_, _ = fmt.Fprintf(out, "✗ git not found\n")
		return false, false
	}

	// Check git version
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "--version")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(out, "✗ git installed but version check failed\n")
		return true, false
	}
	gitVersion := strings.TrimSpace(stdout.String())
	_, _ = fmt.Fprintf(out, "✓ git installed: %s\n", gitVersion)

	// Check git worktree support
	cmd = exec.CommandContext(ctx, "git", "worktree", "list")
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(out, "✗ git worktree not supported\n")
		return true, false
	}
	_, _ = fmt.Fprintf(out, "✓ git worktree supported\n")

	// Check git-spice
	cfg, err := loadConfigForCommand()
	if err != nil {
		_, _ = fmt.Fprintf(out, "! Failed to load config: %v\n", err)
		return true, false
	}

	spiceClient, err := spice.NewClient(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(out, "! git-spice not found (required for stacking)\n")
		_, _ = fmt.Fprintf(out, "  Install with: cargo install git-spice\n")
		_, _ = fmt.Fprintf(out, "              or: brew install git-spice\n")
		return true, false
	}

	version, err := spiceClient.GetVersion(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(out, "! git-spice installed but version check failed\n")
		return true, false
	}

	_, _ = fmt.Fprintf(out, "✓ git-spice installed: %s\n", version)
	return true, true
}

func checkConfiguration(out io.Writer) bool {
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		_, _ = fmt.Fprintln(out, "! No configuration file found")
		_, _ = fmt.Fprintln(out, "  Run 'wt config set <key> <value>' to create one")
		return false
	}

	// Show which configs are active
	if projectPath != "" {
		_, _ = fmt.Fprintf(out, "✓ Project config: %s\n", projectPath)
	}
	if globalPath != "" {
		_, _ = fmt.Fprintf(out, "✓ Global config: %s\n", globalPath)
	}

	// Validate merged config
	cfg, err := config.LoadMerged(projectPath, globalPath)
	if err != nil {
		_, _ = fmt.Fprintf(out, "! Config is invalid: %v\n", err)
		return false
	}

	_, _ = fmt.Fprintln(out, "✓ Config is valid YAML")

	// Check worktree location
	location := cfg.Worktree.Location
	if location == "" {
		location = "per-repo"
	}
	if location == "dedicated" {
		dedicatedPath := cfg.Worktree.DedicatedPath
		if dedicatedPath == "" {
			dedicatedPath = "~/worktrees"
		}
		_, _ = fmt.Fprintf(out, "✓ Worktree location: dedicated (%s)\n", dedicatedPath)
	} else {
		_, _ = fmt.Fprintln(out, "✓ Worktree location: per-repo")
	}

	return true
}

func checkRepoDoctor(ctx context.Context, out io.Writer, gitSpiceOK bool) bool {
	gitClient, err := git.NewClient()
	if err != nil {
		_, _ = fmt.Fprintf(out, "! Git not available for repository check\n")
		return false
	}

	// Check if we're in a git repository
	repoInfo, err := gitClient.GetRepoInfo(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(out, "! Not in a git repository: %v\n", err)
		return false
	}

	_, _ = fmt.Fprintf(out, "✓ Git repository detected\n")
	_, _ = fmt.Fprintf(out, "  Root: %s\n", repoInfo.RootPath)
	_, _ = fmt.Fprintf(out, "  Default branch: %s\n", repoInfo.DefaultBranch)

	// Get current branch
	branch, err := gitClient.GetCurrentBranch(ctx)
	if err != nil {
		// Detached HEAD or error
		_, _ = fmt.Fprintf(out, "! Cannot determine current branch: %v\n", err)
		return true // Not critical
	}

	_, _ = fmt.Fprintf(out, "✓ On branch: %s\n", branch)

	// Check if we can stack (not on main/master)
	if branch == repoInfo.DefaultBranch || branch == "main" || branch == "master" {
		_, _ = fmt.Fprintf(out, "! Cannot stack on default branch (%s)\n", branch)
		_, _ = fmt.Fprintf(out, "  Switch to a feature branch to use 'wt stack'\n")
		return gitSpiceOK // Not critical if git-spice is available
	}

	_, _ = fmt.Fprintf(out, "✓ Can create stack (not on default branch)\n")

	// Check git-spice stack if available
	if gitSpiceOK {
		cfg, err := loadConfigForCommand()
		if err != nil {
			_, _ = fmt.Fprintf(out, "! Failed to load config: %v\n", err)
		} else {
			spiceClient, err := spice.NewClient(cfg)
			if err == nil {
				stack, err := spiceClient.GetStack(ctx)
				if err != nil {
					_, _ = fmt.Fprintf(out, "! Cannot get git-spice stack: %v\n", err)
				} else if len(stack) == 0 {
					_, _ = fmt.Fprintf(out, "! No git-spice stack found (not a stacked branch)\n")
				} else {
					_, _ = fmt.Fprintf(out, "✓ Stacked branch detected (%d branches in stack)\n", len(stack))
				}
			}
		}
	}

	return true
}

func init() {
	RegisterCommand(NewDoctorCmd())
}
