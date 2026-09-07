package cli

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/yoonsung9948/heimdall/internal/config"
	"github.com/yoonsung9948/heimdall/internal/matrix"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/upstream"
)

// PrivilegeExpansionError is returned by `heimdall verify` when --after
// grants at least one (identity, tool) decision that --before denied.
// main.go inspects the error returned from cobra's Execute with errors.As
// so it can exit with a distinct, CI-meaningful status code instead of
// forcing callers to parse printed output (see: terraform's
// -detailed-exitcode, which is the same idea).
type PrivilegeExpansionError struct {
	Changes []matrix.Change
}

func (e *PrivilegeExpansionError) Error() string {
	return fmt.Sprintf("verify: %d privilege expansion(s) detected", len(e.Changes))
}

// newVerifyCommand is diff-access's pipeline with a CI-oriented ending:
// instead of printing every change, it fails when --after grants
// something --before denied. v1 semantics: any Gained change is
// prohibited, full stop — the git history that produced --before/--after
// is the review step, so anything newly allowed hasn't been reviewed yet.
// (A richer "sensitive resource pattern" allowlist/denylist, independent
// of the diff, is future work — see the checklist.)
func newVerifyCommand() *cobra.Command {
	var configPath string
	var snapshotPath string
	var beforePolicyPath string
	var afterPolicyPath string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Fail if --after grants access that --before denied",
		Long: `Builds the access matrix twice — once against --before, once against
--after — using the same identities and tool snapshot both times, and
fails if any (identity, tool) decision flips from DENY to ALLOW.

Intended as a CI gate: run with --before pointed at the policy on main
and --after pointed at the PR's candidate policy, so any unreviewed
privilege expansion fails the build instead of merging silently.`,
		Example: `  heimdall verify --before policy.yaml --after policy.next.yaml`,
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

			var expansions []matrix.Change
			for _, diff := range diffs {
				if diff.Kind == matrix.Gained {
					expansions = append(expansions, diff)
				}
			}

			if len(expansions) == 0 {
				if jsonOut {
					if err := writeJSON(cmd.OutOrStdout(), []ChangeDTO{}); err != nil {
						return fmt.Errorf("write json output: %w", err)
					}
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "verify: no privilege expansion detected")
				return nil
			}

			slices.SortFunc(expansions, func(a, b matrix.Change) int {
				if c := cmp.Compare(a.Identity.Client, b.Identity.Client); c != 0 {
					return c
				}
				return cmp.Compare(a.Tool.Name, b.Tool.Name)
			})
			if jsonOut {
				if err := writeJSON(cmd.OutOrStdout(), newChangeDTOs(expansions)); err != nil {
					return fmt.Errorf("write json output: %w", err)
				}
			} else {
				printChanges(cmd.OutOrStdout(), expansions)
			}

			return &PrivilegeExpansionError{Changes: expansions}
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
