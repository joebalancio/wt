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

var runFuzzySelect = func(ctx context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
	return NewFuzzySelect(title, items, pinned).Run(ctx)
}

var runBranchNameInput = func(existingBranches []string) (string, error) {
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
		return "", err
	}

	return branchName, nil
}

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

	items := make([]FuzzyItem, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.Branch == "" {
			continue
		}
		items = append(items, FuzzyItem{
			Label: fmt.Sprintf("%s -> %s", wt.Branch, wt.Path),
			Value: wt.Path,
		})
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no removable worktrees found")
	}

	selected, err := runFuzzySelect(ctx, "Select worktree to remove:", items, nil)
	if err != nil {
		return "", err
	}

	return selected.Value, nil
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

	items := make([]FuzzyItem, 0, len(branches))
	for _, branch := range branches {
		items = append(items, FuzzyItem{Label: branch, Value: branch})
	}

	selected, err := runFuzzySelect(ctx, "Select or create a branch:", items, []FuzzyItem{
		{Label: newBranchOption, Value: newBranchOption},
	})
	if err != nil {
		return SelectBranchResult{}, err
	}

	if selected.Value == newBranchOption {
		return p.promptNewBranch(ctx, branches)
	}

	return SelectBranchResult{
		Branch: selected.Value,
		IsNew:  false,
	}, nil
}

func (p *Picker) promptNewBranch(ctx context.Context, existingBranches []string) (SelectBranchResult, error) {
	branchName, err := runBranchNameInput(existingBranches)
	if err != nil {
		return SelectBranchResult{}, err
	}

	items := make([]FuzzyItem, len(existingBranches))
	for i, branch := range existingBranches {
		items[i] = FuzzyItem{Label: branch, Value: branch}
	}

	baseBranch, err := runFuzzySelect(ctx, "Select base branch:", items, nil)
	if err != nil {
		return SelectBranchResult{}, err
	}

	return SelectBranchResult{
		Branch:     branchName,
		BaseBranch: baseBranch.Value,
		IsNew:      true,
	}, nil
}
