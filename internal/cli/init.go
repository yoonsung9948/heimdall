package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var configOut string
	var signKeyDir string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a starter config and signing key directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"generating starter config path=%s key directory=%s\n",
				configOut,
				signKeyDir,
			)
			// if force {
			// 	// overwrite existing config
			// }
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrites config file")
	cmd.Flags().StringVar(&configOut, "out", "heimdall.yaml", "Config output path")
	cmd.Flags().StringVar(&signKeyDir, "dir", ".heimdall/keys/", "Signing key directory")
	return cmd
}
