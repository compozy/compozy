package daemon

import (
	"cmp"
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"sync"

	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/runtimemcp"
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
	snapshots map[string]sessionMCPSummarySnapshot
	scopes    map[string]sessionMCPScopeSnapshot
	refreshMu sync.Mutex
}

var _ session.MCPReader = (*sessionMCPReadModel)(nil)

func (r *sessionMCPReadModel) ListSessionMCP(ctx context.Context, id string) (session.MCP, error) {
	ctx = toolspkg.EnsureProjectionListings(toolspkg.WithCachedMCPProjection(ctx))
	ctx = r.registry.ReadContext(toolspkg.WithImmutableProjection(ctx))
	info, err := storedSessionStatus(ctx, r.state.sessions, id)
	if err != nil {
		return session.MCP{}, err
	}
	ctx = session.WithMCPAuthoritySession(ctx, info)
	collection, err := r.sessionMCPCollection(ctx, info)
	if err != nil {
		return session.MCP{}, err
	}
	authorityCtx := ctx
	if reader, ok := r.authority.(interface {
		MCPAuthorityReadContext(context.Context) context.Context
	}); ok {
		authorityCtx = reader.MCPAuthorityReadContext(ctx)
	}
	authority, err := r.authority.EffectiveMCPAuthority(authorityCtx, id)
	if err != nil {
		return session.MCP{}, err
	}
	scope := toolspkg.Scope{
		SessionID:   id,
		ProfileID:   info.ProfileID,
		WorkspaceID: info.WorkspaceID,
		AgentName:   info.AgentName,
	}
	callable, err := r.registry.SessionProjection(ctx, scope)
	if err != nil {
		return session.MCP{}, err
	}
	advertised, err := r.registry.AdvertisedSessionProjection(ctx, scope)
	if err != nil {
		return session.MCP{}, err
	}
	diagnostics, err := r.registry.DiagnosticProjection(ctx, scope)
	if err != nil {
		return session.MCP{}, err
	}
	runtimeReader, ok := r.state.sessions.(interface {
		MCPReadSnapshot(context.Context, string, toolspkg.AdvertisedCatalog) (session.MCPRuntimeSnapshot, error)
	})
	if !ok {
		return session.MCP{}, errors.New("daemon: session MCP runtime reader unavailable")
	}
	runtime, err := runtimeReader.MCPReadSnapshot(ctx, id, toolspkg.NewAdvertisedCatalog(advertised))
	if err != nil {
		return session.MCP{}, err
	}
	return composeSessionMCP(id, collection.MCPServers, authority.MCPServers, callable, diagnostics, runtime), nil
}

func composeSessionMCP(
	id string,
	items []settingspkg.MCPServerItem,
	allowed []string,
	callable, diagnostics []toolspkg.ToolView,
	runtime session.MCPRuntimeSnapshot,
) session.MCP {
	result := session.MCP{
		SessionID: id,
		Runtime:   runtime.Inventory.Runtime,
		RuntimeDisplayName: cmp.Or(
			runtime.Inventory.DisplayName,
			runtime.Inventory.Runtime,
		),
		LastReload:     runtime.LastReload,
		PendingReload:  runtime.Pending,
		CompozyServers: []session.MCPServerRow{},
		RuntimeServers: session.RuntimeMCP{RuntimeMCP: runtime.Mode, Servers: []session.RuntimeMCPRow{}},
	}
	for i := range items {
		item := &items[i]
		row := sessionMCPRow(item, allowed, callable, diagnostics, runtime)
		result.CompozyServers = append(result.CompozyServers, row)
		switch row.State {
		case "ready", "applies_next_message":
			result.Usable++
		case mcpWaitingApproval, mcpNeedsSignIn, hookAgentEventsFailedKey:
			result.Attention++
		}
	}
	slices.SortFunc(
		result.CompozyServers,
		func(a, b session.MCPServerRow) int { return strings.Compare(a.Name, b.Name) },
	)
	for i := range runtime.Inventory.Servers {
		server := &runtime.Inventory.Servers[i]
		state := runtimeMCPRowState(server, runtime.Mode == "off" && runtime.Inventory.RuntimeMCP.CanDisable)
		result.RuntimeServers.Servers = append(
			result.RuntimeServers.Servers,
			session.RuntimeMCPRow{Name: server.Name, Scope: server.Scope, Transport: server.Transport, State: state},
		)
		if state == runtimeMCPLoaded {
			result.Usable++
		}
	}
	return result
}

const (
	runtimeMCPLoaded           = "loaded_by_runtime"
	runtimeMCPOffInCompozy     = "off_in_compozy_sessions"
	runtimeMCPOffInRuntime     = "off_in_runtime"
	runtimeMCPWaitingInRuntime = "waiting_in_runtime"
	runtimeMCPApprovalPending  = "pending"
	runtimeMCPApprovalRejected = "rejected"
)

