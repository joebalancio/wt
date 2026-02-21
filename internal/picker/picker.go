// Package picker provides interactive TUI selection for wt commands.
package picker

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/joebalancio/wt/internal/git"
	"golang.org/x/term"
)

const newBranchOption = "Create new branch"

// Picker provides interactive selection functionality.
type Picker struct {
	gitClient git.BranchLister
}

// NewPicker creates a new Picker instance.
func NewPicker(gitClient git.BranchLister) *Picker {
	return &Picker{gitClient: gitClient}
}

// IsTerminal returns true if stdout is connected to a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// SelectWorktree presents a picker for selecting a worktree to remove.
// Returns the selected worktree path, or an error if selection fails.
func (p *Picker) SelectWorktree(ctx context.Context) (string, error) {
	worktrees, err := p.gitClient.ListWorktrees(ctx)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		return "", fmt.Errorf("no worktrees found")
	}

	var options []huh.Option[string]
	for _, wt := range worktrees {
		if wt.Branch == "" {
			continue
		}
		label := fmt.Sprintf("%s -> %s", wt.Branch, wt.Path)
		options = append(options, huh.NewOption(label, wt.Path))
	}

	if len(options) == 0 {
		return "", fmt.Errorf("no removable worktrees found")
	}

	var selected string
	err = huh.NewSelect[string]().
		Title("Select worktree to remove:").
		Options(options...).
		Value(&selected).
		Run()

	return selected, err
}

// SelectBranchResult contains the result of branch selection.
type SelectBranchResult struct {
	Branch     string
	BaseBranch string
	IsNew      bool
}

// SelectBranch presents a picker for selecting or creating a branch.
func (p *Picker) SelectBranch(ctx context.Context) (SelectBranchResult, error) {
	branches, err := p.gitClient.ListAllBranches(ctx)
	if err != nil {
		return SelectBranchResult{}, fmt.Errorf("list branches: %w", err)
	}

	options := []huh.Option[string]{huh.NewOption(newBranchOption, newBranchOption)}
	for _, branch := range branches {
		options = append(options, huh.NewOption(branch, branch))
	}

	var selected string
	err = huh.NewSelect[string]().
		Title("Select or create a branch:").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return SelectBranchResult{}, err
	}

	if selected == newBranchOption {
		return p.promptNewBranch(branches)
	}

	return SelectBranchResult{
		Branch: selected,
		IsNew:  false,
	}, nil
}

func (p *Picker) promptNewBranch(existingBranches []string) (SelectBranchResult, error) {
	var branchName string
	err := huh.NewInput().
		Title("Enter new branch name:").
		Value(&branchName).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("branch name cannot be empty")
			}
			for _, branch := range existingBranches {
				if branch == s {
					return fmt.Errorf("branch %q already exists", s)
				}
			}
			return nil
		}).
		Run()
	if err != nil {
		return SelectBranchResult{}, err
	}

	baseOptions := make([]huh.Option[string], len(existingBranches))
	for i, branch := range existingBranches {
		baseOptions[i] = huh.NewOption(branch, branch)
	}

	var baseBranch string
	err = huh.NewSelect[string]().
		Title("Select base branch:").
		Options(baseOptions...).
		Value(&baseBranch).
		Run()
	if err != nil {
		return SelectBranchResult{}, err
	}

	return SelectBranchResult{
		Branch:     branchName,
		BaseBranch: baseBranch,
		IsNew:      true,
	}, nil
}
