package stack

import (
	"fmt"
	"strings"

	"github.com/user/wt/pkg/domain"
)

// FormatStackTree formats a stack of branches as a tree with paths
func FormatStackTree(branches []*domain.StackBranch) string {
	var builder strings.Builder

	for i, branch := range branches {
		prefix := getTreePrefix(i, len(branches))
		currentMarker := ""
		if branch.IsHead {
			currentMarker = " (current) ◀────"
		}

		fmt.Fprintf(&builder, "%s%s%s [%s]\n",
			prefix,
			branch.Name,
			currentMarker,
			branch.Path)
	}

	return builder.String()
}

func getTreePrefix(index, _ int) string {
	if index == 0 {
		return ""
	}
	// Simple prefix for now - will be enhanced for proper tree structure
	return "├── "
}