// runtimeMCPRowState keeps the runtime's own enablement and approval visible: a server the runtime
// will not load is never shown as loaded.
func runtimeMCPRowState(server *runtimemcp.Server, offInCompozy bool) string {
	switch {
	case offInCompozy:
		return runtimeMCPOffInCompozy
	case !server.EnabledInRuntime || server.Approval == runtimeMCPApprovalRejected:
		return runtimeMCPOffInRuntime
	case server.Approval == runtimeMCPApprovalPending:
		return runtimeMCPWaitingInRuntime
	default:
		return runtimeMCPLoaded
	}
}

func sessionMCPRow(
	item *settingspkg.MCPServerItem,
	allowed []string,
	callable, diagnostics []toolspkg.ToolView,
	runtime session.MCPRuntimeSnapshot,
) session.MCPServerRow {
	source := item.SourceMetadata.EffectiveSource
	row := session.MCPServerRow{
		Name: item.Name, Owner: item.Owner, State: "ready",
		Scope: string(source.Scope), WorkspaceID: source.WorkspaceID, Profile: source.ProfileName,
		Transport: strings.TrimSpace(string(item.Transport)),
		Source:    session.MCPServerSource{Kind: string(source.Kind)},
	}
	if row.Scope == "" {
		row.Scope, row.WorkspaceID, row.Profile = string(item.Scope), item.WorkspaceID, item.ProfileName
	}
	if item.Approval.State == mcpApprovalWaiting {
		row.Approval = &session.MCPServerApproval{
			Reason:           item.Approval.Reason,
			DefinitionDigest: item.Approval.DefinitionDigest,
		}
		// Consent belongs to the listing profile even when the definition is inherited.
		row.Scope, row.WorkspaceID, row.Profile = string(item.Scope), item.WorkspaceID, item.ProfileName
	}
	name := item.RuntimeName
	if name == "" {
		name = item.Name
	}
	for i := range diagnostics {
		view := &diagnostics[i]
		if view.Descriptor.Source.Kind == toolspkg.SourceMCP && view.Descriptor.Source.RawServerName == name {
			row.ToolCount++
		}
	}
	for i := range callable {
		view := &callable[i]
		if view.Descriptor.Source.Kind == toolspkg.SourceMCP && view.Descriptor.Source.RawServerName == name {
			row.CallableCount++
		}
	}
	switch {
	case item.Approval.State == mcpApprovalWaiting:
		row.State, row.Reason = mcpWaitingApproval, "Project server waiting for approval"
		setSessionMCPPreview(&row, item)
	case !slices.Contains(allowed, item.Name):
		row.State, row.Reason = "not_available", "Limited by the parent session"
	case item.AuthStatus != nil && item.AuthStatus.Status != "" &&
		item.AuthStatus.Status != "authenticated" && item.AuthStatus.Status != "unconfigured":
		row.State, row.Reason = mcpNeedsSignIn, "Sign in to "+item.Name+" in CompozyOS"
	case row.CallableCount == 0:
		row.State, row.Reason = hookAgentEventsFailedKey, "No callable tools available"
	case runtime.Pending && (runtime.PendingServers[name] || runtime.BoundServers[name] != row.CallableCount):
		row.State = "applies_next_message"
	}
	return row
}

func setSessionMCPPreview(row *session.MCPServerRow, item *settingspkg.MCPServerItem) {
	row.Command = diagnosticspkg.Redact(item.Command)
	row.Args = redact.Args(item.Args)
	row.Source.Path = diagnosticspkg.Redact(item.DefinitionSource)
	if endpoint, err := url.Parse(item.URL); err == nil {
		endpoint.User = nil
		query := endpoint.Query()
		for key := range query {
			query.Set(key, "[redacted]")
		}
		endpoint.RawQuery = query.Encode()
		endpoint.Fragment = ""
		row.URL = diagnosticspkg.Redact(endpoint.String())
	}
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

func (r *sessionMCPReadModel) sessionMCPCollection(
	ctx context.Context,
	info *session.Info,
) (settingspkg.CollectionEnvelope, error) {
	var err error
	profile := daemonDefaultProfileName
	if r.state.profiles != nil {
		profile, err = r.state.profiles.ProfileName(ctx, info.ProfileID)
		if err != nil {
			return settingspkg.CollectionEnvelope{}, err
		}
	}
	request := settingspkg.CollectionRequest{
		Collection: settingspkg.CollectionMCPServers, Scope: settingspkg.ScopeUser,
		WorkspaceID: info.WorkspaceID, SkipRuntimeStatus: true,
	}
	if info.WorkspaceID != "" {
		request.Scope = settingspkg.ScopeWorkspace
	}
	if profile != "" && profile != daemonDefaultProfileName {
		request.Scope, request.ProfileName = settingspkg.ScopeProfile, profile
	}
	return r.state.deps.Settings.ListCollection(ctx, request)
}
