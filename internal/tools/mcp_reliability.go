package tools

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/store"
)

// MCPDeadEntityService is the reliability seam required by the MCP registry adapter.
type MCPDeadEntityService interface {
	BeforeProbe(ctx context.Context, key store.DeadEntityKey) (deadentity.ProbeDecision, error)
	Status(ctx context.Context, key store.DeadEntityKey) (deadentity.Status, error)
	RecordFailure(
		ctx context.Context,
		key store.DeadEntityKey,
		class deadentity.FailureClass,
		reason string,
	) error
	RecordSuccess(ctx context.Context, key store.DeadEntityKey) error
	ForgetEntity(key store.DeadEntityKey)
}

// MCPProviderOption configures MCP discovery behavior.
type MCPProviderOption func(*MCPProvider)

// WithMCPDeadEntityService enables workspace-scoped durable MCP recovery.
func WithMCPDeadEntityService(service MCPDeadEntityService) MCPProviderOption {
	return func(provider *MCPProvider) {
		if provider == nil || isNilInterface(service) {
			return
		}
		provider.reliability = &mcpReliability{
			service: service,
			states:  make(map[store.DeadEntityKey]*mcpReliabilityState),
		}
	}
}

func WithMCPConsent(check func(context.Context, SourceRef, string) (bool, error)) MCPProviderOption {
	return func(p *MCPProvider) { p.consent = check }
}

type mcpReliability struct {
	service MCPDeadEntityService

	mu     sync.RWMutex
	states map[store.DeadEntityKey]*mcpReliabilityState
}

type mcpReliabilityState struct {
	descriptors []Descriptor
}

func (p *MCPProvider) listSourceDescriptors(
	ctx context.Context,
	source SourceRef,
	profileID string,
	timeout time.Duration,
) ([]Descriptor, bool, error) {
	if p.consent != nil {
		admitted, err := p.consent(ctx, source, profileID)
		if err != nil {
			return nil, false, err
		}
		if !admitted {
			return nil, true, nil
		}
	}
	if cachedMCPProjection(ctx) {
		return p.cachedSourceDescriptors(source), true, nil
	}
	key, tracked := mcpDeadEntityKey(source, profileID)
	var reliabilityState *mcpReliabilityState
	if tracked && p.reliability != nil {
		reliabilityState = p.reliability.stateFor(key)
		decision, err := p.reliability.service.BeforeProbe(ctx, key)
		if err != nil {
			return nil, false, fmt.Errorf("tools: admit mcp source %q: %w", source.Owner, err)
		}
		if !decision.Allowed {
			if !p.canRecoverAuthFailure(ctx, source, decision.Reason) {
				if p.catalog != nil {
					return p.cachedSourceDescriptors(source), true, nil
				}
				return p.reliability.cached(key, reliabilityState), true, nil
			}
		}
	}

	generation := p.nextDiscoveryGeneration(source, FreshMCPDiscovery(ctx))
	discovered, err := p.discoverSource(ctx, source, timeout)
	p.discoveryMu.Lock()
	defer p.discoveryMu.Unlock()
	current := p.discoveryGenerations[source]
	if current.check != generation.check || (!FreshMCPDiscovery(ctx) && current.list != generation.list) {
		return p.cachedSourceDescriptors(source), true, nil
	}
	if FreshMCPDiscovery(ctx) {
		current.list++
		p.discoveryGenerations[source] = current
	}
	if err != nil {
		return p.discoveryFailed(ctx, source, key, tracked, reliabilityState, err)
	}
	return p.discoverySucceeded(ctx, source, key, tracked, reliabilityState, discovered)
}

func (p *MCPProvider) canRecoverAuthFailure(ctx context.Context, source SourceRef, reason string) bool {
	if isNilInterface(p.auth) || !MCPAuthenticationFailure(reason) {
		return false
	}
	status, err := p.auth.Status(ctx, source, "")
	if err != nil || !status.TokenPresent {
		return false
	}
	return status.Status == mcpAuthStatusAuthenticated ||
		(status.Status == mcpAuthStatusExpired && status.Refreshable)
}

// MCPAuthenticationFailure recognizes persisted auth diagnostics from earlier releases.
func MCPAuthenticationFailure(reason string) bool {
	reason = strings.ToLower(reason)
	return strings.Contains(reason, "mcp_auth_") || strings.Contains(reason, "mcp auth") ||
		strings.Contains(reason, "needs sign-in") || strings.Contains(reason, "token is expired") ||
		strings.Contains(reason, "login required")
}

