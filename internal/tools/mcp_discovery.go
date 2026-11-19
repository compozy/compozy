package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
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
	if BackgroundMCPDiscovery(ctx) {
		discoveryCtx = context.WithValue(discoveryCtx, mcpDiscoveryResponseBudgetKey{}, timeout/2)
	}
	return p.exec.ListTools(discoveryCtx, source)
}

type mcpDiscoveryResponseBudgetKey struct{}
type mcpOwnedDiscoveryKey struct{}

// BackgroundMCPDiscovery reports whether the daemon's hosted catalog worker owns the read.
func BackgroundMCPDiscovery(ctx context.Context) bool {
	return ctx.Value(mcpOwnedDiscoveryKey{}) != nil
}

// MCPCatalogPollDeferred reports that a shared upstream attempt is still within its refresh interval.
// A deferred read can reproduce the last diagnostic, but provides no new evidence of failure.
type MCPCatalogPollDeferred struct{ Reason ReasonCode }

func (*MCPCatalogPollDeferred) Error() string { return "mcp: catalog refresh interval has not elapsed" }

// MCPDiscoveryResponseBudget reserves server response time inside the daemon's discovery budget.
func MCPDiscoveryResponseBudget(ctx context.Context) time.Duration {
	if budget, ok := ctx.Value(mcpDiscoveryResponseBudgetKey{}).(time.Duration); ok {
		return budget
	}
	return 0
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
	source      SourceRef
	content     [sha256.Size]byte
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
	return s.descriptorsAt(redact.RegisteredSecretSnapshot())
}

func (s *mcpDiscoverySnapshot) descriptorsAt(snapshot *redact.SecretSnapshot) []Descriptor {
	revision := snapshot.MaterialFingerprint()
	if s.hasSafeView && s.safeRevision == revision {
		return s.safe
	}
	safe := make([]Descriptor, 0, len(s.descriptors))
	for i, descriptor := range s.descriptors {
		if safeMCPDescriptor(snapshot, descriptor) {
			safe = append(safe, descriptor)
			continue
		}
		redacted, err := RedactMCPToolDescriptor(s.raw[i], snapshot)
		if err != nil {
			continue
		}
		descriptor, err = mcpRegistryDescriptor(s.source, redacted)
		if err == nil && safeMCPDescriptor(snapshot, descriptor) {
			safe = append(safe, descriptor)
		}
	}
	if revision != redact.RegisteredSecretSnapshot().MaterialFingerprint() {
		// Preserve descriptors proven safe against the current material without caching this mixed view.
		redact.WithStableSecretSnapshot(func(current *redact.SecretSnapshot) {
			safe = slices.DeleteFunc(safe, func(descriptor Descriptor) bool {
				return !safeMCPDescriptor(current, descriptor)
			})
		})
		return safe
	}
	s.safeRevision, s.safe, s.hasSafeView = revision, safe, true
	return safe
}

func safeMCPDescriptor(snapshot *redact.SecretSnapshot, descriptor Descriptor) bool {
	raw, err := json.Marshal(descriptor)
	if err != nil {
		return false
	}
	unsafe, err := snapshot.NeedsBinaryJSONRedaction(raw)
	return err == nil && !unsafe
}

func (p *MCPProvider) normalizeDiscovery(source SourceRef, discovered []MCPToolDescriptor) ([]Descriptor, error) {
	if value, ok := p.discovered.Load(source); ok {
		if cached, ok := value.(*mcpDiscoverySnapshot); ok {
			// Exact equality reuses schema digests without dropping internal read-authority metadata.
			if reflect.DeepEqual(cached.raw, discovered) {
				return cloneDescriptors(cached.currentDescriptors()), nil
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
	content, err := FingerprintMCPToolDescriptors(discovered)
	if err != nil {
		return nil, err
	}
	snapshot := &mcpDiscoverySnapshot{source: source, content: content, raw: raw, descriptors: descriptors}
	p.discovered.Store(source, snapshot)
	return cloneDescriptors(snapshot.currentDescriptors()), nil
}

// FingerprintMCPToolDescriptors identifies the contents of one normalized listing.
func FingerprintMCPToolDescriptors(descriptors []MCPToolDescriptor) ([sha256.Size]byte, error) {
	canonical := make([]json.RawMessage, 0, len(descriptors))
	for index := range descriptors {
		// Providers may leave ID empty and let the registry derive it from the upstream name.
		descriptor := &descriptors[index]
		encoded, err := json.Marshal(struct {
			*MCPToolDescriptor
			ID string `json:"id"`
		}{descriptor, string(descriptor.ID)})
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("mcp: encode tool projection descriptor: %w", err)
		}
		canonical = append(canonical, encoded)
	}
	slices.SortFunc(canonical, func(left, right json.RawMessage) int {
		return bytes.Compare(left, right)
	})
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("mcp: encode tool projection generation: %w", err)
	}
	return sha256.Sum256(encoded), nil
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
