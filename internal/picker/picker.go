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
