package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCheckCommand(configPath *string) *cobra.Command {
	var identity string
	var tool string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check whether an identity can call a tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			if identity == "" {
				return fmt.Errorf("--identity is required")
			}
			if tool == "" {
				return fmt.Errorf("--tool is required")
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"checking config=%s itdentity=%s tool=%s\n",
				*configPath,
				identity,
				tool,
			)
			return nil
		},
	}
	addConfigFlag(cmd, configPath)
	cmd.Flags().StringVar(&identity, "identity", "", "identity to check")
	cmd.Flags().StringVar(&tool, "tool", "", "tool name to check")
	return cmd
}