type mcpDiscoveryGeneration struct{ check, list uint64 }

func (p *MCPProvider) nextDiscoveryGeneration(source SourceRef, fresh bool) mcpDiscoveryGeneration {
	p.discoveryMu.Lock()
	defer p.discoveryMu.Unlock()
	if p.discoveryGenerations == nil {
		p.discoveryGenerations = make(map[SourceRef]mcpDiscoveryGeneration)
	}
	generation := p.discoveryGenerations[source]
	if fresh {
		generation.check++
	}
	generation.list++
	p.discoveryGenerations[source] = generation
	return p.discoveryGenerations[source]
}

// discoveryFailed records a failed discovery and returns the last known descriptors. Callers hold
// discoveryMu.
func (p *MCPProvider) discoveryFailed(
	ctx context.Context,
	source SourceRef,
	key store.DeadEntityKey,
	tracked bool,
	reliabilityState *mcpReliabilityState,
	err error,
) ([]Descriptor, bool, error) {
	// Abort aggregate discovery only when its caller stopped. A timeout or cancellation
	// from one executor is a source failure and must not hide healthy MCP servers.
	if contextError := contextErrFromError("", ctx.Err()); contextError != nil {
		return nil, false, contextError
	}
	if deferred, ok := errors.AsType[*MCPCatalogPollDeferred](err); ok {
		if deferred.Reason != "" {
			p.recordDiscoveryReason(ctx, source, deferred.Reason)
		}
		return p.cachedSourceDescriptors(source), false, nil
	}
	p.recordDiscoveryFailure(ctx, source, err)
	if tracked && p.reliability != nil {
		if recordErr := p.reliability.service.RecordFailure(
			ctx,
			key,
			MCPDiscoveryFailureClass(err),
			err.Error(),
		); recordErr != nil {
			return nil, false, fmt.Errorf("tools: record mcp source %q failure: %w", source.Owner, recordErr)
		}
		if p.catalog == nil {
			return p.reliability.cached(key, reliabilityState), false, nil
		}
	}
	return p.cachedSourceDescriptors(source), false, nil
}

func (p *MCPProvider) recordDiscoveryFailure(ctx context.Context, source SourceRef, err error) {
	reason, _ := ReasonOf(err)
	if reason == "" {
		reason = ReasonBackendUnhealthy
	}
	p.recordDiscoveryReason(ctx, source, reason)
}

func (p *MCPProvider) recordDiscoveryReason(ctx context.Context, source SourceRef, reason ReasonCode) {
	previous, _ := p.discoveryFailures.Swap(source, reason)
	if p.catalog != nil && previous != reason {
		p.catalog.sourceChanged(ctx, source, reason)
	}
}

// discoverySucceeded stores the discovered descriptors and records recovery. Callers hold
// discoveryMu.
func (p *MCPProvider) discoverySucceeded(
	ctx context.Context,
	source SourceRef,
	key store.DeadEntityKey,
	tracked bool,
	reliabilityState *mcpReliabilityState,
	discovered []MCPToolDescriptor,
) ([]Descriptor, bool, error) {
	before, hadDiscovery := p.discovered.Load(source)
	_, wasFailed := p.discoveryFailures.Load(source)
	descriptors, err := p.normalizeDiscovery(source, discovered)
	if err != nil {
		return nil, false, err
	}
	p.discoveryFailures.Delete(source)
	if tracked && p.reliability != nil {
		if err := p.reliability.service.RecordSuccess(ctx, key); err != nil {
			return nil, false, fmt.Errorf("tools: record mcp source %q recovery: %w", source.Owner, err)
		}
		if p.catalog == nil {
			p.reliability.store(key, reliabilityState, descriptors)
		}
	}
	after, _ := p.discovered.Load(source)
	if p.catalog != nil && (!hadDiscovery || before != after || wasFailed) {
		p.catalog.sourceChanged(ctx, source, "")
	}
	return descriptors, true, nil
}

// ForgetMCPServer retires one workspace MCP server's volatile reliability state.
func (p *MCPProvider) ForgetMCPServer(workspaceID string, serverName string) {
	if p == nil {
		return
	}
	p.forgetDiscovered(func(source SourceRef) bool {
		return source.WorkspaceID == workspaceID && mcpSourceServerName(source) == serverName
	})
	if p.reliability != nil {
		p.reliability.forgetSource(workspaceID, serverName)
	}
}

