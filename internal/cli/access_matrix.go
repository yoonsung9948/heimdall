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

func newAccessMatrixCommand() *cobra.Command {
	var configPath string
	var snapshotPath string
	var policyPath string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "access-matrix",
		Short: "Print every (identity, tool) decision as a table",
		Long: `Reads the committed tool snapshot (--snapshot) instead of connecting to
live upstreams, so this works offline in CI. Output is a human-readable
table with a deterministic row order; a --json flag for machine-readable
output is a separate, later piece of work.`,
		Example: `  heimdall access-matrix --snapshot tools.snapshot.json --policy policy.yaml`,
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

			engine, err := policy.LoadFile(policyPath)
			if err != nil {
				return fmt.Errorf("load policy: %w", err)
			}

			cells := matrix.Build(engine, identities, toolsByServer)

			// Single sort at the CLI boundary — matrix.Build intentionally
			// does no ordering of its own. Key is (Identity.Client, Tool.Name);
			// ServerName is excluded because Tool.Name already encodes it
			// (see the prefixing in Registry.Register).
			slices.SortFunc(cells, func(a, b matrix.Cell) int {
				if c := cmp.Compare(a.Identity.Client, b.Identity.Client); c != 0 {
					return c
				}
				return cmp.Compare(a.Tool.Name, b.Tool.Name)
			})
			if jsonOut {
				cellDTOs := newCellDTOs(cells)
				if err := writeJSON(cmd.OutOrStdout(), cellDTOs); err != nil {
					return fmt.Errorf("write json output: %w", err)
				}
			} else {
				printMatrix(cmd.OutOrStdout(), cells)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "write the output as json")
	cmd.Flags().StringVar(&configPath, "config", "heimdall.yaml", "path to config file")
	cmd.Flags().StringVar(&snapshotPath, "snapshot", "tools.snapshot.json", "path to the tool snapshot file")
	cmd.Flags().StringVar(&policyPath, "policy", "policy.yaml", "path to the policy config file")

	return cmd
}

// printMatrix is a placeholder: it renders columns with tabwriter, which is
// fine as a starting point but doesn't do anything smarter (grouping by
// identity, coloring ALLOW/DENY, etc). Revisit once you know what's
// actually readable at the row counts you expect.
func printMatrix(w io.Writer, cells []matrix.Cell) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "IDENTITY\tSERVER\tTOOL\tDECISION")
	for _, c := range cells {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Identity.Client, c.ServerName, c.Tool.Name, allowLabel(c.Explanation.Decision.Allow))
	}
	tw.Flush()
}
