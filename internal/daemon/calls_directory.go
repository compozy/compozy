package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	callspkg "github.com/compozy/compozy/internal/calls"
)

type daemonCallDirectory struct {
	store callspkg.Store
	state *bootState
}

func (d *daemonCallDirectory) ResolveCallTarget(
	ctx context.Context,
	input callspkg.CreateInput,
) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
	target, err := d.store.ResolveCallTargetContext(ctx, input)
	if err != nil {
		return callspkg.TargetContext{}, nil, fmt.Errorf("daemon: resolve call target context: %w", err)
	}
	catalog := agentCatalogDependency(d.state.agentCatalog)
	if catalog == nil {
		return target, nil, nil
	}
	var entries []core.AgentCatalogEntry
	profileName, profileErr := callRosterProfileName(ctx, d.state, input.ProfileID)
	if profileErr != nil {
		return callspkg.TargetContext{}, nil, profileErr
	}
	if input.Scope == callspkg.ScopeWorkspace && d.state.workspaceResolver != nil {
		resolved, resolveErr := d.state.workspaceResolver.ResolveForProfile(ctx, input.WorkspaceID, profileName)
		if resolveErr != nil {
			return callspkg.TargetContext{}, nil, fmt.Errorf("daemon: resolve call workspace: %w", resolveErr)
		}
		entries, err = catalog.ListAgentsForWorkspace(ctx, &resolved)
	} else {
		entries, err = catalog.ListAgentsForProfile(ctx, input.ProfileID, profileName)
	}
	if err != nil {
		return callspkg.TargetContext{}, nil, fmt.Errorf("daemon: list call agent roster: %w", err)
	}
	roster := make([]callspkg.AgentRosterEntry, 0, len(entries))
	requested := strings.TrimSpace(input.Target.Agent)
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Def.Name)
		roster = append(roster, callspkg.AgentRosterEntry{
			Name: name, Description: strings.TrimSpace(entry.Def.Description),
		})
		if name == requested {
			target.AgentName = name
		}
	}
	return target, roster, nil
}

var _ callspkg.Directory = (*daemonCallDirectory)(nil)
