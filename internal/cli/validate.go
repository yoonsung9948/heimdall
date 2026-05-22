package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValidateCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validates a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(
				cmd.OutOrStdout(), "validating config=%s\n",
				*configPath,
			)
			return nil
		},
	}
	addConfigFlag(cmd, configPath)
	return cmd
}
