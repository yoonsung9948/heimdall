package matrix_test

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/matrix"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
)

func newEngine(t *testing.T, rules []policy.Rule) *policy.Engine {
	t.Helper()
	e, err := policy.NewEngine(rules)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return e
}

func TestBuild_Decision(t *testing.T) {
	engine := newEngine(t, []policy.Rule{
		{
			ID:     "allow-search",
			Effect: policy.EffectAllow,
			Subjects: policy.SubjectSelector{
				Users: []string{"alice"},
			},
			Actions: []policy.Action{policy.ActionToolsCall},
			Resources: policy.ResourceSelector{
				Kinds: []policy.ResourceKind{policy.ResourceKindTool},
				Names: []string{"github.search"},
			},
		},
		{
			ID:     "deny-delete",
			Effect: policy.EffectDeny,
			Subjects: policy.SubjectSelector{
				Users: []string{"alice"},
			},
			Actions: []policy.Action{policy.ActionToolsCall},
			Resources: policy.ResourceSelector{
				Kinds: []policy.ResourceKind{policy.ResourceKindTool},
				Names: []string{"github.delete_repo"},
			},
		},
	})

	identity := types.Identity{Client: "claude", User: "alice"}

	tests := []struct {
		name        string
		toolName    string
		wantAllow   bool
		wantRuleID  string
		wantNilRule bool
	}{
		{
			name:       "explicit allow rule matches",
			toolName:   "github.search",
			wantAllow:  true,
			wantRuleID: "allow-search",
		},
		{
			name:       "explicit deny rule matches",
			toolName:   "github.delete_repo",
			wantAllow:  false,
			wantRuleID: "deny-delete",
		},
		{
			name:        "no rule matches — default deny",
			toolName:    "github.rename_repo",
			wantAllow:   false,
			wantRuleID:  "",
			wantNilRule: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &mcp.Tool{Name: tt.toolName}
			tools := map[string][]*mcp.Tool{"github": {tool}}

			cells := matrix.Build(engine, []types.Identity{identity}, tools)
			if len(cells) != 1 {
				t.Fatalf("len(cells) = %d, want 1", len(cells))
			}
			cell := cells[0]

			if cell.Tool != tool {
				t.Errorf("cell.Tool = %p, want %p (same pointer as input)", cell.Tool, tool)
			}
			if cell.Explanation.Decision.Allow != tt.wantAllow {
				t.Errorf("Decision.Allow = %v, want %v", cell.Explanation.Decision.Allow, tt.wantAllow)
			}
			if cell.Explanation.Decision.RuleID != tt.wantRuleID {
				t.Errorf("Decision.RuleID = %q, want %q", cell.Explanation.Decision.RuleID, tt.wantRuleID)
			}
			if tt.wantNilRule && cell.Explanation.Rule != nil {
				t.Errorf("Explanation.Rule = %+v, want nil", cell.Explanation.Rule)
			}
			if !tt.wantNilRule && cell.Explanation.Rule == nil {
				t.Errorf("Explanation.Rule = nil, want non-nil (rule %q)", tt.wantRuleID)
			}
		})
	}
}

// decisionKey identifies one (identity, server, tool) combination so
// per-cell decisions can be checked without depending on Build's
// iteration order.
type decisionKey struct {
	client, user, server, tool string
}

type wantDecision struct {
	allow   bool
	ruleID  string
	nilRule bool
}

