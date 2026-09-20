package matrix_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/matrix"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
)

func testCell(
	identity types.Identity,
	tool *mcp.Tool,
	serverName string,
	explanation policy.Explanation,
) matrix.Cell {
	return matrix.Cell{
		Identity:    identity,
		Tool:        tool,
		ServerName:  serverName,
		Explanation: explanation,
	}
}

func TestDiff_EmptyInputs(t *testing.T) {
	tests := []struct {
		name       string
		c1         []matrix.Cell
		c2         []matrix.Cell
		wantChange []matrix.Change
		wantErr    bool
	}{
		{name: "both nil"},
		{name: "nil and empty", c2: []matrix.Cell{}},
		{name: "empty and nil", c1: []matrix.Cell{}},
		{
			name:       "both empty",
			c1:         []matrix.Cell{},
			c2:         []matrix.Cell{},
			wantChange: []matrix.Change{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matrix.Diff(tt.c1, tt.c2)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Diff() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Nil and empty slices both represent no changes
			if len(got) != 0 {
				t.Errorf("Diff() = %#v, want no changes", got)
			}
		})
	}
}
func TestDiff_Changes(t *testing.T) {
	identities := []types.Identity{
		{User: "alice", Client: "alice-laptop", Groups: []string{"engineering"}},
		{User: "bob", Client: "bob-laptop", Groups: []string{"support"}},
		{User: "carol", Client: "carol-laptop", Groups: []string{"engineering", "senior"}},
		{User: "dave", Client: "dave-laptop", Groups: []string{"sre"}},
		{User: "ci", Client: "ci-bot", Groups: []string{"ci"}},
	}
	tools := []*mcp.Tool{
		{Name: "github.list_repos"},
		{Name: "github.delete_repo"},
		{Name: "jira.create_ticket"},
		{Name: "slack.post_message"},
		{Name: "postgres.query"},
	}

	serverNames := []string{
		"github",
		"github",
		"jira",
		"slack",
		"postgres",
	}
	explanations := []policy.Explanation{
		{
			Decision: policy.Decision{
				Allow:  true,
				RuleID: "allow-engineering",
				Reason: "allowed by matching rule",
			},
		},
		{
			Decision: policy.Decision{
				Allow:  false,
				RuleID: "deny-delete",
				Reason: "denied by matching rule",
			},
		},
		{
			Decision: policy.Decision{
				Allow:  false,
				Reason: "denied by default: no matching allow rule",
			},
		},
		{
			Decision: policy.Decision{
				Allow:  true,
				RuleID: "allow-support",
				Reason: "allowed by matching rule",
			},
		},
		{
			Decision: policy.Decision{
				Allow:  true,
				RuleID: "allow-ci",
				Reason: "allowed by matching rule",
			},
		},
	}
	var cells []matrix.Cell

	for i := range len(identities) {
		cells = append(cells, testCell(identities[i], tools[i], serverNames[i], explanations[i]))
	}
	wantUnchanged := make([]matrix.Change, len(cells))
	for i, cell := range cells {
		wantUnchanged[i] = matrix.Change{
			Identity: cell.Identity, Tool: cell.Tool, ServerName: cell.ServerName,
			Kind: matrix.Unchanged, Before: cell.Explanation, After: cell.Explanation,
		}
	}

	tests := []struct {
		name       string
		c1         []matrix.Cell
		c2         []matrix.Cell
		wantChange []matrix.Change
		wantErr    bool
	}{
		{
			name: "single cell, unchanged",
			c1:   []matrix.Cell{testCell(identities[0], tools[0], serverNames[0], explanations[0])},
			c2:   []matrix.Cell{testCell(identities[0], tools[0], serverNames[0], explanations[0])},
			wantChange: []matrix.Change{
				{
					Identity:   identities[0],
					Tool:       tools[0],
					ServerName: serverNames[0],
					Kind:       matrix.Unchanged,
					Before:     explanations[0],
					After:      explanations[0],
				},
			},
			wantErr: false,
		},
		{
			name:       "multiple cells, unchanged",
			c1:         cells,
			c2:         append([]matrix.Cell(nil), cells...),
			wantChange: wantUnchanged,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matrix.Diff(tt.c1, tt.c2)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Diff() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.wantChange, got) {
				t.Errorf("Diff() = %#v, want %#v", got, tt.wantChange)
			}
		})
	}
}

