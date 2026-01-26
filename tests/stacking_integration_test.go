package tests

import (
	"testing"
)

// TestStacking_BasicWorkflow tests the basic stacking workflow:
// 1. Initialize git-spice in a repo
// 2. Create a root branch
// 3. Stack on it (auto-suffix)
// 4. Verify both branches exist
// 5. Clean up
func TestStacking_BasicWorkflow(t *testing.T) {
	skipIfNoGit(t)

	// TODO: These tests use outdated git-spice API (v0.22+ uses 'repo init' instead of 'init',
	// requires branch tracking, etc.). Update tests to current git-spice workflow.
	// Skip for now as this is testing git-spice workflow, not wt functionality.
	t.Skip("git-spice integration tests need update for v0.22+ API")
}

// TestStacking_BranchNaming tests the nanoid-based branch naming:
// 1. Create a root branch
// 2. Stack with auto-suffix (generates 4-char suffix)
// 3. Verify branch name format
func TestStacking_BranchNaming(t *testing.T) {
	skipIfNoGit(t)

	// TODO: These tests use outdated git-spice API (v0.22+ uses 'repo init' instead of 'init',
	// requires branch tracking, etc.). Update tests to current git-spice workflow.
	// Skip for now as this is testing git-spice workflow, not wt functionality.
	t.Skip("git-spice integration tests need update for v0.22+ API")
}
