package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/redact"
)

type projectionListingsKey struct{}

const defaultProjectionValidationCacheCapacity = 128

// WithProjectionValidationCacheCapacity sets the positive cache limit in place of the default of 128 entries.
func WithProjectionValidationCacheCapacity(capacity int) RegistryOption {
	return func(registry *RuntimeRegistry) {
		if capacity > 0 {
			registry.validatedListings.capacity = capacity
		}
	}
}

// ProjectionListings shares validated provider listings among projection passes attached to it.
// It never expires entries: its owner bounds freshness by replacing the instance.
type ProjectionListings struct {
	mu                   sync.Mutex
	entries              map[projectionListingKey]*projectionListing
	policies             projectionMemo[projectionPolicyKey, []ToolView]
	authorities          projectionMemo[hostedSessionPolicyKey, SessionToolPolicy]
	nativeObservations   projectionMemo[*RuntimeRegistry, string]
	externalObservations projectionMemo[projectionObservationKey, [sha256.Size]byte]
}

type hostedSessionPolicyKey struct {
	scope     Scope
	policy    [sha256.Size]byte
	ceiling   string
	redaction [sha256.Size]byte
}

// CachedHostedSessionPolicy shares materialized authority for equal hosted visibility inputs.
func CachedHostedSessionPolicy(
	ctx context.Context, scope Scope, inputs PolicyInputs, ceiling string,
	materialize func() (SessionToolPolicy, error),
) (SessionToolPolicy, error) {
	listings, ok := ctx.Value(projectionListingsKey{}).(*ProjectionListings)
	if !ok || !IsHostedCatalogRead(ctx) {
		return materialize()
	}
	data, err := json.Marshal(projectionPolicy(inputs))
	if err != nil {
		return SessionToolPolicy{}, fmt.Errorf("tools: encode hosted authority inputs: %w", err)
	}
	key := hostedSessionPolicyKey{scope, sha256.Sum256(data), ceiling, redact.SecretFingerprint()}
	return listings.authorities.load(ctx, key, func() (SessionToolPolicy, bool, error) {
		policy, err := materialize()
		if err != nil || key.redaction != redact.SecretFingerprint() {
			return policy, false, err
		}
		encoded, err := json.Marshal(policy)
		if err != nil {
			return SessionToolPolicy{}, false, fmt.Errorf("tools: encode hosted authority: %w", err)
		}
		policy.projectionDigest = sha256.Sum256(encoded)
		return policy, true, nil
	})
}

type projectionPolicyKey struct {
	registry  *RuntimeRegistry
	scope     Scope
	policy    [sha256.Size]byte
	redaction [sha256.Size]byte
}

func (l *ProjectionListings) loadPolicy(
	ctx context.Context, r *RuntimeRegistry, scope Scope, inputs PolicyInputs,
) ([]ToolView, error) {
	data, err := json.Marshal(projectionPolicy(inputs))
	if err != nil {
		return nil, fmt.Errorf("tools: encode projection policy: %w", err)
	}
	key := projectionPolicyKey{r, scope, sha256.Sum256(data), r.ProjectionRedactionRevision()}
	return l.policies.load(ctx, key, func() ([]ToolView, bool, error) {
		views, err := r.policyProjection(ctx, scope, inputs)
		return views, key.redaction == r.ProjectionRedactionRevision(), err
	})
}

type projectionMemo[K comparable, V any] struct {
	mu      sync.Mutex
	entries map[K]V
	pending map[K]chan struct{}
}

// load coalesces successful builds. A waiter can leave on cancellation or retry a failed build.
func (m *projectionMemo[K, V]) load(ctx context.Context, key K, build func() (V, bool, error)) (V, error) {
	for {
		if err := ctx.Err(); err != nil {
			var zero V
			return zero, err
		}
		m.mu.Lock()
		if value, found := m.entries[key]; found {
			m.mu.Unlock()
			return value, nil
		}
		done, pending := m.pending[key]
		if !pending {
			done = make(chan struct{})
			if m.pending == nil {
				m.pending = make(map[K]chan struct{})
			}
			m.pending[key] = done
		}
		m.mu.Unlock()
		if !pending {
			return m.build(key, done, build)
		}
		select {
		case <-ctx.Done():
			var zero V
			return zero, ctx.Err()
		case <-done:
		}
	}
}

func (m *projectionMemo[K, V]) build(key K, done chan struct{}, build func() (V, bool, error)) (V, error) {
	defer func() {
		m.mu.Lock()
		delete(m.pending, key)
		close(done)
		m.mu.Unlock()
	}()
	value, cacheable, err := build()
	if cacheable && err == nil {
		m.mu.Lock()
		if len(m.entries) >= defaultProjectionValidationCacheCapacity || m.entries == nil {
			m.entries = make(map[K]V)
		}
		m.entries[key] = value
		m.mu.Unlock()
	}
	return value, err
}

type projectionListingKey struct {
	registry      *RuntimeRegistry
	providerIndex int
	scope         Scope
}

// projectionListing is immutable once done closes; readers clone descriptors before mutating them.
type projectionListing struct {
	done        chan struct{}
	redaction   [sha256.Size]byte
	descriptors []Descriptor
	err         error
	complete    bool
}

// NewProjectionListings creates an empty listing share.
func NewProjectionListings() *ProjectionListings {
	return &ProjectionListings{entries: make(map[projectionListingKey]*projectionListing)}
}

