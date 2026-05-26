package policy

import (
	"strings"
	"sync"
	"testing"
)

func TestNewEngine_ValidRules(t *testing.T) {
	tests := []struct {
		name  string
		rules []Rule
	}{
		{
			name:  "nil rules is valid and means default deny",
			rules: nil,
		},
		{
			name:  "empty rules is valid and means default deny",
			rules: []Rule{},
		},
		{
			name: "single allow rule is valid",
			rules: []Rule{
				{
					ID:     "allow-github",
					Effect: EffectAllow,
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.*",
						},
					},
				},
			},
		},
		{
			name: "single deny rule is valid",
			rules: []Rule{
				{
					ID:     "deny-github-delete",
					Effect: EffectDeny,
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.delete_repo",
						},
					},
				},
			},
		},
		{
			name: "rule with unconstrained selectors is valid",
			rules: []Rule{
				{
					ID:     "allow-all",
					Effect: EffectAllow,
				},
			},
		},
		{
			name: "rule with all subject selector fields is valid",
			rules: []Rule{
				{
					ID:     "allow-alice-from-claude",
					Effect: EffectAllow,
					Subjects: SubjectSelector{
						Users:   []string{"alice@example.com"},
						Clients: []string{"claude-desktop"},
						Groups:  []string{"engineering"},
					},
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.*",
						},
					},
				},
			},
		},
		{
			name: "rule with exact supported actions is valid",
			rules: []Rule{
				{
					ID:     "allow-supported-actions",
					Effect: EffectAllow,
					Actions: []Action{
						ActionToolsList,
						ActionToolsCall,
						ActionResourcesList,
						ActionResourcesRead,
						ActionPromptsList,
						ActionPromptsGet,
					},
				},
			},
		},
		{
			name: "rule with all resource kinds is valid",
			rules: []Rule{
				{
					ID:     "allow-all-resource-kinds",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
							ResourceKindResource,
							ResourceKindPrompt,
						},
					},
				},
			},
		},
		{
			name: "rule with resource name patterns is valid",
			rules: []Rule{
				{
					ID:     "allow-patterns",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Names: []string{
							"*",
							"github.*",
							"file:///repo/*",
							"deploy-*",
							"github.create_pr",
						},
					},
				},
			},
		},
		{
			name: "allow and deny overlap is valid because deny precedence makes semantics deterministic",
			rules: []Rule{
				{
					ID:     "allow-github",
					Effect: EffectAllow,
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.*",
						},
					},
				},
				{
					ID:     "deny-github-delete",
					Effect: EffectDeny,
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.delete_repo",
						},
					},
				},
			},
		},
		{
			name: "exact allow and exact deny overlap is valid but deny wins at evaluation time",
			rules: []Rule{
				{
					ID:     "allow-delete",
					Effect: EffectAllow,
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.delete_repo",
						},
					},
				},
				{
					ID:     "deny-delete",
					Effect: EffectDeny,
					Actions: []Action{
						ActionToolsCall,
					},
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKindTool,
						},
						Names: []string{
							"github.delete_repo",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(tt.rules)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}
			if engine == nil {
				t.Fatalf("NewEngine() returned nil engine")
			}
		})
	}
}

