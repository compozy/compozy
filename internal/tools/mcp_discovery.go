package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/redact"
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
	snapshot, ok := value.(*mcpDiscoverySnapshot)
	if !ok {
		panic("invariant: discovered MCP source has an unexpected type")
	}
	return cloneDescriptors(snapshot.currentDescriptors())
}

type mcpDiscoverySnapshot struct {
	raw         []MCPToolDescriptor
	descriptors []Descriptor

	mu           sync.Mutex
	hasSafeView  bool
	safeRevision [sha256.Size]byte
	safe         []Descriptor
}

func (s *mcpDiscoverySnapshot) currentDescriptors() []Descriptor {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := redact.RegisteredSecretSnapshot()
	revision := snapshot.MaterialFingerprint()
	if s.hasSafeView && s.safeRevision == revision {
		return s.safe
	}
	safe := make([]Descriptor, 0, len(s.descriptors))
	for _, descriptor := range s.descriptors {
		raw, err := json.Marshal(descriptor)
		if err != nil {
			continue
		}
		unsafe, err := snapshot.NeedsBinaryJSONRedaction(raw)
		if err == nil && !unsafe {
			safe = append(safe, descriptor)
		}
	}
	if snapshot != redact.RegisteredSecretSnapshot() {
		return nil
	}
	s.safeRevision, s.safe, s.hasSafeView = revision, safe, true
	return safe
}

func (p *MCPProvider) normalizeDiscovery(source SourceRef, discovered []MCPToolDescriptor) ([]Descriptor, error) {
	if value, ok := p.discovered.Load(source); ok {
		if cached, ok := value.(*mcpDiscoverySnapshot); ok {
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
	p.discovered.Store(source, &mcpDiscoverySnapshot{raw: raw, descriptors: cloneDescriptors(descriptors)})
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
	p.discoveryMu.Lock()
	defer p.discoveryMu.Unlock()
	for source := range p.discoveryGenerations {
		if retire(source) {
			delete(p.discoveryGenerations, source)
			p.discovered.Delete(source)
			p.discoveryFailures.Delete(source)
		}
	}
}

func (p *MCPProvider) retainDiscoveredSources(sources []SourceRef) {
	active := make(map[SourceRef]bool, len(sources))
	for _, source := range sources {
		active[source] = true
	}
	// Prune against the full source inventory: a workspace override must not evict
	// a global source's last discovery used by another workspace.
	p.forgetDiscovered(func(source SourceRef) bool {
		return !active[MCPDeclarationSource(source)]
	})
}

// MCPDeclarationSource removes the caller's profile from inherited declaration identities.
func MCPDeclarationSource(source SourceRef) SourceRef {
	switch source.Scope {
	case "", mcpSourceScopeUser, mcpSourceScopeWorkspace:
		source.ProfileID = ""
	}
	return source
}