func TestDiff_DecisionTransitions(t *testing.T) {
	identity := types.Identity{User: "alice", Client: "alice-laptop", Groups: []string{"engineering"}}
	tests := []struct {
		name        string
		beforeAllow bool
		afterAllow  bool
		wantKind    matrix.ChangeKind
	}{
		{"allowed to allowed", true, true, matrix.Unchanged},
		{"denied to denied", false, false, matrix.Unchanged},
		{"denied to allowed", false, true, matrix.Gained},
		{"allowed to denied", true, false, matrix.Lost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeExplanation := policy.Explanation{
				Decision: policy.Decision{Allow: tt.beforeAllow, RuleID: "before-rule", Reason: "before reason"},
				Detail:   &policy.MatchResult{Matched: true, RuleID: "before-rule"},
				Rule:     &policy.Rule{ID: "before-rule", Description: "before reason"},
			}
			afterExplanation := policy.Explanation{
				Decision: policy.Decision{Allow: tt.afterAllow, RuleID: "after-rule", Reason: "after reason"},
				Detail:   &policy.MatchResult{Matched: true, RuleID: "after-rule"},
				Rule:     &policy.Rule{ID: "after-rule", Description: "after reason"},
			}
			before := []matrix.Cell{testCell(identity, &mcp.Tool{Name: "github.list_repos"}, "github", beforeExplanation)}
			after := []matrix.Cell{testCell(identity, &mcp.Tool{Name: "github.list_repos"}, "github", afterExplanation)}
			want := []matrix.Change{{
				Identity:   identity,
				Tool:       &mcp.Tool{Name: "github.list_repos"},
				ServerName: "github",
				Kind:       tt.wantKind,
				Before: policy.Explanation{
					Decision: policy.Decision{Allow: tt.beforeAllow, RuleID: "before-rule", Reason: "before reason"},
					Detail:   &policy.MatchResult{Matched: true, RuleID: "before-rule"},
					Rule:     &policy.Rule{ID: "before-rule", Description: "before reason"},
				},
				After: policy.Explanation{
					Decision: policy.Decision{Allow: tt.afterAllow, RuleID: "after-rule", Reason: "after reason"},
					Detail:   &policy.MatchResult{Matched: true, RuleID: "after-rule"},
					Rule:     &policy.Rule{ID: "after-rule", Description: "after reason"},
				},
			}}
			got, err := matrix.Diff(before, after)
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Diff() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestDiff_MatchesByClientAndTool(t *testing.T) {
	laptop := types.Identity{User: "alice", Client: "alice-laptop"}
	bot := types.Identity{User: "alice", Client: "alice-bot"}
	list := &mcp.Tool{Name: "github.list_repos"}
	deleteRepo := &mcp.Tool{Name: "github.delete_repo"}
	allowed := policy.Explanation{Decision: policy.Decision{Allow: true, RuleID: "allow"}}
	denied := policy.Explanation{Decision: policy.Decision{Allow: false, RuleID: "deny"}}
	before := []matrix.Cell{
		testCell(laptop, list, "github", denied),
		testCell(bot, list, "github", allowed),
		testCell(laptop, deleteRepo, "github", denied),
	}
	after := []matrix.Cell{
		testCell(laptop, deleteRepo, "github", denied),
		testCell(laptop, list, "github", allowed),
		testCell(bot, list, "github", denied),
	}

	want := []matrix.Change{
		{Identity: laptop, Tool: deleteRepo, ServerName: "github", Kind: matrix.Unchanged, Before: denied, After: denied},
		{Identity: laptop, Tool: list, ServerName: "github", Kind: matrix.Gained, Before: denied, After: allowed},
		{Identity: bot, Tool: list, ServerName: "github", Kind: matrix.Lost, Before: allowed, After: denied},
	}
	got, err := matrix.Diff(before, after)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %#v, want %#v", got, want)
	}
}

func TestDiff_MismatchedKeys(t *testing.T) {
	identity := types.Identity{Client: "alice-laptop"}
	explanation := policy.Explanation{Decision: policy.Decision{Allow: true}}
	cell := testCell(identity, &mcp.Tool{Name: "github.list_repos"}, "github", explanation)
	otherClient := testCell(types.Identity{Client: "bob-laptop"}, cell.Tool, "github", explanation)
	otherTool := testCell(identity, &mcp.Tool{Name: "github.delete_repo"}, "github", explanation)
	tests := []struct {
		name        string
		before      []matrix.Cell
		after       []matrix.Cell
		wantErrPart string
	}{
		{"empty before", nil, []matrix.Cell{cell}, "present in after but not before"},
		{"empty after", []matrix.Cell{cell}, nil, "present in before but not after"},
		{"different client", []matrix.Cell{cell}, []matrix.Cell{otherClient}, "present in after but not before"},
		{"different tool", []matrix.Cell{cell}, []matrix.Cell{otherTool}, "present in after but not before"},
		{"extra after following a match", []matrix.Cell{cell}, []matrix.Cell{cell, otherTool}, "present in after but not before"},
		{"leftover before following a match", []matrix.Cell{cell, otherTool}, []matrix.Cell{cell}, "present in before but not after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matrix.Diff(tt.before, tt.after)
			if err == nil {
				t.Fatal("Diff() error = nil, want a mismatched-key error")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("Diff() error = %q, want to contain %q", err, tt.wantErrPart)
			}
			if got != nil {
				t.Errorf("Diff() = %#v, want nil on error", got)
			}
		})
	}
}