func TestBuild_DecisionPerIdentity(t *testing.T) {
	engine := newEngine(t, []policy.Rule{
		{
			ID:        "allow-alice-search",
			Effect:    policy.EffectAllow,
			Subjects:  policy.SubjectSelector{Users: []string{"alice"}},
			Actions:   []policy.Action{policy.ActionToolsCall},
			Resources: policy.ResourceSelector{Kinds: []policy.ResourceKind{policy.ResourceKindTool}, Names: []string{"github.search"}},
		},
		{
			ID:        "allow-engineering-list",
			Effect:    policy.EffectAllow,
			Subjects:  policy.SubjectSelector{Groups: []string{"engineering"}},
			Actions:   []policy.Action{policy.ActionToolsCall},
			Resources: policy.ResourceSelector{Kinds: []policy.ResourceKind{policy.ResourceKindTool}, Names: []string{"slack.list_channels"}},
		},
		{
			ID:        "deny-bob-delete",
			Effect:    policy.EffectDeny,
			Subjects:  policy.SubjectSelector{Users: []string{"bob"}},
			Actions:   []policy.Action{policy.ActionToolsCall},
			Resources: policy.ResourceSelector{Kinds: []policy.ResourceKind{policy.ResourceKindTool}, Names: []string{"github.delete_repo"}},
		},
	})

	identities := []types.Identity{
		{Client: "claude", User: "alice", Groups: []string{"engineering"}}, // matches both allow rules
		{Client: "cursor", User: "bob"},                                    // matches deny rule only; no group
		{Client: "claude", User: "carol"},                                  // matches nothing
	}

	tools := map[string][]*mcp.Tool{
		"github": {{Name: "github.search"}, {Name: "github.delete_repo"}},
		"slack":  {{Name: "slack.list_channels"}},
	}

	want := map[decisionKey]wantDecision{
		{"claude", "alice", "github", "github.search"}:      {allow: true, ruleID: "allow-alice-search"},
		{"claude", "alice", "github", "github.delete_repo"}: {allow: false, nilRule: true},
		{"claude", "alice", "slack", "slack.list_channels"}: {allow: true, ruleID: "allow-engineering-list"},
		{"cursor", "bob", "github", "github.search"}:        {allow: false, nilRule: true},
		{"cursor", "bob", "github", "github.delete_repo"}:   {allow: false, ruleID: "deny-bob-delete"},
		{"cursor", "bob", "slack", "slack.list_channels"}:   {allow: false, nilRule: true}, // bob isn't in "engineering"
		{"claude", "carol", "github", "github.search"}:      {allow: false, nilRule: true},
		{"claude", "carol", "github", "github.delete_repo"}: {allow: false, nilRule: true},
		{"claude", "carol", "slack", "slack.list_channels"}: {allow: false, nilRule: true},
	}

	cells := matrix.Build(engine, identities, tools)
	if len(cells) != len(want) {
		t.Fatalf("len(cells) = %d, want %d", len(cells), len(want))
	}

	seen := make(map[decisionKey]bool, len(cells))
	for _, cell := range cells {
		k := decisionKey{cell.Identity.Client, cell.Identity.User, cell.ServerName, cell.Tool.Name}
		w, ok := want[k]
		if !ok {
			t.Errorf("unexpected cell for %+v", k)
			continue
		}
		if seen[k] {
			t.Errorf("duplicate cell for %+v", k)
		}
		seen[k] = true

		if cell.Explanation.Decision.Allow != w.allow {
			t.Errorf("%+v: Decision.Allow = %v, want %v", k, cell.Explanation.Decision.Allow, w.allow)
		}
		if cell.Explanation.Decision.RuleID != w.ruleID {
			t.Errorf("%+v: Decision.RuleID = %q, want %q", k, cell.Explanation.Decision.RuleID, w.ruleID)
		}
		if w.nilRule && cell.Explanation.Rule != nil {
			t.Errorf("%+v: Explanation.Rule = %+v, want nil", k, cell.Explanation.Rule)
		}
		if !w.nilRule && cell.Explanation.Rule == nil {
			t.Errorf("%+v: Explanation.Rule = nil, want non-nil (rule %q)", k, w.ruleID)
		}
	}
}

type cellKey struct {
	client, user, server, tool string
}

func TestBuild_CrossProduct(t *testing.T) {
	engine := newEngine(t, []policy.Rule{
		{
			ID:        "allow-all",
			Effect:    policy.EffectAllow,
			Actions:   []policy.Action{policy.ActionToolsCall},
			Resources: policy.ResourceSelector{Kinds: []policy.ResourceKind{policy.ResourceKindTool}},
		},
	})

	identities := []types.Identity{
		{Client: "claude", User: "alice"},
		{Client: "cursor", User: "bob"},
	}

	tools := map[string][]*mcp.Tool{
		"github": {
			{Name: "github.search"},
			{Name: "github.create_issue"},
		},
		"slack": {
			{Name: "slack.send_message"},
			{Name: "slack.list_channels"},
			{Name: "slack.search"},
		},
	}

	wantCount := 0
	want := make(map[cellKey]bool)
	for _, id := range identities {
		for server, tl := range tools {
			for _, tool := range tl {
				want[cellKey{id.Client, id.User, server, tool.Name}] = true
				wantCount++
			}
		}
	}

	cells := matrix.Build(engine, identities, tools)

	if len(cells) != wantCount {
		t.Fatalf("len(cells) = %d, want %d", len(cells), wantCount)
	}

	got := make(map[cellKey]bool, len(cells))
	for _, cell := range cells {
		if !cell.Explanation.Decision.Allow {
			t.Errorf("cell %+v: Allow = false, want true (allow-all rule)", cellKey{
				cell.Identity.Client, cell.Identity.User, cell.ServerName, cell.Tool.Name,
			})
		}
		k := cellKey{cell.Identity.Client, cell.Identity.User, cell.ServerName, cell.Tool.Name}
		if got[k] {
			t.Errorf("duplicate cell for %+v", k)
		}
		got[k] = true
	}

	for k := range want {
		if !got[k] {
			t.Errorf("missing cell for %+v", k)
		}
	}
}

func TestBuild_EmptyInputs(t *testing.T) {
	engine := newEngine(t, nil)

	tests := []struct {
		name       string
		identities []types.Identity
		tools      map[string][]*mcp.Tool
	}{
		{"no identities", nil, map[string][]*mcp.Tool{"github": {{Name: "github.search"}}}},
		{"no tools", []types.Identity{{Client: "claude", User: "alice"}}, map[string][]*mcp.Tool{}},
		{"neither", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := matrix.Build(engine, tt.identities, tt.tools)
			if len(cells) != 0 {
				t.Errorf("len(cells) = %d, want 0", len(cells))
			}
		})
	}
}
