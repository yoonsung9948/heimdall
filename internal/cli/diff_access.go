package cli

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yoonsung9948/heimdall/internal/config"
	"github.com/yoonsung9948/heimdall/internal/matrix"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/upstream"
)

// newDiffAccessCommand compares the access matrix produced by two policy
// files against the *same* identities/tools, and reports what changed.
// This is what `heimdall verify` will call in CI to catch unintended
// privilege expansion.
func newDiffAccessCommand() *cobra.Command {
	var configPath string
	var snapshotPath string
	var beforePolicyPath string
	var afterPolicyPath string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "diff-access",
		Short: "Show which (identity, tool) decisions change between two policy files",
		Long: `Builds the access matrix twice — once against --before, once against
--after — using the same identities and tool snapshot both times, then
reports permissions gained, lost, and unchanged. Privilege expansion
(DENY -> ALLOW) is highlighted separately from narrowing, since that's
the direction that actually matters for a CI gate.`,
		Example: `  heimdall diff-access --before policy.yaml --after policy.next.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFile(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			identities := upstream.BuildIdentityList(*cfg)

			snapshot, err := upstream.ReadFile(snapshotPath)
			if err != nil {
				return fmt.Errorf("read snapshot: %w", err)
			}
			toolsByServer := upstream.ToolsFromSnapshot(snapshot)

			beforeEngine, err := policy.LoadFile(beforePolicyPath)
			if err != nil {
				return fmt.Errorf("load --before policy: %w", err)
			}
			afterEngine, err := policy.LoadFile(afterPolicyPath)
			if err != nil {
				return fmt.Errorf("load --after policy: %w", err)
			}

			before := matrix.Build(beforeEngine, identities, toolsByServer)
			after := matrix.Build(afterEngine, identities, toolsByServer)

			diffs, err := matrix.Diff(before, after)
			if err != nil {
				return err
			}

			var changes []matrix.Change

			for _, diff := range diffs {
				if diff.Kind != matrix.Unchanged {
					changes = append(changes, diff)
				}
			}

			slices.SortFunc(changes, func(a, b matrix.Change) int {
				if c := cmp.Compare(a.Identity.Client, b.Identity.Client); c != 0 {
					return c
				}
				return cmp.Compare(a.Tool.Name, b.Tool.Name)
			})
			if jsonOut {
				changesDTOs := newChangeDTOs(changes)
				if err := writeJSON(cmd.OutOrStdout(), changesDTOs); err != nil {
					return fmt.Errorf("write json output: %w", err)
				}
			} else {
				printChanges(cmd.OutOrStdout(), changes)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "write the output as json")
	cmd.Flags().StringVar(&configPath, "config", "heimdall.yaml", "path to config file")
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "tools.snapshot.json", "path to the tool snapshot file")
	cmd.Flags().StringVar(&beforePolicyPath, "before", "", "path to the baseline policy file")
	cmd.Flags().StringVar(&afterPolicyPath, "after", "", "path to the candidate policy file")
	mustMarkRequired(cmd, "before")
	mustMarkRequired(cmd, "after")

	return cmd
}

// printChanges renders one row per change, in the same tabwriter-column
// style as access-matrix's printMatrix, so the two commands read as one
// consistent tool rather than two differently-formatted outputs. Filtering
// (e.g. dropping Unchanged, or showing only Gained) is the caller's job —
// this just renders whatever slice it's handed.
func printChanges(w io.Writer, changes []matrix.Change) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "IDENTITY\tSERVER\tTOOL\tCHANGE\tBEFORE\tAFTER")
	for _, c := range changes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			c.Identity.Client,
			c.ServerName,
			c.Tool.Name,
			changeKindLabel(c.Kind),
			allowLabel(c.Before.Decision.Allow),
			allowLabel(c.After.Decision.Allow),
		)
	}
	tw.Flush()
}

func changeKindLabel(k matrix.ChangeKind) string {
	switch k {
	case matrix.Gained:
		return "GAINED"
	case matrix.Lost:
		return "LOST"
	default:
		return "unchanged"
	}
}

func allowLabel(allow bool) string {
	if allow {
		return "ALLOW"
	}
	return "DENY"
}
