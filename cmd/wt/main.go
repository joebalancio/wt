// Package main is the entry point for the wt CLI tool.
// Wt is a high-level CLI for managing git worktrees with tmux integration.
package main

import (
	"os"

	"github.com/joebalancio/wt/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
