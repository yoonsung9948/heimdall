package policy

import "testing"

func TestRuleMatches_SubjectSelectors(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		req  Request
		want bool
	}{
		{
			name: "empty subject selector matches any identity",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User:   "alice@example.com",
					Client: "unknown-client",
					Groups: []string{"random"},
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "exact user matches",
			rule: Rule{
				Subjects: SubjectSelector{
					Users: []string{"alice@example.com"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
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
			},
			want: true,
		},
		{
			name: "wrong user does not match",
			rule: Rule{
				Subjects: SubjectSelector{
					Users: []string{"alice@example.com"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User:   "bob@example.com",
					Client: "claude-desktop",
					Groups: []string{"engineering"},
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
		{
			name: "exact client matches",
			rule: Rule{
				Subjects: SubjectSelector{
					Clients: []string{"claude-desktop"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
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
			},
			want: true,
		},
		{
			name: "wrong client does not match",
			rule: Rule{
				Subjects: SubjectSelector{
					Clients: []string{"trusted-client"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User:   "alice@example.com",
					Client: "untrusted-client",
					Groups: []string{"engineering"},
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
		{
			name: "any group match is enough",
			rule: Rule{
				Subjects: SubjectSelector{
					Groups: []string{"platform"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User:   "alice@example.com",
					Client: "claude-desktop",
					Groups: []string{"engineering", "platform"},
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "non-empty groups selector does not match identity with no groups",
			rule: Rule{
				Subjects: SubjectSelector{
					Groups: []string{"engineering"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{},
				Action:   ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
		{
			name: "subject dimensions are ANDed",
			rule: Rule{
				Subjects: SubjectSelector{
					Users:   []string{"alice@example.com"},
					Clients: []string{"claude-desktop"},
					Groups:  []string{"engineering"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User:   "alice@example.com",
					Client: "claude-code",
					Groups: []string{"engineering"},
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
		{
			name: "subject dimensions match when all constrained fields match",
			rule: Rule{
				Subjects: SubjectSelector{
					Users:   []string{"alice@example.com"},
					Clients: []string{"claude-desktop"},
					Groups:  []string{"engineering"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User:   "alice@example.com",
					Client: "claude-desktop",
					Groups: []string{"engineering", "platform"},
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
	}

	runRuleMatchTests(t, tests)
}

func TestRuleMatches_ActionSelectors(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		req  Request
		want bool
	}{
		{
			name: "empty actions selector matches any action",
			rule: Rule{
				Subjects: SubjectSelector{
					Clients: []string{"claude-code"},
				},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					Client: "claude-code",
				},
				Action: ActionPromptsGet,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "exact action matches",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "wrong exact action does not match",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Action: ActionToolsList,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
		{
			name: "tools wildcard matches tools list",
			rule: Rule{
				Actions: []Action{"tools/*"},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"*"},
				},
			},
			req: Request{
				Action: ActionToolsList,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "tools wildcard matches tools call",
			rule: Rule{
				Actions: []Action{"tools/*"},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "tools wildcard does not match resources read",
			rule: Rule{
				Actions: []Action{"tools/*"},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindResource},
					Names: []string{"file:///repo/*"},
				},
			},
			req: Request{
				Action: ActionResourcesRead,
				Resource: Resource{
					Kind: ResourceKindResource,
					Name: "file:///repo/README.md",
				},
			},
			want: false,
		},
		{
			name: "global action wildcard matches prompt get",
			rule: Rule{
				Actions: []Action{"*"},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindPrompt},
					Names: []string{"deploy-*"},
				},
			},
			req: Request{
				Action: ActionPromptsGet,
				Resource: Resource{
					Kind: ResourceKindPrompt,
					Name: "deploy-prod",
				},
			},
			want: true,
		},
	}

	runRuleMatchTests(t, tests)
}

func TestRuleMatches_ResourceSelectors(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		req  Request
		want bool
	}{
		{
			name: "empty resource selector matches any resource",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "anything",
				},
			},
			want: true,
		},
		{
			name: "exact resource kind and name match",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.create_pr"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "resource kind and name are ANDed",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindPrompt,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
		{
			name: "resource kind selector alone matches any resource name",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "postgres.query",
				},
			},
			want: true,
		},
		{
			name: "resource name selector alone matches any resource kind",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "resource name prefix wildcard matches tool",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
		{
			name: "resource name prefix wildcard does not match different prefix",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "postgres.query",
				},
			},
			want: false,
		},
		{
			name: "resource name prefix wildcard matches file URI",
			rule: Rule{
				Actions: []Action{ActionResourcesRead},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindResource},
					Names: []string{"file:///repo/*"},
				},
			},
			req: Request{
				Action: ActionResourcesRead,
				Resource: Resource{
					Kind: ResourceKindResource,
					Name: "file:///repo/README.md",
				},
			},
			want: true,
		},
		{
			name: "resource exact name does not match prefix sibling",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.create_pr"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr_draft",
				},
			},
			want: false,
		},
		{
			name: "multiple resource patterns match if any name pattern matches",
			rule: Rule{
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"slack.*", "github.*"},
				},
			},
			req: Request{
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: true,
		},
	}

	runRuleMatchTests(t, tests)
}

func TestRuleMatches_Composition(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		req  Request
		want bool
	}{
		{
			name: "matches when subject action and resource all match",
			rule: Rule{
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
			req: Request{
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
			},
			want: true,
		},
		{
			name: "does not match when only subject matches",
			rule: Rule{
				Subjects: SubjectSelector{
					Users: []string{"alice@example.com"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User: "alice@example.com",
				},
				Action: ActionResourcesRead,
				Resource: Resource{
					Kind: ResourceKindResource,
					Name: "file:///repo/README.md",
				},
			},
			want: false,
		},
		{
			name: "does not match when only action matches",
			rule: Rule{
				Subjects: SubjectSelector{
					Users: []string{"alice@example.com"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					User: "bob@example.com",
				},
				Action: ActionToolsCall,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "postgres.query",
				},
			},
			want: false,
		},
		{
			name: "does not match when only resource matches",
			rule: Rule{
				Subjects: SubjectSelector{
					Groups: []string{"engineering"},
				},
				Actions: []Action{ActionToolsCall},
				Resources: ResourceSelector{
					Kinds: []ResourceKind{ResourceKindTool},
					Names: []string{"github.*"},
				},
			},
			req: Request{
				Identity: Identity{
					Groups: []string{"finance"},
				},
				Action: ActionResourcesRead,
				Resource: Resource{
					Kind: ResourceKindTool,
					Name: "github.create_pr",
				},
			},
			want: false,
		},
	}

	runRuleMatchTests(t, tests)
}

func runRuleMatchTests(t *testing.T, tests []struct {
	name string
	rule Rule
	req  Request
	want bool
}) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.Matches(tt.req)
			if got != tt.want {
				t.Fatalf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
