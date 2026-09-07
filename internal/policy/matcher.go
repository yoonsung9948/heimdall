package policy

import (
	"slices"
	"strings"

	"github.com/yoonsung9948/heimdall/internal/types"
)

func (r Rule) Matches(req Request) bool {
	actionMatched := matchAction(r.Actions, req.Action)
	subjectMatched := matchSubject(r.Subjects, req.Identity)
	resourceMatched := matchResource(r.Resources, req.Resource)
	return subjectMatched && actionMatched && resourceMatched
}

func matchUser(users []string, user string) bool {
	return len(users) == 0 || slices.Contains(users, user)
}

func matchClient(clients []string, client string) bool {
	return len(clients) == 0 || slices.Contains(clients, client)
}

func matchGroup(groups []string, identityGroups []string) bool {
	if len(groups) == 0 {
		return true
	}
	for _, group := range identityGroups {
		if slices.Contains(groups, group) {
			return true
		}
	}
	return false
}

func matchSubject(ss SubjectSelector, identity types.Identity) bool {
	return matchUser(ss.Users, identity.User) &&
		matchClient(ss.Clients, identity.Client) &&
		matchGroup(ss.Groups, identity.Groups)
}

func matchKind(kinds []ResourceKind, kind ResourceKind) bool {
	return len(kinds) == 0 || slices.Contains(kinds, kind)
}

func matchName(names []string, name string) bool {
	if len(names) == 0 {
		return true
	}
	for _, pattern := range names {
		if matchPattern(pattern, name) {
			return true
		}
	}
	return false
}

func matchResource(rs ResourceSelector, resource Resource) bool {
	return matchKind(rs.Kinds, resource.Kind) && matchName(rs.Names, resource.Name)
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

func (r Rule) evaluateDetailed(req Request) MatchResult {
	userMatched := matchUser(r.Subjects.Users, req.Identity.User)
	clientMatched := matchClient(r.Subjects.Clients, req.Identity.Client)
	groupMatched := matchGroup(r.Subjects.Groups, req.Identity.Groups)
	actionMatched := matchAction(r.Actions, req.Action)
	rsKindMatched := matchKind(r.Resources.Kinds, req.Resource.Kind)
	rsNameMatched := matchName(r.Resources.Names, req.Resource.Name)
	matched := r.Matches(req)
	return MatchResult{
		Matched: matched,
		RuleID:  r.ID,
		Subjects: SubjectMatch{
			UserMatched:   userMatched,
			ClientMatched: clientMatched,
			GroupMatched:  groupMatched,
		},
		Action: actionMatched,
		Resource: ResourceMatch{
			KindMatched: rsKindMatched,
			NameMatched: rsNameMatched,
		},
	}
}

type MatchResult struct {
	Matched  bool
	RuleID   string
	Subjects SubjectMatch
	Action   bool
	Resource ResourceMatch
}

type SubjectMatch struct {
	UserMatched   bool
	ClientMatched bool
	GroupMatched  bool
}

type ResourceMatch struct {
	KindMatched bool
	NameMatched bool
}
