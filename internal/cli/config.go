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
	RegisterCommand(configCmd)
}
