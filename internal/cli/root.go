package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	var configPath string
	rootCmd := &cobra.Command{
		Use:   "heimdall",
		Short: "Identity-aware MCP gateway for developers",
	}

	rootCmd.AddCommand(newServeCommand(&configPath))
	rootCmd.AddCommand(newCheckCommand(&configPath))
	rootCmd.AddCommand(newToolsCommand(&configPath))
	rootCmd.AddCommand(newValidateCommand(&configPath))
	rootCmd.AddCommand(newInitCommand())

	return rootCmd
}

func addConfigFlag(cmd *cobra.Command, configPath *string) {
	cmd.Flags().StringVar(configPath, "config", "heimdall.yaml", "path to config file")
}