func TestNewEngine_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		rules   []Rule
		wantErr string
	}{
		{
			name: "missing rule id",
			rules: []Rule{
				{
					Effect: EffectAllow,
				},
			},
			wantErr: "rule id",
		},
		{
			name: "whitespace-only rule id",
			rules: []Rule{
				{
					ID:     "   ",
					Effect: EffectAllow,
				},
			},
			wantErr: "rule id",
		},
		{
			name: "duplicate rule id",
			rules: []Rule{
				{
					ID:     "duplicate",
					Effect: EffectAllow,
				},
				{
					ID:     "duplicate",
					Effect: EffectDeny,
				},
			},
			wantErr: "duplicate",
		},
		{
			name: "invalid effect",
			rules: []Rule{
				{
					ID:     "invalid-effect",
					Effect: Effect("maybe"),
				},
			},
			wantErr: "invalid effect",
		},
		{
			name: "empty effect",
			rules: []Rule{
				{
					ID:     "empty-effect",
					Effect: Effect(""),
				},
			},
			wantErr: "invalid effect",
		},
		{
			name: "invalid resource kind",
			rules: []Rule{
				{
					ID:     "invalid-kind",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKind("database"),
						},
					},
				},
			},
			wantErr: "invalid resource kind",
		},
		{
			name: "empty resource kind",
			rules: []Rule{
				{
					ID:     "empty-kind",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Kinds: []ResourceKind{
							ResourceKind(""),
						},
					},
				},
			},
			wantErr: "invalid resource kind",
		},
		{
			name: "invalid action",
			rules: []Rule{
				{
					ID:     "invalid-action",
					Effect: EffectAllow,
					Actions: []Action{
						Action("bad/action"),
					},
				},
			},
			wantErr: "invalid action",
		},
		{
			name: "empty action",
			rules: []Rule{
				{
					ID:     "empty-action",
					Effect: EffectAllow,
					Actions: []Action{
						Action(""),
					},
				},
			},
			wantErr: "invalid action",
		},
		{
			name: "empty subject user",
			rules: []Rule{
				{
					ID:     "empty-user",
					Effect: EffectAllow,
					Subjects: SubjectSelector{
						Users: []string{""},
					},
				},
			},
			wantErr: "Subjects.Users",
		},
		{
			name: "whitespace-only subject user",
			rules: []Rule{
				{
					ID:     "whitespace-user",
					Effect: EffectAllow,
					Subjects: SubjectSelector{
						Users: []string{"   "},
					},
				},
			},
			wantErr: "Subjects.Users",
		},
		{
			name: "subject user with leading whitespace",
			rules: []Rule{
				{
					ID:     "leading-space-user",
					Effect: EffectAllow,
					Subjects: SubjectSelector{
						Users: []string{" alice@example.com"},
					},
				},
			},
			wantErr: "Subjects.Users",
		},
		{
			name: "empty subject client",
			rules: []Rule{
				{
					ID:     "empty-client",
					Effect: EffectAllow,
					Subjects: SubjectSelector{
						Clients: []string{""},
					},
				},
			},
			wantErr: "Subjects.Clients",
		},
		{
			name: "empty subject group",
			rules: []Rule{
				{
					ID:     "empty-group",
					Effect: EffectAllow,
					Subjects: SubjectSelector{
						Groups: []string{""},
					},
				},
			},
			wantErr: "Subjects.Groups",
		},
		{
			name: "empty resource name",
			rules: []Rule{
				{
					ID:     "empty-resource-name",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Names: []string{""},
					},
				},
			},
			wantErr: "Resources.Names",
		},
		{
			name: "whitespace-only resource name",
			rules: []Rule{
				{
					ID:     "whitespace-resource-name",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Names: []string{"   "},
					},
				},
			},
			wantErr: "Resources.Names",
		},
		{
			name: "invalid resource pattern with leading wildcard",
			rules: []Rule{
				{
					ID:     "invalid-resource-pattern",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Names: []string{"*delete"},
					},
				},
			},
			wantErr: "resource",
		},
		{
			name: "invalid resource pattern with embedded wildcard",
			rules: []Rule{
				{
					ID:     "invalid-resource-pattern",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Names: []string{"git*hub"},
					},
				},
			},
			wantErr: "resource",
		},
		{
			name: "invalid resource pattern with multiple wildcards",
			rules: []Rule{
				{
					ID:     "invalid-resource-pattern",
					Effect: EffectAllow,
					Resources: ResourceSelector{
						Names: []string{"github.**"},
					},
				},
			},
			wantErr: "resource",
		},
		{
			name: "invalid action wildcard",
			rules: []Rule{
				{
					ID:     "invalid-action-wildcard",
					Effect: EffectAllow,
					Actions: []Action{
						Action("tool*"),
					},
				},
			},
			wantErr: "invalid action",
		},
		{
			name: "unsupported action namespace wildcard",
			rules: []Rule{
				{
					ID:     "unsupported-action-wildcard",
					Effect: EffectAllow,
					Actions: []Action{
						Action("unknown/*"),
					},
				},
			},
			wantErr: "invalid action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(tt.rules)
			if err == nil {
				t.Fatalf("NewEngine() error = nil, want error; engine = %+v", engine)
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NewEngine() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAuthorize_DefaultDeny(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Identity: Identity{
			User:   "alice@example.com",
			Client: "claude-desktop",
			Groups: []string{"engineering"},
		},
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	})

	assertDecision(t, decision, false, "", "denied by default")
}

