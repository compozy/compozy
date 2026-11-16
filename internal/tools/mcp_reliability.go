package tools

import (
	"context"
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
			// A settled breaker decision is reusable; failed discovery attempts are not.
			return p.reliability.cached(key, reliabilityState), true, nil
		}
	}

	generation := p.nextDiscoveryGeneration(source)
	discovered, err := p.discoverSource(ctx, source, timeout)
	p.discoveryMu.Lock()
	defer p.discoveryMu.Unlock()
	if p.discoveryGenerations[source] != generation {
		return p.cachedSourceDescriptors(source), true, nil
	}
	if err != nil {
		return p.discoveryFailed(ctx, source, key, tracked, reliabilityState, err)
	}
	return p.discoverySucceeded(ctx, source, key, tracked, reliabilityState, discovered)
}

// nextDiscoveryGeneration starts one discovery of source; a result whose generation is no longer
// current was overtaken by a newer discovery or check and must not replace it.
func (p *MCPProvider) nextDiscoveryGeneration(source SourceRef) uint64 {
	p.discoveryMu.Lock()
	defer p.discoveryMu.Unlock()
	if p.discoveryGenerations == nil {
		p.discoveryGenerations = make(map[SourceRef]uint64)
	}
	p.discoveryGenerations[source]++
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
		p.discoveryFailures.Store(source, struct{}{})
		return nil, false, contextError
	}
	p.discoveryFailures.Store(source, struct{}{})
	if tracked && p.reliability != nil {
		if recordErr := p.reliability.service.RecordFailure(
			ctx,
			key,
			MCPDiscoveryFailureClass(err),
			err.Error(),
		); recordErr != nil {
			return nil, false, fmt.Errorf("tools: record mcp source %q failure: %w", source.Owner, recordErr)
		}
		return p.reliability.cached(key, reliabilityState), false, nil
	}
	return p.cachedSourceDescriptors(source), false, nil
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
	descriptors := make([]Descriptor, 0, len(discovered))
	for i := range discovered {
		descriptor, err := mcpRegistryDescriptor(source, discovered[i])
		if err != nil {
			return nil, false, wrapField(err, fmt.Sprintf("mcp_tools[%d]", i))
		}
		descriptors = append(descriptors, descriptor)
	}
	p.discoveryFailures.Delete(source)
	p.discovered.Store(source, cloneDescriptors(descriptors))
	if tracked && p.reliability != nil {
		if err := p.reliability.service.RecordSuccess(ctx, key); err != nil {
			return nil, false, fmt.Errorf("tools: record mcp source %q recovery: %w", source.Owner, err)
		}
		p.reliability.store(key, reliabilityState, descriptors)
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
		ReasonMCPAuthRefreshFailed,
		ReasonSourceDisabled:
		return deadentity.FailurePermanent
	default:
		return deadentity.FailureTransient
	}
}
