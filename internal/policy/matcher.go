package policy

import (
	"slices"
	"strings"

	"github.com/yoonsung9948/heimdall/internal/types"
)

func (r Rule) Matches(request Request) bool {
	actionMatches := matchAction(r.Actions, request.Action)
	subjectMatches := matchSubject(r.Subjects, request.Identity)
	resourceMatches := matchResource(r.Resources, request.Resource)
	return subjectMatches && actionMatches && resourceMatches
}

func matchAction(actions []Action, action Action) bool {
	if len(actions) == 0 {
		return true
	}
	for _, pattern := range actions {
		if matchPattern(string(pattern), string(action)) {
			return true
		}
	}
	return false
}

func matchSubject(ss SubjectSelector, identity types.Identity) bool {
	userMatches := len(ss.Users) == 0 || slices.Contains(ss.Users, identity.User)
	clientMatches := len(ss.Clients) == 0 || slices.Contains(ss.Clients, identity.Client)
	groupMatches := len(ss.Groups) == 0

	if len(ss.Groups) > 0 {
		for _, group := range identity.Groups {
			if slices.Contains(ss.Groups, group) {
				groupMatches = true
				break
			}
		}
	}

	return userMatches && clientMatches && groupMatches
}

func matchResource(rs ResourceSelector, resource Resource) bool {
	kindMatches := len(rs.Kinds) == 0 || slices.Contains(rs.Kinds, resource.Kind)
	nameMatches := len(rs.Names) == 0
	if len(rs.Names) > 0 {
		for _, pattern := range rs.Names {
			if matchPattern(pattern, resource.Name) {
				nameMatches = true
				break
			}
		}
	}
	return kindMatches && nameMatches
}

func matchPattern(pattern, value string) bool {
	switch {
	case pattern == "*":
		return true
	case strings.HasSuffix(pattern, "*"):
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	default:
		return pattern == value
	}
}

// TODO: matcher should return matchResult for granular match tracing
// engine should be able to produce detailed reasons about the decision
type MatchResult struct {
	Matched  bool
	RuleID   string
	Subjects FieldMatch
	Action   FieldMatch
	Resource FieldMatch
}

type FieldMatch struct {
	Matched bool
	Reason  string
}
