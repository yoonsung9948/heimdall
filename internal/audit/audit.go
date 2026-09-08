package audit

import (
	"time"

	"github.com/yoonsung9948/heimdall/internal/policy"
)

type AuditEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	UserName       string    `json:"user_name"`
	ClientName     string    `json:"client_name"`
	ToolName       string    `json:"tool_name"`
	ServerName     string    `json:"server_name"`
	Groups         []string  `json:"groups"`
	DecisionAllow  bool      `json:"decision_allow"`
	DecisionReason string    `json:"decision_reason"`
	RuleID         string    `json:"rule_id"`
}

func NewAuditEvent(req policy.Request, serverName string, decision policy.Decision, timestamp time.Time) AuditEvent {
	return AuditEvent{
		Timestamp:      timestamp,
		UserName:       req.Identity.User,
		ClientName:     req.Identity.Client,
		ToolName:       req.Resource.Name,
		ServerName:     serverName,
		Groups:         req.Identity.Groups,
		DecisionAllow:  decision.Allow,
		DecisionReason: decision.Reason,
		RuleID:         decision.RuleID,
	}
}
