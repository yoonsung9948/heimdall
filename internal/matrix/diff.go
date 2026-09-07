package matrix

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yoonsung9948/heimdall/internal/policy"
	"github.com/yoonsung9948/heimdall/internal/types"
)

type cellKey struct {
	Client   string
	ToolName string
}

type ChangeKind int

const (
	Unchanged ChangeKind = iota
	Gained
	Lost
)

type Change struct {
	Identity   types.Identity
	Tool       *mcp.Tool
	ServerName string
	Kind       ChangeKind
	Before     policy.Explanation
	After      policy.Explanation
}

func Diff(before, after []Cell) ([]Change, error) {
	// build map[cellKey]Cell from before
	// loop after, look up by key — handle the "not found" (ok == false)
	// case as a real error, don't let it silently zero-value through
	// classify Allow-before vs Allow-after into Gained/Lost/Unchanged
	var res []Change
	beforeByKey := make(map[cellKey]Cell, len(before))
	for _, c := range before {
		beforeByKey[cellKey{c.Identity.Client, c.Tool.Name}] = c
	}
	for _, ac := range after {
		key := cellKey{ac.Identity.Client, ac.Tool.Name}
		bc, ok := beforeByKey[key]
		if !ok {
			return nil, fmt.Errorf("diff: client %q tool %q present in after but not before — before/after were built from different identities/tools", ac.Identity.Client, ac.Tool.Name)
		}
		delete(beforeByKey, key)
		kind := Unchanged
		afterAllow, beforeAllow := ac.Explanation.Decision.Allow, bc.Explanation.Decision.Allow
		if !beforeAllow && afterAllow {
			kind = Gained
		} else if beforeAllow && !afterAllow {
			kind = Lost
		}

		res = append(res, Change{
			Identity:   ac.Identity,
			Tool:       ac.Tool,
			ServerName: ac.ServerName,
			Kind:       kind,
			Before:     bc.Explanation,
			After:      ac.Explanation,
		})
	}

	if len(beforeByKey) > 0 {
		for key := range beforeByKey {
			return nil, fmt.Errorf("diff: client %q tool %q present in before but not after — before/after were built from different identities/tools", key.Client, key.ToolName)
		}
	}

	return res, nil
}
