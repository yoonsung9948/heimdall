package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServeCommand(configPath *string) *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Heimdall gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(
				cmd.OutOrStdout(), "serving with config=%s addr=%s\n",
				*configPath,
				addr,
			)
			return nil
		},
	}
	addConfigFlag(cmd, configPath)
	cmd.Flags().StringVar(&addr, "addr", ":9090", "listen address")
	return cmd
}
