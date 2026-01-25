package stack

import (
	"strings"
	"testing"

	"github.com/user/wt/pkg/domain"
)

func TestFormatStackTree(t *testing.T) {
	branches := []*domain.StackBranch{
		{
			Name:   "feat/auth",
			IsRoot: true,
			Path:   "~/worktrees/feat/auth",
		},
		{
			Name:   "feat/auth-xY7k",
			IsRoot: false,
			IsHead: true,
			Path:   "~/worktrees/feat/auth-xY7k",
		},
		{
			Name:   "feat/auth-k9P2",
			IsRoot: false,
			Path:   "~/worktrees/feat/auth-k9P2",
		},
	}

	output := FormatStackTree(branches)

	if !strings.Contains(output, "feat/auth") {
		t.Error("output should contain root branch")
	}
	if !strings.Contains(output, "(current)") {
		t.Error("output should mark current branch")
	}
	if !strings.Contains(output, "~/worktrees") {
		t.Error("output should contain paths")
	}
}
