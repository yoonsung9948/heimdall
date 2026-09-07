package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yoonsung9948/heimdall/internal/matrix"
	"github.com/yoonsung9948/heimdall/internal/policy"
)

type ExplanationDTO struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
	RuleID string `json:"rule_id"`

	SubjectMatched  bool `json:"subject_matched"`
	ActionMatched   bool `json:"action_matched"`
	ResourceMatched bool `json:"resource_matched"`
}

type CellDTO struct {
	Client      string         `json:"client"`
	User        string         `json:"user"`
	Groups      []string       `json:"groups"`
	ServerName  string         `json:"server"`
	ToolName    string         `json:"tool"`
	Explanation ExplanationDTO `json:"explanation"`
}

type ChangeDTO struct {
	Client     string         `json:"client"`
	ServerName string         `json:"server"`
	ToolName   string         `json:"tool"`
	Kind       string         `json:"kind"` // "gained" | "lost" | "unchanged"
	Before     ExplanationDTO `json:"before"`
	After      ExplanationDTO `json:"after"`
}

type DecisionDTO struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
	RuleID string `json:"rule_id"`
}

func newDecisionDTO(dec policy.Decision) DecisionDTO {
	return DecisionDTO{
		Allow:  dec.Allow,
		Reason: dec.Reason,
		RuleID: dec.RuleID,
	}
}

func newExplanationDTO(exp policy.Explanation) ExplanationDTO {
	dto := ExplanationDTO{
		Allow:  exp.Decision.Allow,
		Reason: exp.Decision.Reason,
		RuleID: exp.Decision.RuleID,
	}
	if exp.Detail != nil {
		s := exp.Detail.Subjects
		dto.SubjectMatched = s.UserMatched || s.ClientMatched || s.GroupMatched
		dto.ActionMatched = exp.Detail.Action
		dto.ResourceMatched = exp.Detail.Resource.KindMatched && exp.Detail.Resource.NameMatched
	}
	return dto
}

func newCellDTO(cell matrix.Cell) CellDTO {
	return CellDTO{
		Client:      cell.Identity.Client,
		User:        cell.Identity.User,
		Groups:      cell.Identity.Groups,
		ServerName:  cell.ServerName,
		ToolName:    cell.Tool.Name,
		Explanation: newExplanationDTO(cell.Explanation),
	}
}

func newCellDTOs(cells []matrix.Cell) []CellDTO {
	var cellDTOs []CellDTO
	for _, cell := range cells {
		cellDTOs = append(cellDTOs, newCellDTO(cell))
	}
	return cellDTOs
}

func newChangeDTO(change matrix.Change) ChangeDTO {

	return ChangeDTO{
		Client:     change.Identity.Client,
		ServerName: change.ServerName,
		ToolName:   change.Tool.Name,
		Kind:       strings.ToLower(changeKindLabel(change.Kind)),
		Before:     newExplanationDTO(change.Before),
		After:      newExplanationDTO(change.After),
	}
}

func newChangeDTOs(changes []matrix.Change) []ChangeDTO {
	res := make([]ChangeDTO, 0, len(changes))
	for _, change := range changes {
		res = append(res, newChangeDTO(change))
	}
	return res
}

func writeJSON(w io.Writer, v any) error {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
