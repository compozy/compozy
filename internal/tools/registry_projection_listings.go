package tools

import (
	"context"
	"errors"
	"fmt"
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

// listProvider returns sorted, validated descriptors that callers must clone before mutating.
func (r *RuntimeRegistry) listProvider(
	ctx context.Context,
	provider Provider,
	providerIndex int,
	scope Scope,
) ([]Descriptor, error) {
	listings, shared := ctx.Value(projectionListingsKey{}).(*ProjectionListings)
	scoped, declared := provider.(ProjectionScopeProvider)
	if !shared || !declared {
		descriptors, _, err := listValidatedProvider(ctx, provider, scope)
		return descriptors, err
	}
	key := projectionListingKey{registry: r, providerIndex: providerIndex, scope: scoped.ProjectionScope(scope)}
	return listings.load(ctx, key, func() ([]Descriptor, bool, error) {
		return listValidatedProvider(ctx, provider, scope)
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

func listValidatedProvider(ctx context.Context, provider Provider, scope Scope) ([]Descriptor, bool, error) {
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
	for i := range descriptors {
		if err := descriptors[i].Validate(); err != nil {
			return nil, false, indexValidationError(descriptors[i].ID, err)
		}
	}
	return descriptors, complete, nil
}