func TestAuthorize_MatchingAllow(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:          "allow-engineering-github",
			Description: "engineering can call github tools",
			Effect:      EffectAllow,
			Subjects: SubjectSelector{
				Groups:  []string{"engineering"},
				Clients: []string{"claude-desktop"},
			},
			Actions: []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Identity: Identity{
			User:   "alice@example.com",
			Client: "claude-desktop",
			Groups: []string{"engineering"},
		},
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	})

	assertDecision(t, decision, true, "allow-engineering-github", "engineering can call github tools")
}

func TestAuthorize_MatchingDeny(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:          "deny-github-delete",
			Description: "repository deletion is blocked",
			Effect:      EffectDeny,
			Actions:     []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.delete_repo"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.delete_repo",
		},
	})

	assertDecision(t, decision, false, "deny-github-delete", "repository deletion is blocked")
}

func TestAuthorize_NonMatchingAllowDefaultsToDeny(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:     "allow-github",
			Effect: EffectAllow,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"github.*",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "postgres.query",
		},
	})

	assertDecision(t, decision, false, "", "denied by default")
}

func TestAuthorize_DenyOverridesEarlierAllow(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:     "allow-github",
			Effect: EffectAllow,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"github.*",
				},
			},
		},
		{
			ID:     "deny-delete",
			Effect: EffectDeny,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"github.delete_repo",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.delete_repo",
		},
	})

	assertDecision(t, decision, false, "deny-delete", "denied by matching rule")
}

func TestAuthorize_DenyOverridesLaterAllow(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:     "deny-delete",
			Effect: EffectDeny,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"github.delete_repo",
				},
			},
		},
		{
			ID:     "allow-github",
			Effect: EffectAllow,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"github.*",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.delete_repo",
		},
	})

	assertDecision(t, decision, false, "deny-delete", "denied by matching rule")
}

func TestAuthorize_ExactAllowAndExactDenyDenies(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:      "allow-delete",
			Effect:  EffectAllow,
			Actions: []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.delete_repo"},
			},
		},
		{
			ID:      "deny-delete",
			Effect:  EffectDeny,
			Actions: []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.delete_repo"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.delete_repo",
		},
	})

	assertDecision(t, decision, false, "deny-delete", "denied by matching rule")
}

func TestAuthorize_FirstMatchingAllowDeterminesRuleID(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:     "allow-all-tools",
			Effect: EffectAllow,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"*",
				},
			},
		},
		{
			ID:     "allow-github",
			Effect: EffectAllow,
			Actions: []Action{
				ActionToolsCall,
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{
					ResourceKindTool,
				},
				Names: []string{
					"github.*",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	})

	assertDecision(t, decision, true, "allow-all-tools", "allowed by matching rule")
}

func TestAuthorize_UsesFallbackReasonWhenDescriptionEmpty(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:      "allow-github",
			Effect:  EffectAllow,
			Actions: []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	})

	assertDecision(t, decision, true, "allow-github", "allowed by matching rule")
}

func TestAuthorize_UsesRuleDescriptionAsReason(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:          "allow-github",
			Description: "github access allowed for engineering",
			Effect:      EffectAllow,
			Actions:     []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	})

	assertDecision(t, decision, true, "allow-github", "github access allowed for engineering")
}

