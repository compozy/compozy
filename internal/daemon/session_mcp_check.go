package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
)

var _ session.MCPChecker = (*sessionMCPReadModel)(nil)

func (r *sessionMCPReadModel) CheckSessionMCP(ctx context.Context, id, name, owner string) (session.MCP, error) {
	current, err := r.ListSessionMCP(ctx, id)
	if err != nil {
		return session.MCP{}, err
	}
	name, owner = strings.TrimSpace(name), vault.NormalizeMCPOwner(owner)
	index := slices.IndexFunc(current.CompozyServers, func(row session.MCPServerRow) bool {
		return row.Name == name && vault.NormalizeMCPOwner(row.Owner) == owner
	})
	if index < 0 {
		return session.MCP{}, fmt.Errorf("%w: MCP server %q", settingspkg.ErrNotFound, name)
	}
	row := current.CompozyServers[index]
	if row.State == mcpWaitingApproval || row.State == "not_available" {
		return current, nil
	}
	checker, ok := r.state.mcpToolProvider.(interface {
		CheckMCPServer(context.Context, toolspkg.Scope, string) error
	})
	if !ok {
		return session.MCP{}, errors.New("daemon: MCP check unavailable")
	}
	info, err := r.state.sessions.Status(ctx, id)
	if err != nil {
		return session.MCP{}, err
	}
	collection, err := r.sessionMCPCollection(ctx, info)
	if err != nil {
		return session.MCP{}, err
	}
	for i := range collection.MCPServers {
		item := &collection.MCPServers[i]
		if item.Name != name || vault.NormalizeMCPOwner(item.Owner) != owner {
			continue
		}
		scope := toolspkg.Scope{
			SessionID:   id,
			ProfileID:   info.ProfileID,
			WorkspaceID: info.WorkspaceID,
			AgentName:   info.AgentName,
		}
		if err := checker.CheckMCPServer(ctx, scope, cmp.Or(item.RuntimeName, item.Name)); err != nil {
			return session.MCP{}, err
		}
		return r.ListSessionMCP(ctx, id)
	}
	return session.MCP{}, fmt.Errorf("%w: MCP server %q", settingspkg.ErrNotFound, name)
}
