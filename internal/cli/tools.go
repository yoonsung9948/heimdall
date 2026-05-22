package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newToolsCommand(configPath *string) *cobra.Command {
	var identity string
	var client string
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List effective tools for a client or identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			if identity == "" && client == "" {
				return fmt.Errorf("either --identity or --client is required")
			}

			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "config:   %s\n", *configPath)

			if identity != "" {
				fmt.Fprintf(out, "identity: %s\n", identity)
			}

			if client != "" {
				fmt.Fprintf(out, "client:   %s\n", client)
			}

			return nil
		},
	}
	addConfigFlag(cmd, configPath)
	cmd.Flags().StringVar(&identity, "identity", "", "identity to check")
	cmd.Flags().StringVar(&client, "client", "", "client name to check")
	return cmd
}
