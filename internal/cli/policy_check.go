package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yoonsung9948/heimdall/internal/policy"
)

func newPolicyCheckCommand() *cobra.Command {
	var policyPath string
	var user string
	var client string
	var groups []string
	var action string
	var resourceKind string
	var resourceName string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Inspect and dry-run authorization policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			action = strings.ToLower(strings.TrimSpace(action))
			resourceKind = strings.ToLower(strings.TrimSpace(resourceKind))
			resourceName = strings.TrimSpace(resourceName)
			user = strings.TrimSpace(user)
			client = strings.TrimSpace(client)

			normalizedGroups, err := normalizeGroups(groups)
			e, err := policy.LoadFile(policyPath)
			if err != nil {
				return err
			}
			r := policy.Request{
				Identity: policy.Identity{
					User:   user,
					Client: client,
					Groups: normalizedGroups,
				},
				Action: policy.Action(action),
				Resource: policy.Resource{
					Kind: policy.ResourceKind(resourceKind),
					Name: resourceName,
				},
			}
			decision := e.Authorize(r)
			var status string
			if decision.Allow {
				status = "ALLOW"
			} else {
				status = "DENY"
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s rule=%q reason=%q\n", status, decision.RuleID, decision.Reason)
			return nil
		},
	}

	cmd.Flags().StringVar(&policyPath, "policy", "policy.yaml", "policy config file path")
	cmd.Flags().StringVar(&user, "user", "", "user")
	cmd.Flags().StringVar(&client, "client", "", "client")
	cmd.Flags().StringArrayVar(&groups, "group", []string{}, "groups")
	cmd.Flags().StringVar(&action, "action", "", "action")
	cmd.Flags().StringVar(&resourceKind, "resource-kind", "", "kind of resource")
	cmd.Flags().StringVar(&resourceName, "resource-name", "", "name of resource")

	mustMarkRequired(cmd, "action")
	mustMarkRequired(cmd, "resource-kind")
	mustMarkRequired(cmd, "resource-name")

	return cmd
}

func normalizeGroups(groups []string) ([]string, error) {
	out := make([]string, 0, len(groups))

	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			return nil, fmt.Errorf("--group contains empty value")
		}
		out = append(out, group)
	}

	return out, nil
}

func mustMarkRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}
