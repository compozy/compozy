package tools

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

type projectionListingsKey struct{}

// ProjectionListings shares validated provider listings among projection passes attached to it.
// It never expires entries: its owner bounds freshness by replacing the instance.
type ProjectionListings struct {
	mu      sync.Mutex
	entries map[projectionListingKey]*projectionListing
}

type projectionListingKey struct {
	registry      *RuntimeRegistry
	providerIndex int
	scope         Scope
}

// projectionListing is immutable once done closes; readers clone descriptors before mutating them.
type projectionListing struct {
	done        chan struct{}
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
	return listings.load(ctx, key, func() ([]Descriptor, bool, error) {
		return r.listValidatedProvider(ctx, provider, key, scope)
	})
}

// load joins an in-flight or finished listing for key. Failures are never shared: a waiter whose
// leader failed lists on its own context.
func (l *ProjectionListings) load(
	ctx context.Context,
	key projectionListingKey,
	list func() ([]Descriptor, bool, error),
) ([]Descriptor, error) {
	l.mu.Lock()
	entry, joined := l.entries[key]
	if !joined {
		entry = &projectionListing{done: make(chan struct{})}
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
			delete(l.entries, key)
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
		if validated.entries == nil || len(validated.entries) >= 128 {
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
	mu      sync.Mutex
	entries map[projectionListingKey][]Descriptor
}
