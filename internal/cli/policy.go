package cli

import "github.com/spf13/cobra"

// TODO: implement policy config override
// policy file should be loaded on the "serve" command
// and get the policy config file from a struct that stores the config
// if dry run, override the config file
func newPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Inspect and dry-run authorization policy",
	}

	cmd.AddCommand(newPolicyCheckCommand())

	return cmd
}
