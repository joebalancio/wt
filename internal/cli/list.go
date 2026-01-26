package cli

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

// printWorktrees prints worktrees to the given writer
func printWorktrees(w io.Writer, worktrees []*domain.Worktree) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, wt := range worktrees {
		fmt.Fprintf(tw, "%s\t%s\n", wt.Path, wt.Branch)
	}
	return tw.Flush()
}

// NewListCmd creates the list command
func NewListCmd() *cobra.Command {
	var branches []string
	var pathPrefix string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List git worktrees",
		Long:  `List all git worktrees in the current repository.`,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			cfg, err := loadConfigForCommand()
			if err != nil {
				Fatal("Failed to load config: %v", err)
			}

			svc, err := worktree.NewService(gitClient, cfg)
			if err != nil {
				Fatal("Failed to create service: %v", err)
			}

			filter := &domain.WorktreeFilter{
				Branches:   branches,
				PathPrefix: pathPrefix,
			}

			worktrees, err := svc.List(ctx, filter)
			if err != nil {
				Fatal("Failed to list worktrees: %v", err)
			}

			if err := printWorktrees(cmd.OutOrStdout(), worktrees); err != nil {
				Fatal("Failed to print worktrees: %v", err)
			}
		},
	}

	cmd.Flags().StringSliceVar(&branches, "branches", nil, "filter by branch names")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "filter by path prefix")

	return cmd
}

func init() {
	RegisterCommand(NewListCmd())
}
