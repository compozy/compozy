package daemon

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionMCPReadModel struct {
	state     *bootState
	authority session.MCPAuthorityResolver
	registry  *toolspkg.RuntimeRegistry
	mu        sync.Mutex
	summaries map[string]session.MCPSummary
}

var _ session.MCPReader = (*sessionMCPReadModel)(nil)

func (r *sessionMCPReadModel) ListSessionMCP(ctx context.Context, id string) (session.SessionMCP, error) {
	ctx = toolspkg.WithProjectionListings(toolspkg.WithCachedMCPProjection(ctx), toolspkg.NewProjectionListings())
	info, err := r.state.sessions.Status(ctx, id)
	if err != nil {
		return session.SessionMCP{}, err
	}
	profile := daemonDefaultProfileName
	if r.state.profiles != nil {
		profile, err = r.state.profiles.ProfileName(ctx, info.ProfileID)
		if err != nil {
			return session.SessionMCP{}, err
		}
	}
	collection, err := r.state.deps.Settings.ListCollection(ctx, settingspkg.CollectionRequest{
		Collection: settingspkg.CollectionMCPServers, Scope: settingspkg.ScopeProfile,
		ProfileName: profile, WorkspaceID: info.WorkspaceID, SkipRuntimeStatus: true,
	})
	if err != nil {
		return session.SessionMCP{}, err
	}
	authority, err := r.authority.EffectiveMCPAuthority(ctx, id)
	if err != nil {
		return session.SessionMCP{}, err
	}
	scope := toolspkg.Scope{
		SessionID:   id,
		ProfileID:   info.ProfileID,
		WorkspaceID: info.WorkspaceID,
		AgentName:   info.AgentName,
	}
	callable, err := r.registry.SessionProjection(ctx, scope)
	if err != nil {
		return session.SessionMCP{}, err
	}
	advertised, err := r.registry.AdvertisedSessionProjection(ctx, scope)
	if err != nil {
		return session.SessionMCP{}, err
	}
	diagnostics, err := r.registry.DiagnosticProjection(ctx, scope)
	if err != nil {
		return session.SessionMCP{}, err
	}
	runtimeReader, ok := r.state.sessions.(interface {
		MCPReadSnapshot(context.Context, string, toolspkg.AdvertisedCatalog) (session.MCPRuntimeSnapshot, error)
	})
	if !ok {
		return session.SessionMCP{}, errors.New("daemon: session MCP runtime reader unavailable")
	}
	runtime, err := runtimeReader.MCPReadSnapshot(ctx, id, toolspkg.NewAdvertisedCatalog(advertised))
	if err != nil {
		return session.SessionMCP{}, err
	}
	return composeSessionMCP(id, collection.MCPServers, authority.MCPServers, callable, diagnostics, runtime), nil
}

func composeSessionMCP(
	id string,
	items []settingspkg.MCPServerItem,
	allowed []string,
	callable, diagnostics []toolspkg.ToolView,
	runtime session.MCPRuntimeSnapshot,
) session.SessionMCP {
	result := session.SessionMCP{SessionID: id, Runtime: runtime.Inventory.Runtime,
		RuntimeDisplayName: cmp.Or(runtime.Inventory.DisplayName, runtime.Inventory.Runtime), LastReload: runtime.LastReload,
		MCPSummary: session.MCPSummary{PendingReload: runtime.Pending}, CompozyServers: []session.MCPServerRow{},
		RuntimeServers: session.SessionRuntimeMCP{RuntimeMCP: runtime.Mode, Servers: []session.RuntimeMCPRow{}},
	}
	for _, item := range items {
		row := sessionMCPRow(item, allowed, callable, diagnostics, runtime)
		result.CompozyServers = append(result.CompozyServers, row)
		switch row.State {
		case "ready", "applies_next_message":
			result.Usable++
		case "waiting_approval", "needs_sign_in", "failed":
			result.Attention++
		}
	}
	slices.SortFunc(result.CompozyServers, func(a, b session.MCPServerRow) int { return strings.Compare(a.Name, b.Name) })
	for _, server := range runtime.Inventory.Servers {
		state := "loaded_by_runtime"
		if runtime.Mode == "off" && runtime.Inventory.RuntimeMCP.CanDisable {
			state = "off_in_compozy_sessions"
		}
		result.RuntimeServers.Servers = append(
			result.RuntimeServers.Servers,
			session.RuntimeMCPRow{Name: server.Name, Scope: server.Scope, Transport: server.Transport, State: state},
		)
		if state == "loaded_by_runtime" && server.EnabledInRuntime && server.Approval != "pending" &&
			server.Approval != "rejected" {
			result.Usable++
		}
	}
	return result
}

