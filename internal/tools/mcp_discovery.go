package tools

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// DeferInitialDiscovery keeps vendor MCP startup out of the hosted session handshake.
func (*MCPProvider) DeferInitialDiscovery() bool {
	return true
}

// WithMCPDiscoveryBudget bounds one catalog discovery pass without shortening tool calls.
// Sources share the remaining budget, so a slow peer cannot consume later peers' time.
func WithMCPDiscoveryBudget(budget time.Duration) MCPProviderOption {
	return func(provider *MCPProvider) {
		if budget > 0 {
			provider.discoveryBudget = budget
		}
	}
}

func (p *MCPProvider) discoverSource(
	ctx context.Context,
	source SourceRef,
	timeout time.Duration,
) ([]MCPToolDescriptor, error) {
	if timeout <= 0 {
		return p.exec.ListTools(ctx, source)
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.exec.ListTools(discoveryCtx, source)
}

func (p *MCPProvider) cachedSourceDescriptors(source SourceRef) []Descriptor {
	value, ok := p.discovered.Load(source)
	if !ok {
		return nil
	}
	if snapshot, ok := value.(mcpDiscoverySnapshot); ok {
		return cloneDescriptors(snapshot.descriptors)
	}
	panic("invariant: discovered MCP descriptors have an unexpected type")
}

type mcpDiscoverySnapshot struct {
	raw         []MCPToolDescriptor
	descriptors []Descriptor
}

func (p *MCPProvider) normalizeDiscovery(source SourceRef, discovered []MCPToolDescriptor) ([]Descriptor, error) {
	if value, ok := p.discovered.Load(source); ok {
		if cached, ok := value.(mcpDiscoverySnapshot); ok {
			// Exact equality reuses schema digests without dropping internal read-authority metadata.
			if reflect.DeepEqual(cached.raw, discovered) {
				return cloneDescriptors(cached.descriptors), nil
			}
		}
	}
	descriptors := make([]Descriptor, 0, len(discovered))
	raw := make([]MCPToolDescriptor, len(discovered))
	for i := range discovered {
		descriptor, err := mcpRegistryDescriptor(source, discovered[i])
		if err != nil {
			return nil, wrapField(err, fmt.Sprintf("mcp_tools[%d]", i))
		}
		descriptors = append(descriptors, descriptor)
		raw[i] = discovered[i]
		raw[i].InputSchema = cloneRawMessage(raw[i].InputSchema)
		raw[i].OutputSchema = cloneRawMessage(raw[i].OutputSchema)
	}
	p.discovered.Store(source, mcpDiscoverySnapshot{raw, cloneDescriptors(descriptors)})
	return descriptors, nil
}

func cloneDescriptors(descriptors []Descriptor) []Descriptor {
	result := make([]Descriptor, len(descriptors))
	for i := range descriptors {
		result[i] = cloneDescriptor(descriptors[i])
	}
	return result
}

func (p *MCPProvider) forgetDiscovered(retire func(SourceRef) bool) {
	p.discovered.Range(func(key, _ any) bool {
		source, ok := key.(SourceRef)
		if !ok {
			panic("invariant: discovered MCP source has an unexpected type")
		}
		if retire(source) {
			p.discovered.Delete(key)
			p.discoveryFailures.Delete(key)
		}
		return true
	})
}

func (p *MCPProvider) retainDiscoveredSources(sources []SourceRef) {
	active := make(map[SourceRef]bool, len(sources))
	for _, source := range sources {
		active[source] = true
	}
	// Prune against the full source inventory: a workspace override must not evict
	// a global source's last discovery used by another workspace.
	p.forgetDiscovered(func(source SourceRef) bool {
		switch source.Scope {
		case "", mcpSourceScopeUser, mcpSourceScopeWorkspace:
			// Projection stamps the caller profile onto user and workspace sources.
			source.ProfileID = ""
		}
		return !active[source]
	})
}
