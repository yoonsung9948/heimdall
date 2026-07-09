package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yoonsung9948/heimdall/internal/app"
)

func newServeCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Heimdall gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(
				cmd.OutOrStdout(), "serving with config=%s\n",
				*configPath,
			)
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := app.Run(ctx, *configPath); err != nil {
				return fmt.Errorf("start gateway: %w", err)
			}
			return nil
		},
	}
	addConfigFlag(cmd, configPath)
	return cmd
}
