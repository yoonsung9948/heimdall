package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/yoonsung9948/heimdall/internal/config"
	"github.com/yoonsung9948/heimdall/internal/matrix"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
	"github.com/yoonsung9948/heimdall/internal/upstream"
)

type DeniedError struct {
	Decision policy.Decision
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("denied: %s", e.Decision.Reason)
}

func newCheckCommand() *cobra.Command {
	var policyPath string
	var configPath string
	var snapshotPath string
	var user string
	var client string
	var groups []string
	var action string
	var resourceKind string
	var resourceName string
	var toolName string
	var explain bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check or explain an authorization decision against a policy file",
		Long: strings.TrimSpace(`
Without --explain: evaluate a hypothetical identity (--user/--client/--group)
against an arbitrary action/resource (--action/--resource-kind/--resource-name).
The identity doesn't need to exist anywhere — useful for testing a policy
file's rules against subjects that aren't (or aren't yet) registered.

With --explain: look up a real, registered client (--client, resolved from
--config) and a real tool (--tool, resolved from --snapshot), then print the
matched rule and which subject/resource fields drove the decision.
`),
		Example: strings.TrimSpace(`
  heimdall check --policy policy.yaml --user alice --action tools/call --resource-kind tool --resource-name search
  heimdall check --explain --client claude-desktop --tool search.web
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := policy.LoadFile(policyPath)
			if err != nil {
				return fmt.Errorf("load policy: %w", err)
			}

			if explain {
				return runExplain(cmd.OutOrStdout(), e, configPath, snapshotPath, client, toolName, jsonOut)
			}
			return runCheck(cmd.OutOrStdout(), e, user, client, groups, action, resourceKind, resourceName, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "write the output as json")
	cmd.Flags().BoolVar(&explain, "explain", false, "look up a registered client/tool by name (--config/--snapshot) and print a detailed explanation, instead of a raw dry-run")
	cmd.Flags().StringVar(&policyPath, "policy", "policy.yaml", "path to the policy config file to evaluate against")
	cmd.Flags().StringVar(&configPath, "config", "heimdall.yaml", "path to config file (only used with --explain)")
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "tools.snapshot.json", "path to the tool snapshot file (only used with --explain)")
	cmd.Flags().StringVar(&user, "user", "", "user identity to check, e.g. alice (only used without --explain)")
	cmd.Flags().StringVar(&client, "client", "", "client identity to check (raw), or the registered client name to look up with --explain")
	cmd.Flags().StringArrayVar(&groups, "group", []string{}, "group the identity belongs to, repeatable: --group a --group b (only used without --explain)")
	cmd.Flags().StringVar(&action, "action", "", "action being requested, e.g. tools/call (only used without --explain)")
	cmd.Flags().StringVar(&resourceKind, "resource-kind", "", "kind of resource being accessed, e.g. tool (only used without --explain)")
	cmd.Flags().StringVar(&resourceName, "resource-name", "", "name of the resource being accessed (only used without --explain)")
	cmd.Flags().StringVar(&toolName, "tool", "", "tool name to look up from the snapshot (only used with --explain)")

	return cmd
}

// runCheck builds a hypothetical Request straight from flag values — no
// config or snapshot involved — and prints a flat ALLOW/DENY decision.
func runCheck(w io.Writer, e *policy.Engine, user, client string, groups []string, action, resourceKind, resourceName string, jsonOut bool) error {
	action = strings.ToLower(strings.TrimSpace(action))
	resourceKind = strings.ToLower(strings.TrimSpace(resourceKind))
	resourceName = strings.TrimSpace(resourceName)
	user = strings.TrimSpace(user)
	client = strings.TrimSpace(client)

	if action == "" || resourceKind == "" || resourceName == "" {
		return fmt.Errorf("--action, --resource-kind, and --resource-name are required unless --explain is set")
	}

	normalizedGroups, err := normalizeGroups(groups)
	if err != nil {
		return err
	}

	req := policy.Request{
		Identity: types.Identity{
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
	decision := e.Authorize(req)
	if jsonOut {
		if err := writeJSON(w, newDecisionDTO(decision)); err != nil {
			return fmt.Errorf("write json output: %w", err)
		}
	} else {
		printDecision(w, decision)
	}
	if !decision.Allow {
		return &DeniedError{
			Decision: decision,
		}
	}
	return nil
}

// runExplain resolves a real client from --config and a real tool from
// --snapshot, evaluates that pair through matrix.Evaluate (the same single
// source of truth used by access-matrix/diff-access), and prints the
// detailed explanation.
func runExplain(w io.Writer, e *policy.Engine, configPath, snapshotPath, clientName, toolName string, jsonOut bool) error {
	if clientName == "" || toolName == "" {
		return fmt.Errorf("--client and --tool are required with --explain")
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	identity, ok := findIdentity(upstream.BuildIdentityList(*cfg), clientName)
	if !ok {
		return fmt.Errorf("client %q not found in config", clientName)
	}

	snapshot, err := upstream.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	tool, serverName, ok := findTool(upstream.ToolsFromSnapshot(snapshot), toolName)
	if !ok {
		return fmt.Errorf("tool %q not found in snapshot", toolName)
	}

	cell := matrix.Evaluate(e, identity, tool, serverName)

	if jsonOut {
		if err := writeJSON(w, newExplanationDTO(cell.Explanation)); err != nil {
			return fmt.Errorf("write json output: %w", err)
		}
	} else {
		printExplanation(w, cell.Explanation)
	}
	if !cell.Explanation.Decision.Allow {
		return &DeniedError{
			Decision: cell.Explanation.Decision,
		}
	}
	return nil
}

func findIdentity(identities []types.Identity, client string) (types.Identity, bool) {
	for _, id := range identities {
		if id.Client == client {
			return id, true
		}
	}
	return types.Identity{}, false
}

func findTool(toolsByServer map[string][]*mcp.Tool, name string) (*mcp.Tool, string, bool) {
	for serverName, tools := range toolsByServer {
		for _, tool := range tools {
			if tool.Name == name {
				return tool, serverName, true
			}
		}
	}
	return nil, "", false
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

// printDecision renders the header block shared by both check and
// check --explain: a status line, then an indented, column-aligned
// rule/reason. --explain builds on top of exactly this, rather than using
// a different shape, so the two stay visually consistent.
func printDecision(w io.Writer, d policy.Decision) {
	fmt.Fprintln(w, allowLabel(d.Allow))

	tw := tabwriter.NewWriter(w, 0, 2, 1, ' ', 0)
	if d.RuleID != "" {
		fmt.Fprintf(tw, "  rule:\t%s\n", d.RuleID)
	}
	fmt.Fprintf(tw, "  reason:\t%s\n", d.Reason)
	tw.Flush()
}

func printExplanation(w io.Writer, exp policy.Explanation) {
	printDecision(w, exp.Decision)
	if exp.Detail == nil || exp.Rule == nil {
		return
	}
	rule, detail := exp.Rule, exp.Detail

	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 2, 1, ' ', 0)

	if len(rule.Subjects.Users) > 0 || len(rule.Subjects.Clients) > 0 || len(rule.Subjects.Groups) > 0 {
		fmt.Fprintln(tw, "  subjects:")
		if len(rule.Subjects.Users) > 0 {
			fmt.Fprintf(tw, "    user:\t%s\n", matchLabel(detail.Subjects.UserMatched))
		}
		if len(rule.Subjects.Clients) > 0 {
			fmt.Fprintf(tw, "    client:\t%s\n", matchLabel(detail.Subjects.ClientMatched))
		}
		if len(rule.Subjects.Groups) > 0 {
			fmt.Fprintf(tw, "    group:\t%s\n", matchLabel(detail.Subjects.GroupMatched))
		}
	}

	fmt.Fprintf(tw, "  action:\t%s\n", matchLabel(detail.Action))

	if len(rule.Resources.Kinds) > 0 || len(rule.Resources.Names) > 0 {
		fmt.Fprintln(tw, "  resource:")
		if len(rule.Resources.Kinds) > 0 {
			fmt.Fprintf(tw, "    kind:\t%s\n", matchLabel(detail.Resource.KindMatched))
		}
		if len(rule.Resources.Names) > 0 {
			fmt.Fprintf(tw, "    name:\t%s\n", matchLabel(detail.Resource.NameMatched))
		}
	}
	tw.Flush()
}

func matchLabel(matched bool) string {
	if matched {
		return "matched"
	}
	return "not matched"
}
