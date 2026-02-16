package cli

import (
	"fmt"

	configpkg "github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigListCmd creates the config list command
func NewConfigListCmd() *cobra.Command {
	var local, global bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		Long: `List all config values.

By default, shows the merged config (project-local > global > defaults).
Use --local to show only project-local config.
Use --global to show only global config.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			// Determine scope
			scope := ScopeMerged // Default: merged read
			if local {
				scope = ScopeLocal
			} else if global {
				scope = ScopeGlobal
			}

			cfg := loadConfigForScope(scope)

			// Explicitly reference config package to ensure import is used
			_ = configpkg.DefaultConfig

			data, err := yaml.Marshal(cfg)
			if err != nil {
				Fatal("marshaling config: %v", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(data)); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&local, "local", "l", false, "show project-local config only")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "show global config only")
	cmd.MarkFlagsMutuallyExclusive("local", "global")

	return cmd
}