// WithProjectionListings lets projection passes on ctx reuse listings of providers that declare
// their listing scope, so peers with the same provider scope list each provider once.
func WithProjectionListings(ctx context.Context, listings *ProjectionListings) context.Context {
	if listings == nil {
		return ctx
	}
	return context.WithValue(ctx, projectionListingsKey{}, listings)
}

// EnsureProjectionListings preserves a refresh window's listing share when one is already present.
func EnsureProjectionListings(ctx context.Context) context.Context {
	if listings, ok := ctx.Value(projectionListingsKey{}).(*ProjectionListings); ok && listings != nil {
		return ctx
	}
	return WithProjectionListings(ctx, NewProjectionListings())
}

// ProjectionRedactionRevision reports the registry's live MCP redaction dependency.
func (r *RuntimeRegistry) ProjectionRedactionRevision() [sha256.Size]byte {
	for _, provider := range r.providers {
		if _, mcp := provider.(*MCPProvider); mcp {
			return redact.SecretFingerprint()
		}
	}
	return [sha256.Size]byte{}
}

// listProvider returns sorted, validated descriptors that callers must clone before mutating.
func (r *RuntimeRegistry) listProvider(
	ctx context.Context,
	provider Provider,
	providerIndex int,
	scope Scope,
) ([]Descriptor, error) {
	listings, shared := ctx.Value(projectionListingsKey{}).(*ProjectionListings)
	scoped, declared := provider.(ProjectionScopeProvider)
	providerScope := scope
	if declared {
		providerScope = scoped.ProjectionScope(scope)
	}
	key := projectionListingKey{registry: r, providerIndex: providerIndex, scope: providerScope}
	if !shared || !declared {
		descriptors, _, err := r.listValidatedProvider(ctx, provider, key, scope)
		return descriptors, err
	}
	var redaction [sha256.Size]byte
	// MCP descriptor normalization uses the live secret registry.
	if _, mcp := provider.(*MCPProvider); mcp {
		redaction = redact.SecretFingerprint()
	}
	return listings.load(ctx, key, redaction, func() ([]Descriptor, bool, error) {
		return r.listValidatedProvider(ctx, provider, key, scope)
	})
}

// load joins an in-flight or finished listing for key. Failures are never shared: a waiter whose
// leader failed lists on its own context.
func (l *ProjectionListings) load(
	ctx context.Context,
	key projectionListingKey,
	redaction [sha256.Size]byte,
	list func() ([]Descriptor, bool, error),
) ([]Descriptor, error) {
	l.mu.Lock()
	entry, joined := l.entries[key]
	if !joined || entry.redaction != redaction {
		joined = false
		entry = &projectionListing{done: make(chan struct{}), redaction: redaction}
		l.entries[key] = entry
	}
	l.mu.Unlock()
	if !joined {
		return l.lead(key, entry, list)
	}
	select {
	case <-ctx.Done():
		return nil, contextErr(ctx, "")
	case <-entry.done:
	}
	if entry.err != nil || !entry.complete {
		descriptors, _, err := list()
		return descriptors, err
	}
	return entry.descriptors, nil
}

func (l *ProjectionListings) lead(
	key projectionListingKey,
	entry *projectionListing,
	list func() ([]Descriptor, bool, error),
) ([]Descriptor, error) {
	entry.err = errProjectionListingAbandoned
	defer func() {
		if entry.err != nil || !entry.complete {
			l.mu.Lock()
			if l.entries[key] == entry {
				delete(l.entries, key)
			}
			l.mu.Unlock()
		}
		close(entry.done)
	}()
	entry.descriptors, entry.complete, entry.err = list()
	return entry.descriptors, entry.err
}

var errProjectionListingAbandoned = errors.New("tools: projection listing abandoned")

func (r *RuntimeRegistry) listValidatedProvider(
	ctx context.Context, provider Provider, key projectionListingKey, scope Scope,
) ([]Descriptor, bool, error) {
	var descriptors []Descriptor
	var err error
	complete := true
	if listing, ok := provider.(interface {
		listProjection(context.Context, Scope) ([]Descriptor, bool, error)
	}); ok {
		descriptors, complete, err = listing.listProjection(ctx, scope)
	} else {
		descriptors, err = provider.List(ctx, scope)
	}
	if err != nil {
		return nil, false, fmt.Errorf("tools: list provider %s: %w", sourceKey(provider.ID()), err)
	}
	slices.SortFunc(descriptors, func(a Descriptor, b Descriptor) int {
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	validated := &r.validatedListings
	validated.mu.Lock()
	defer validated.mu.Unlock()
	// Exact Go-value equality avoids re-parsing schemas while retaining non-JSON policy metadata.
	if previous, ok := validated.entries[key]; ok && reflect.DeepEqual(previous, descriptors) {
		return previous, complete, nil
	}
	for i := range descriptors {
		if err := descriptors[i].Validate(); err != nil {
			return nil, false, indexValidationError(descriptors[i].ID, err)
		}
	}
	if complete {
		if validated.entries == nil || len(validated.entries) >= validated.capacity {
			validated.entries = make(map[projectionListingKey][]Descriptor)
		}
		immutable := make([]Descriptor, len(descriptors))
		for i := range descriptors {
			immutable[i] = cloneDescriptor(descriptors[i])
		}
		validated.entries[key] = immutable
		return immutable, true, nil
	}
	return descriptors, complete, nil
}

type validatedProviderListings struct {
	mu       sync.Mutex
	entries  map[projectionListingKey][]Descriptor
	capacity int
}
