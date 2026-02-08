package cli

import (
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage wt configuration",
	Long:  `Initialize, validate, and view wt configuration files.`,
}

func init() {
	// Register subcommands
	configCmd.AddCommand(
		NewConfigGetCmd(),
		NewConfigListCmd(),
		NewConfigSetCmd(),
		NewConfigUnsetCmd(),
		NewConfigValidateCmd(),
	)

	RegisterCommand(configCmd)
}