func TestAuthorize_UnconstrainedAllowMatchesAnyRequest(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:     "allow-all",
			Effect: EffectAllow,
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	decision := engine.Authorize(Request{
		Identity: Identity{
			User:   "anyone@example.com",
			Client: "any-client",
			Groups: []string{"any-group"},
		},
		Action: ActionPromptsGet,
		Resource: Resource{
			Kind: ResourceKindPrompt,
			Name: "anything",
		},
	})

	assertDecision(t, decision, true, "allow-all", "allowed by matching rule")
}

func TestNewEngine_DeepCopiesRules(t *testing.T) {
	rules := []Rule{
		{
			ID:      "allow-github",
			Effect:  EffectAllow,
			Actions: []Action{ActionToolsCall},
			Subjects: SubjectSelector{
				Users:   []string{"alice@example.com"},
				Clients: []string{"claude-desktop"},
				Groups:  []string{"engineering"},
			},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.*"},
			},
		},
	}

	engine, err := NewEngine(rules)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	rules[0].Effect = EffectDeny
	rules[0].Actions[0] = ActionResourcesRead
	rules[0].Subjects.Users[0] = "bob@example.com"
	rules[0].Subjects.Clients[0] = "untrusted-client"
	rules[0].Subjects.Groups[0] = "finance"
	rules[0].Resources.Kinds[0] = ResourceKindPrompt
	rules[0].Resources.Names[0] = "postgres.*"

	decision := engine.Authorize(Request{
		Identity: Identity{
			User:   "alice@example.com",
			Client: "claude-desktop",
			Groups: []string{"engineering"},
		},
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	})

	if !decision.Allow {
		t.Fatalf("engine was affected by caller mutation; decision = %+v", decision)
	}
	if decision.RuleID != "allow-github" {
		t.Fatalf("RuleID = %q, want %q", decision.RuleID, "allow-github")
	}
}

func TestAuthorize_ConcurrentReadSafe(t *testing.T) {
	engine, err := NewEngine([]Rule{
		{
			ID:     "allow-github",
			Effect: EffectAllow,
			Subjects: SubjectSelector{
				Groups: []string{"engineering"},
			},
			Actions: []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.*"},
			},
		},
		{
			ID:      "deny-delete",
			Effect:  EffectDeny,
			Actions: []Action{ActionToolsCall},
			Resources: ResourceSelector{
				Kinds: []ResourceKind{ResourceKindTool},
				Names: []string{"github.delete_repo"},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	req := Request{
		Identity: Identity{
			User:   "alice@example.com",
			Client: "claude-desktop",
			Groups: []string{"engineering"},
		},
		Action: ActionToolsCall,
		Resource: Resource{
			Kind: ResourceKindTool,
			Name: "github.create_pr",
		},
	}

	const goroutines = 32
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
				decision := engine.Authorize(req)
				if !decision.Allow {
					t.Errorf("Authorize().Allow = false, want true; decision = %+v", decision)
					return
				}
				if decision.RuleID != "allow-github" {
					t.Errorf("RuleID = %q, want %q", decision.RuleID, "allow-github")
					return
				}
			}
		}()
	}

	wg.Wait()
}

func assertDecision(t *testing.T, got Decision, wantAllow bool, wantRuleID string, wantReasonContains string) {
	t.Helper()

	if got.Allow != wantAllow {
		t.Fatalf("Allow = %v, want %v; decision = %+v", got.Allow, wantAllow, got)
	}

	if got.RuleID != wantRuleID {
		t.Fatalf("RuleID = %q, want %q; decision = %+v", got.RuleID, wantRuleID, got)
	}

	if wantReasonContains != "" && !strings.Contains(got.Reason, wantReasonContains) {
		t.Fatalf("Reason = %q, want substring %q; decision = %+v", got.Reason, wantReasonContains, got)
	}
}