// ForgetWorkspace retires cached MCP descriptors owned by one workspace.
func (p *MCPProvider) ForgetWorkspace(workspaceID string) {
	if p == nil {
		return
	}
	p.forgetDiscovered(func(source SourceRef) bool { return source.WorkspaceID == workspaceID })
	if p.reliability != nil {
		p.reliability.forgetWorkspace(workspaceID)
	}
}

func (r *mcpReliability) dead(ctx context.Context, source SourceRef, profileID string) bool {
	if r == nil || isNilInterface(r.service) {
		return false
	}
	key, tracked := mcpDeadEntityKey(source, profileID)
	if !tracked {
		return false
	}
	status, err := r.service.Status(ctx, key)
	return err == nil && status.Dead
}

func (r *mcpReliability) stateFor(key store.DeadEntityKey) *mcpReliabilityState {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.states[key]
	if state == nil {
		state = &mcpReliabilityState{}
		r.states[key] = state
	}
	return state
}

func (r *mcpReliability) cached(
	key store.DeadEntityKey,
	state *mcpReliabilityState,
) []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.states[key] != state {
		return nil
	}
	descriptors := state.descriptors
	cloned := make([]Descriptor, len(descriptors))
	for i := range descriptors {
		cloned[i] = cloneDescriptor(descriptors[i])
	}
	return cloned
}

func (r *mcpReliability) store(
	key store.DeadEntityKey,
	state *mcpReliabilityState,
	descriptors []Descriptor,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.states[key] != state {
		return
	}
	cloned := make([]Descriptor, len(descriptors))
	for i := range descriptors {
		cloned[i] = cloneDescriptor(descriptors[i])
	}
	state.descriptors = cloned
}

func (r *mcpReliability) forgetSource(workspaceID string, serverName string) {
	workspaceID = strings.TrimSpace(workspaceID)
	serverName = strings.TrimSpace(serverName)
	if workspaceID == "" || serverName == "" {
		return
	}
	retired := make([]store.DeadEntityKey, 0)
	r.mu.Lock()
	for key := range r.states {
		if key.WorkspaceID != workspaceID ||
			key.Kind != store.DeadEntityKindMCPSidecar ||
			key.EntityID != serverName {
			continue
		}
		delete(r.states, key)
		retired = append(retired, key)
	}
	r.mu.Unlock()
	for _, key := range retired {
		r.service.ForgetEntity(key)
	}
}

func (r *mcpReliability) forgetWorkspace(workspaceID string) {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	maps.DeleteFunc(r.states, func(key store.DeadEntityKey, _ *mcpReliabilityState) bool {
		return key.WorkspaceID == trimmed
	})
}

func (r *mcpReliability) retainActiveSources(scope Scope, sources []SourceRef) {
	if r == nil {
		return
	}
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		return
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	active := make(map[store.DeadEntityKey]struct{}, len(sources))
	for _, source := range sources {
		if key, tracked := mcpDeadEntityKey(source, scope.ProfileID); tracked {
			active[key] = struct{}{}
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.states {
		if key.WorkspaceID != workspaceID || key.ProfileID != profileID {
			continue
		}
		if _, ok := active[key]; !ok {
			delete(r.states, key)
		}
	}
}

func mcpDeadEntityKey(source SourceRef, profileID string) (store.DeadEntityKey, bool) {
	if source.WorkspaceID == "" {
		return store.DeadEntityKey{}, false
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	key := store.DeadEntityKey{
		ProfileID:   profileID,
		WorkspaceID: source.WorkspaceID,
		Kind:        store.DeadEntityKindMCPSidecar,
		EntityID:    firstNonEmpty(source.RawServerName, source.Owner),
	}.Normalize()
	return key, key.Validate() == nil
}

// MCPDiscoveryFailureClass maps structured MCP errors into reliability signals.
func MCPDiscoveryFailureClass(err error) deadentity.FailureClass {
	reason, ok := ReasonOf(err)
	if !ok {
		return deadentity.FailureTransient
	}
	switch reason {
	case ReasonDependencyMissing,
		ReasonMCPUnreachable,
		ReasonMCPAuthUnconfigured,
		ReasonMCPAuthRequired,
		ReasonMCPAuthExpired,
		ReasonMCPAuthInvalid,
		ReasonSourceDisabled:
		return deadentity.FailurePermanent
	default:
		return deadentity.FailureTransient
	}
}
