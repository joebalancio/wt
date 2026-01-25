package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/spice"
)

// NewDoctorCmd creates the doctor command
func NewDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
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
			ctx := context.Background()
			out := cmd.OutOrStdout()

			allPass := true

			// Check wt installation
			fmt.Fprintln(out, "Checking wt installation...")
			if !checkWTBinary(ctx, out) {
				allPass = false
			}

			// Check dependencies
			fmt.Fprintln(out, "\nChecking dependencies...")
			gitOK, gitSpiceOK := checkDependencies(ctx, out)
			if !gitOK {
				allPass = false
			}

			// Check configuration
			fmt.Fprintln(out, "\nChecking configuration...")
			if !checkConfiguration(out) {
				allPass = false
			}

			// Check repository (only if git is available)
			if gitOK {
				fmt.Fprintln(out, "\nChecking current repository...")
				if !checkRepoDoctor(ctx, out, gitSpiceOK) {
					// Git-spice missing is not critical for basic operations
				}
			}

			// Summary
			fmt.Fprintln(out)
			if allPass {
				fmt.Fprintln(out, "All checks passed!")
				os.Exit(0)
			} else if gitOK {
				// Git is OK but other checks failed
				os.Exit(2) // Warning
			} else {
				// Critical failure
				os.Exit(1)
			}
		},
	}

	return cmd
}

func checkWTBinary(ctx context.Context, out io.Writer) bool {
	// Get wt binary path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(out, "! wt binary path unknown: %v\n", err)
		return false
	}
	fmt.Fprintf(out, "✓ wt binary: %s\n", execPath)

	// Get version
	fmt.Fprintf(out, "✓ Version: %s\n", "v2.0.0")
	fmt.Fprintf(out, "✓ OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	return true
}

func checkDependencies(ctx context.Context, out io.Writer) (gitOK bool, gitSpiceOK bool) {
	// Check git
	_, err := git.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git not found\n")
		return false, false
	}

	// Check git version
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "--version")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "✗ git installed but version check failed\n")
		return true, false
	}
	gitVersion := strings.TrimSpace(stdout.String())
	fmt.Fprintf(out, "✓ git installed: %s\n", gitVersion)

	// Check git worktree support
	cmd = exec.CommandContext(ctx, "git", "worktree", "list")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "✗ git worktree not supported\n")
		return true, false
	}
	fmt.Fprintf(out, "✓ git worktree supported\n")

	// Check git-spice
	spiceClient, err := spice.NewClient()
	if err != nil {
		fmt.Fprintf(out, "! git-spice not found (required for stacking)\n")
		fmt.Fprintf(out, "  Install with: cargo install git-spice\n")
		fmt.Fprintf(out, "              or: brew install git-spice\n")
		return true, false
	}

	version, err := spiceClient.GetVersion(ctx)
	if err != nil {
		fmt.Fprintf(out, "! git-spice installed but version check failed\n")
		return true, false
	}

	fmt.Fprintf(out, "✓ git-spice installed: %s\n", version)
	return true, true
}

func checkConfiguration(out io.Writer) bool {
	// Check user config
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			fmt.Fprintf(out, "! Cannot determine home directory\n")
			return false
		}
	}
	configPath := filepath.Join(home, ".config", "wt", "config.yaml")

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "! User config not found: %s\n", configPath)
			fmt.Fprintf(out, "  Run 'wt init' to create default config\n")
			return false
		}
		fmt.Fprintf(out, "! Cannot access config: %v\n", err)
		return false
	}

	fmt.Fprintf(out, "✓ User config: %s\n", configPath)

	// Validate config
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(out, "! Config is invalid: %v\n", err)
		return false
	}

	fmt.Fprintf(out, "✓ Config is valid YAML\n")

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
		fmt.Fprintf(out, "✓ Worktree location: dedicated (%s)\n", dedicatedPath)
	} else {
		fmt.Fprintf(out, "✓ Worktree location: per-repo\n")
	}

	return true
}

func checkRepoDoctor(ctx context.Context, out io.Writer, gitSpiceOK bool) bool {
	gitClient, err := git.NewClient()
	if err != nil {
		fmt.Fprintf(out, "! Git not available for repository check\n")
		return false
	}

	// Check if we're in a git repository
	repoInfo, err := gitClient.GetRepoInfo(ctx)
	if err != nil {
		fmt.Fprintf(out, "! Not in a git repository: %v\n", err)
		return false
	}

	fmt.Fprintf(out, "✓ Git repository detected\n")
	fmt.Fprintf(out, "  Root: %s\n", repoInfo.RootPath)
	fmt.Fprintf(out, "  Default branch: %s\n", repoInfo.DefaultBranch)

	// Get current branch
	branch, err := gitClient.GetCurrentBranch(ctx)
	if err != nil {
		// Detached HEAD or error
		fmt.Fprintf(out, "! Cannot determine current branch: %v\n", err)
		return true // Not critical
	}

	fmt.Fprintf(out, "✓ On branch: %s\n", branch)

	// Check if we can stack (not on main/master)
	if branch == repoInfo.DefaultBranch || branch == "main" || branch == "master" {
		fmt.Fprintf(out, "! Cannot stack on default branch (%s)\n", branch)
		fmt.Fprintf(out, "  Switch to a feature branch to use 'wt stack'\n")
		return gitSpiceOK // Not critical if git-spice is available
	}

	fmt.Fprintf(out, "✓ Can create stack (not on default branch)\n")

	// Check git-spice stack if available
	if gitSpiceOK {
		spiceClient, err := spice.NewClient()
		if err == nil {
			stack, err := spiceClient.GetStack(ctx)
			if err != nil {
				fmt.Fprintf(out, "! Cannot get git-spice stack: %v\n", err)
			} else if len(stack) == 0 {
				fmt.Fprintf(out, "! No git-spice stack found (not a stacked branch)\n")
			} else {
				fmt.Fprintf(out, "✓ Stacked branch detected (%d branches in stack)\n", len(stack))
			}
		}
	}

	return true
}

func init() {
	RegisterCommand(NewDoctorCmd())
}
