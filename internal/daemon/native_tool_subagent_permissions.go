package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

const subagentToolPermission = "tool"

func (n *daemonNativeTools) validateSubagentPermissions(ctx context.Context, req session.SubagentRequest) error {
	parent, err := n.deps.Sessions.Status(ctx, req.Caller.SessionID)
	if err != nil {
		return err
	}
	if parent == nil {
		return session.ErrSubagentParentNotActive
	}
	denied := func(message string) error {
		return &session.SubagentError{
			Code:    "permission_escalation_denied",
			Message: "Subagent cannot widen permissions: " + message,
			Err:     session.ErrSpawnPermissionDenied,
		}
	}
	if req.PermissionMode != "" &&
		subagentPermissionRank(string(req.PermissionMode)) > subagentPermissionRank(parent.EffectivePermissions) {
		return denied("permission_mode is broader than the caller's mode.")
	}
	if req.Narrowing == nil {
		return nil
	}
	budget := store.SessionPermissionPolicy{}
	if parent.Lineage != nil {
		budget = parent.Lineage.PermissionPolicy
	}
	for _, category := range []struct {
		name          string
		parent, child []string
	}{
		{subagentToolPermission, budget.Tools, req.Narrowing.Tools}, {"skill", budget.Skills, req.Narrowing.Skills},
		{"MCP server", budget.MCPServers, req.Narrowing.MCPServers},
		{"workspace path", budget.WorkspacePaths, req.Narrowing.WorkspacePaths},
	} {
		for _, atom := range category.child {
			atom = strings.TrimSpace(atom)
			if !slices.Contains(category.parent, atom) {
				return denied(fmt.Sprintf("%s %s is outside the caller's budget.", category.name, atom))
			}
		}
	}
	return nil
}

func subagentPermissionRank(mode string) int {
	switch mode {
	case "approve-all":
		return 2
	case "approve-reads":
		return 1
	default:
		return 0
	}
}
