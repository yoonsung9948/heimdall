package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yoonsung9948/heimdall/internal/config"
)

func newValidateCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validates a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.LoadFile(*configPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s is a valid config file\n", filepath.Base(*configPath))
			return nil
		},
	}
	addConfigFlag(cmd, configPath)
	return cmd
}
