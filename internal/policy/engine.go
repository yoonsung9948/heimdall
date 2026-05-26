package policy

import (
	"errors"
	"fmt"
	"strings"
)

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

type Action string

const (
	ActionToolsList     Action = "tools/list"
	ActionToolsCall     Action = "tools/call"
	ActionResourcesList Action = "resources/list"
	ActionResourcesRead Action = "resources/read"
	ActionPromptsList   Action = "prompts/list"
	ActionPromptsGet    Action = "prompts/get"
)

type Identity struct {
	User   string
	Client string
	Groups []string
}

type SubjectSelector struct {
	Users   []string
	Clients []string
	Groups  []string
}

type ResourceKind string

const (
	ResourceKindTool     ResourceKind = "tool"
	ResourceKindResource ResourceKind = "resource"
	ResourceKindPrompt   ResourceKind = "prompt"
)

type Resource struct {
	Kind ResourceKind
	Name string
}

type ResourceSelector struct {
	Kinds []ResourceKind
	Names []string
}

type Request struct {
	Identity Identity
	Action   Action
	Resource Resource
}

type Decision struct {
	Allow  bool
	Reason string
	RuleID string
}

type Rule struct {
	ID          string
	Effect      Effect
	Description string

	Subjects  SubjectSelector
	Actions   []Action
	Resources ResourceSelector
}

type Engine struct {
	rules []Rule
}

func NewEngine(rules []Rule) (*Engine, error) {
	if err := validateRules(rules); err != nil {
		return nil, err
	}
	return &Engine{
		rules: copyRules(rules),
	}, nil
}

func (e *Engine) Authorize(req Request) Decision {
	var allow *Rule

	for i := range e.rules {
		rule := &e.rules[i]

		if !rule.Matches(req) {
			continue
		}

		switch rule.Effect {
		case EffectDeny:
			return Decision{
				Allow:  false,
				RuleID: rule.ID,
				Reason: reasonForRule(*rule, "denied by matching rule"),
			}

		case EffectAllow:
			if allow == nil {
				allow = rule
			}
		}
	}

	if allow != nil {
		return Decision{
			Allow:  true,
			RuleID: allow.ID,
			Reason: reasonForRule(*allow, "allowed by matching rule"),
		}
	}

	return Decision{
		Allow:  false,
		RuleID: "",
		Reason: "denied by default: no matching allow rule",
	}
}

func validateRules(rules []Rule) error {
	seen := make(map[string]struct{})

	for _, rule := range rules {
		if rule.ID == "" {
			return errors.New("rule id is required")
		}

		fields := []struct {
			name   string
			values []string
		}{
			{"Subjects.Users", rule.Subjects.Users},
			{"Subjects.Clients", rule.Subjects.Clients},
			{"Subjects.Groups", rule.Subjects.Groups},
			{"Resources.Names", rule.Resources.Names},
		}

		for _, field := range fields {
			if err := validateNonEmptyStrings(rule.ID, field.name, field.values); err != nil {
				return err
			}
		}

		if _, ok := seen[rule.ID]; ok {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		seen[rule.ID] = struct{}{}

		if !validEffect(rule.Effect) {
			return fmt.Errorf("rule %q: invalid effect %q", rule.ID, rule.Effect)
		}

		for _, kind := range rule.Resources.Kinds {
			if !validResourceKind(kind) {
				return fmt.Errorf("rule %q: invalid resource kind %q", rule.ID, kind)
			}
		}
		for _, name := range rule.Resources.Names {
			if !validResourceNamePattern(name) {
				return fmt.Errorf("rule %q: invalid resource name %q", rule.ID, name)
			}
		}

		for _, action := range rule.Actions {
			if !validAction(action) {
				return fmt.Errorf("rule %q: invalid action %q", rule.ID, action)
			}
		}
	}

	return nil
}

func validEffect(effect Effect) bool {
	return effect == EffectAllow || effect == EffectDeny
}

func validResourceKind(kind ResourceKind) bool {
	switch kind {
	case ResourceKindTool,
		ResourceKindResource,
		ResourceKindPrompt:
		return true
	default:
		return false
	}
}

func validResourceNamePattern(name string) bool {
	return validPattern(name)
}

func validAction(action Action) bool {
	if !validPattern(string(action)) {
		return false
	}
	switch action {
	case "*",
		"tools/*",
		"resources/*",
		"prompts/*",
		ActionToolsList,
		ActionToolsCall,
		ActionResourcesRead,
		ActionResourcesList,
		ActionPromptsList,
		ActionPromptsGet:
		return true
	default:
		return false
	}
}

func validateNonEmptyStrings(ruleID, field string, values []string) error {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("rule %q: %s contains empty or whitespace-only string", ruleID, field)
		}
		if trimmed != value {
			return fmt.Errorf("rule %q: %s contains leading or trailing whitespace", ruleID, field)
		}
	}
	return nil
}

func validPattern(pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		return strings.HasSuffix(pattern, "*") &&
			strings.Count(pattern, "*") == 1 &&
			len(strings.TrimSuffix(pattern, "*")) > 0
	}
	return true
}

func reasonForRule(rule Rule, fallback string) string {
	if rule.Description != "" {
		return rule.Description
	}
	return fallback
}

func copyRules(rules []Rule) []Rule {
	copied := make([]Rule, len(rules))
	for i, rule := range rules {
		copied[i] = rule
		copied[i].Actions = append([]Action(nil), rule.Actions...)
		copied[i].Subjects.Users = append([]string(nil), rule.Subjects.Users...)
		copied[i].Subjects.Clients = append([]string(nil), rule.Subjects.Clients...)
		copied[i].Subjects.Groups = append([]string(nil), rule.Subjects.Groups...)
		copied[i].Resources.Kinds = append([]ResourceKind(nil), rule.Resources.Kinds...)
		copied[i].Resources.Names = append([]string(nil), rule.Resources.Names...)
	}
	return copied
}
