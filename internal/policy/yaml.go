package policy

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type YAMLPolicy struct {
	Rules []YAMLRule `yaml:"rules"`
}

type YAMLRule struct {
	ID          string               `yaml:"id"`
	Description string               `yaml:"description"`
	Effect      string               `yaml:"effect"`
	Subjects    YAMLSubjectSelector  `yaml:"subjects"`
	Actions     []string             `yaml:"actions"`
	Resources   YAMLResourceSelector `yaml:"resources"`
}

type YAMLSubjectSelector struct {
	Users   []string `yaml:"users"`
	Clients []string `yaml:"clients"`
	Groups  []string `yaml:"groups"`
}

type YAMLResourceSelector struct {
	Kinds []string `yaml:"kinds"`
	Names []string `yaml:"names"`
}

func LoadFile(path string) (*Engine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	e, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("load policy file %q: %w", path, err)
	}
	return e, nil
}

func Load(r io.Reader) (*Engine, error) {
	var yp YAMLPolicy
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&yp); err != nil {
		return nil, fmt.Errorf("decode policy yaml: %w", err)
	}
	rules := yp.toRules()
	return NewEngine(rules)
}

func (y YAMLPolicy) toRules() []Rule {
	out := make([]Rule, len(y.Rules))
	for i, yr := range y.Rules {
		ss := SubjectSelector{
			Users:   normalizeStrings(yr.Subjects.Users),
			Clients: normalizeStrings(yr.Subjects.Clients),
			Groups:  normalizeStrings(yr.Subjects.Groups),
		}
		rs := ResourceSelector{
			Kinds: toResourceKinds(normalizeKeywords(yr.Resources.Kinds)),
			Names: normalizeStrings(yr.Resources.Names),
		}
		rule := Rule{
			ID:          normalizeString(yr.ID),
			Effect:      Effect(normalizeKeyword(yr.Effect)),
			Description: normalizeString(yr.Description),
			Subjects:    ss,
			Actions:     toActions(normalizeKeywords(yr.Actions)),
			Resources:   rs,
		}
		out[i] = rule
	}
	return out
}

func normalizeKeyword(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeString(value string) string {
	return strings.TrimSpace(value)
}

func normalizeStrings(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = normalizeString(value)
	}
	return out
}

func normalizeKeywords(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = normalizeKeyword(value)
	}
	return out
}

func toActions(values []string) []Action {
	out := make([]Action, len(values))
	for i, value := range values {
		out[i] = Action(value)
	}
	return out
}

func toResourceKinds(values []string) []ResourceKind {
	out := make([]ResourceKind, len(values))
	for i, value := range values {
		out[i] = ResourceKind(value)
	}
	return out
}
