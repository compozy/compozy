package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/deadentity"
	"github.com/compozy/agh/internal/store"
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
			cache:   make(map[store.DeadEntityKey][]Descriptor),
		}
	}
}

type mcpReliability struct {
	service MCPDeadEntityService

	mu    sync.RWMutex
	cache map[store.DeadEntityKey][]Descriptor
}

func (p *MCPProvider) listSourceDescriptors(ctx context.Context, source SourceRef) ([]Descriptor, error) {
	key, tracked := mcpDeadEntityKey(source)
	if tracked && p.reliability != nil {
		decision, err := p.reliability.service.BeforeProbe(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("tools: admit mcp source %q: %w", source.Owner, err)
		}
		if !decision.Allowed {
			return p.reliability.cached(key), nil
		}
	}

	discovered, err := p.exec.ListTools(ctx, source)
	if err != nil {
		if contextError := contextErrFromError("", err); contextError != nil {
			return nil, contextError
		}
		if tracked && p.reliability != nil {
			if recordErr := p.reliability.service.RecordFailure(
				ctx,
				key,
				MCPDiscoveryFailureClass(err),
				err.Error(),
			); recordErr != nil {
				return nil, fmt.Errorf("tools: record mcp source %q failure: %w", source.Owner, recordErr)
			}
			return p.reliability.cached(key), nil
		}
		return nil, nil
	}

	descriptors := make([]Descriptor, 0, len(discovered))
	for i := range discovered {
		descriptor, err := mcpRegistryDescriptor(source, discovered[i])
		if err != nil {
			return nil, wrapField(err, fmt.Sprintf("mcp_tools[%d]", i))
		}
		descriptors = append(descriptors, descriptor)
	}
	if tracked && p.reliability != nil {
		if err := p.reliability.service.RecordSuccess(ctx, key); err != nil {
			return nil, fmt.Errorf("tools: record mcp source %q recovery: %w", source.Owner, err)
		}
		p.reliability.store(key, descriptors)
	}
	return descriptors, nil
}

func (r *mcpReliability) dead(ctx context.Context, source SourceRef) bool {
	if r == nil || isNilInterface(r.service) {
		return false
	}
	key, tracked := mcpDeadEntityKey(source)
	if !tracked {
		return false
	}
	status, err := r.service.Status(ctx, key)
	return err == nil && status.Dead
}

func (r *mcpReliability) cached(key store.DeadEntityKey) []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptors := r.cache[key]
	cloned := make([]Descriptor, len(descriptors))
	for i := range descriptors {
		cloned[i] = cloneDescriptor(descriptors[i])
	}
	return cloned
}

func (r *mcpReliability) store(key store.DeadEntityKey, descriptors []Descriptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := make([]Descriptor, len(descriptors))
	for i := range descriptors {
		cloned[i] = cloneDescriptor(descriptors[i])
	}
	r.cache[key] = cloned
}

func (r *mcpReliability) retainActiveSources(scope Scope, sources []SourceRef) {
	if r == nil {
		return
	}
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		return
	}
	active := make(map[store.DeadEntityKey]struct{}, len(sources))
	for _, source := range sources {
		if key, tracked := mcpDeadEntityKey(source); tracked {
			active[key] = struct{}{}
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.cache {
		if key.WorkspaceID != workspaceID {
			continue
		}
		if _, ok := active[key]; !ok {
			delete(r.cache, key)
		}
	}
}

func mcpDeadEntityKey(source SourceRef) (store.DeadEntityKey, bool) {
	if source.WorkspaceID == "" {
		return store.DeadEntityKey{}, false
	}
	key := store.DeadEntityKey{
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