func sessionMCPRow(
	item settingspkg.MCPServerItem,
	allowed []string,
	callable, diagnostics []toolspkg.ToolView,
	runtime session.MCPRuntimeSnapshot,
) session.MCPServerRow {
	row := session.MCPServerRow{Name: item.Name, Owner: item.Owner, State: "ready"}
	name := item.RuntimeName
	if name == "" {
		name = item.Name
	}
	for _, view := range diagnostics {
		if view.Descriptor.Source.Kind == toolspkg.SourceMCP && view.Descriptor.Source.RawServerName == name {
			row.ToolCount++
		}
	}
	for _, view := range callable {
		if view.Descriptor.Source.Kind == toolspkg.SourceMCP && view.Descriptor.Source.RawServerName == name {
			row.CallableCount++
		}
	}
	switch {
	case item.Approval.State == "waiting":
		row.State, row.Reason = "waiting_approval", "Project server waiting for approval"
	case !slices.Contains(allowed, item.Name):
		row.State, row.Reason = "not_available", "Limited by the parent session"
	case item.AuthStatus != nil && item.AuthStatus.Status != "" && item.AuthStatus.Status != "authenticated" && item.AuthStatus.Status != "unconfigured":
		row.State, row.Reason = "needs_sign_in", "Sign in to "+item.Name+" in CompozyOS"
	case row.CallableCount == 0:
		row.State, row.Reason = "failed", "No callable tools available"
	case runtime.Pending && (runtime.PendingServers[name] || runtime.BoundServers[name] != row.CallableCount):
		row.State = "applies_next_message"
	}
	return row
}

// refresh debounces summary notifications at the declaration watcher's existing cadence.
func (r *sessionMCPReadModel) refresh(ctx context.Context) error {
	manager, ok := r.state.sessions.(interface {
		BoundMCPSessionIDs() []string
		RecordMCPChanged(context.Context, string, session.MCPSummary) error
	})
	if !ok {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	active := map[string]bool{}
	var errs []error
	for _, id := range manager.BoundMCPSessionIDs() {
		active[id] = true
		result, err := r.ListSessionMCP(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		previous, exists := r.summaries[id]
		if exists && previous == result.MCPSummary {
			continue
		}
		if err := manager.RecordMCPChanged(ctx, id, result.MCPSummary); err != nil {
			errs = append(errs, err)
			continue
		}
		r.summaries[id] = result.MCPSummary
	}
	for id := range r.summaries {
		if !active[id] {
			delete(r.summaries, id)
		}
	}
	return errors.Join(errs...)
}

func bindSessionMCPReader(
	state *bootState,
	authority session.MCPAuthorityResolver,
	registry *toolspkg.RuntimeRegistry,
) {
	reader := &sessionMCPReadModel{
		state:     state,
		authority: authority,
		registry:  registry,
		summaries: map[string]session.MCPSummary{},
	}
	state.sessionMCPReadModel = reader
	if manager, ok := state.sessions.(interface{ SetMCPReader(session.MCPReader) }); ok {
		manager.SetMCPReader(reader)
	}
}
