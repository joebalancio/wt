package domain

import (
	"fmt"
	"strings"
)

// Worktree represents a git worktree
type Worktree struct {
	Path     string // Absolute path to worktree directory
	Branch   string // Branch name (refs/heads/ prefix removed)
	Head     string // Commit SHA or "detached"
	Bare     bool   // Is this a bare worktree
	Modified bool   // Has uncommitted changes
}

// String returns a formatted representation of the worktree
func (w *Worktree) String() string {
	if w == nil {
		return "<nil>"
	}
	if w.Bare {
		return fmt.Sprintf("%s (bare)", w.Path)
	}
	if w.Detached() {
		return fmt.Sprintf("%s [detached %s]", w.Path, w.Head)
	}
	return fmt.Sprintf("%s [%s]", w.Path, w.Branch)
}

// Detached returns true if HEAD is detached
func (w *Worktree) Detached() bool {
	return w.Head != "" && w.Branch == ""
}

// WorktreeCreateSpec defines parameters for creating a worktree
type WorktreeCreateSpec struct {
	Branch   string  // Branch name (required)
	Base     string  // Base branch for new branches (optional)
	Path     string  // Custom path (optional, auto-generated if empty)
	Force    bool    // Force creation even if path exists
	Checkout bool    // Whether to checkout the branch (default: true)
	Track    *string // Remote branch to track (optional, pointer for nil vs empty)
}

// Validate checks if the spec is valid
func (s *WorktreeCreateSpec) Validate() error {
	if s.Branch == "" {
		return fmt.Errorf("branch name is required")
	}
	return nil
}

// WorktreeFilter defines filters for listing worktrees
type WorktreeFilter struct {
	Branches    []string // Filter by branch names
	PathPrefix  string   // Filter by path prefix
	IncludeBare bool     // Include bare worktrees
}

// Matches checks if a worktree matches the filter
func (f *WorktreeFilter) Matches(w *Worktree) bool {
	if !f.IncludeBare && w.Bare {
		return false
	}
	if f.PathPrefix != "" && !strings.HasPrefix(w.Path, f.PathPrefix) {
		return false
	}
	if len(f.Branches) > 0 {
		for _, b := range f.Branches {
			if w.Branch == b {
				return true
			}
		}
		return false
	}
	return true
}

// GitRepo represents the git repository context
type GitRepo struct {
	RootPath      string // Absolute path to main worktree
	DefaultBranch string // Default branch (main/master)
	IsBare        bool   // Is this a bare repository
}

// IsValid checks if the repo info is valid
func (r *GitRepo) IsValid() bool {
	return r.RootPath != ""
}

// Branch represents a git branch
type Branch struct {
	Name   string  // Short branch name
	SHA    string  // Commit SHA
	Remote *string // Remote name (nil for local-only)
}

// StackBranch represents a branch in a git-spice stack
type StackBranch struct {
	Name     string         // Branch name
	IsRoot   bool           // Is this the root of the stack
	IsHead   bool           // Is this the current branch
	Path     string         // Worktree path if checked out
	Children []*StackBranch // Child branches
}
