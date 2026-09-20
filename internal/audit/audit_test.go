package audit

import (
	"reflect"
	"testing"
	"time"

	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
)

func TestNewAuditEvent(t *testing.T) {
	timestamp := time.Date(2026, time.September, 19, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		req        policy.Request
		serverName string
		decision   policy.Decision
		want       AuditEvent
	}{
		{
			name: "allow decision",
			req: policy.Request{
				Identity: types.Identity{
					User:   "alice",
					Client: "cli-1",
					Groups: []string{"admins"},
				},
				Action: policy.ActionToolsCall,
				Resource: policy.Resource{
					Kind: policy.ResourceKindTool,
					Name: "docs.search",
				},
			},
			serverName: "docs",
			decision: policy.Decision{
				Allow:  true,
				Reason: "allowed by matching rule",
				RuleID: "allow-admins",
			},
			want: AuditEvent{
				Timestamp:      timestamp,
				UserName:       "alice",
				ClientName:     "cli-1",
				ToolName:       "docs.search",
				ServerName:     "docs",
				Groups:         []string{"admins"},
				DecisionAllow:  true,
				DecisionReason: "allowed by matching rule",
				RuleID:         "allow-admins",
			},
		},
		{
			name: "deny with no matching rule and no groups",
			req: policy.Request{
				Identity: types.Identity{
					User:   "bob",
					Client: "cli-2",
					Groups: nil,
				},
				Action: policy.ActionToolsCall,
				Resource: policy.Resource{
					Kind: policy.ResourceKindTool,
					Name: "docs.delete",
				},
			},
			serverName: "docs",
			decision: policy.Decision{
				Allow:  false,
				Reason: "denied by default: no matching allow rule",
				RuleID: "",
			},
			want: AuditEvent{
				Timestamp:      timestamp,
				UserName:       "bob",
				ClientName:     "cli-2",
				ToolName:       "docs.delete",
				ServerName:     "docs",
				Groups:         nil,
				DecisionAllow:  false,
				DecisionReason: "denied by default: no matching allow rule",
				RuleID:         "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAuditEvent(tt.req, tt.serverName, tt.decision, timestamp)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAuditEvent() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewAuditEvent_GroupsIsIndependentOfSourceSlice(t *testing.T) {
	// NewAuditEvent copies Groups into the event rather than sharing the
	// caller's backing array. An audit record should be an immutable
	// snapshot at the moment of the decision — mutating the source
	// Identity's Groups afterward (or a Logger goroutine encoding the event
	// concurrently) must never see or cause a change here.
	groups := []string{"admins"}
	req := policy.Request{
		Identity: types.Identity{User: "alice", Client: "cli-1", Groups: groups},
		Resource: policy.Resource{Kind: policy.ResourceKindTool, Name: "docs.search"},
	}

	event := NewAuditEvent(req, "docs", policy.Decision{Allow: true}, time.Now())

	groups[0] = "mutated"

	if event.Groups[0] != "admins" {
		t.Fatalf("expected AuditEvent.Groups to be independent of the source slice; got %v", event.Groups)
	}

	// Mutating the event's own Groups must not reach back into the
	// original slice either — independence should hold in both directions.
	event.Groups[0] = "also-mutated"
	if groups[0] != "mutated" {
		t.Fatalf("expected source slice to be unaffected by mutating AuditEvent.Groups; got %v", groups)
	}
}
