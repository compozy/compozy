package bridgesdk

import (
	"context"
	"errors"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrProviderStopped reports that a provider stopped while waiting for runtime state.
var ErrProviderStopped = errors.New("bridgesdk: provider stopped")

// RoutePathFunc returns every normalized webhook path owned by one resolved config.
type RoutePathFunc[C any] func(C) []string

// RouteMergeFunc carries provider-owned resources from an existing config into its replacement.
// It must be deterministic and safe to invoke again when reconciliation publishes a finalized snapshot.
type RouteMergeFunc[C any] func(previous C, next C) C

// RouteTable stores resolved configs by instance and indexes zero or more configs per path.
type RouteTable[C any] struct {
	mu       sync.RWMutex
	routes   map[string]C
	paths    map[string][]string
	pathKeys RoutePathFunc[C]
	changed  chan struct{}
}

// NewRouteTable creates an empty typed route table.
func NewRouteTable[C any](pathKeys RoutePathFunc[C]) *RouteTable[C] {
	return &RouteTable[C]{
		routes:   make(map[string]C),
		paths:    make(map[string][]string),
		pathKeys: pathKeys,
		changed:  make(chan struct{}),
	}
}

// Replace atomically swaps all configs and returns values removed from the table.
func (t *RouteTable[C]) Replace(next map[string]C, merge RouteMergeFunc[C]) []C {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	replacement, removed := t.prepareReplacementLocked(next, merge)
	t.publishReplacementLocked(replacement)
	return removed
}

// ReplaceWithCleanup commits a route swap only after every removed config is cleaned successfully.
func (t *RouteTable[C]) ReplaceWithCleanup(
	next map[string]C,
	merge RouteMergeFunc[C],
	cleanup func(C) error,
) error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	replacement, removed := t.prepareReplacementLocked(next, merge)
	if cleanup != nil {
		for _, config := range removed {
			if err := cleanup(config); err != nil {
				return err
			}
		}
	}
	t.publishReplacementLocked(replacement)
	return nil
}

func (t *RouteTable[C]) prepareReplacementLocked(
	next map[string]C,
	merge RouteMergeFunc[C],
) (map[string]C, []C) {
	replacement := make(map[string]C, len(next))
	for instanceID, config := range next {
		instanceID = strings.TrimSpace(instanceID)
		if instanceID == "" {
			continue
		}
		if previous, ok := t.routes[instanceID]; ok && merge != nil {
			config = merge(previous, config)
		}
		replacement[instanceID] = config
	}
	removed := make([]C, 0)
	for instanceID, config := range t.routes {
		if _, retained := replacement[instanceID]; !retained {
			removed = append(removed, config)
		}
	}
	return replacement, removed
}

func (t *RouteTable[C]) publishReplacementLocked(replacement map[string]C) {
	t.routes = replacement
	t.reindexLocked()
	close(t.changed)
	t.changed = make(chan struct{})
}

// Get returns one config by bridge instance id.
func (t *RouteTable[C]) Get(instanceID string) (C, bool) {
	var zero C
	if t == nil {
		return zero, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	config, ok := t.routes[strings.TrimSpace(instanceID)]
	return config, ok
}

// ByPath returns every config indexed under one normalized webhook path.
func (t *RouteTable[C]) ByPath(path string) []C {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	ids := t.paths[normalizeProviderRoutePath(path)]
	configs := make([]C, 0, len(ids))
	for _, id := range ids {
		if config, ok := t.routes[id]; ok {
			configs = append(configs, config)
		}
	}
	return configs
}

// Snapshot returns a copy of the current instance map.
func (t *RouteTable[C]) Snapshot() map[string]C {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	cloned := make(map[string]C, len(t.routes))
	maps.Copy(cloned, t.routes)
	return cloned
}

// Update replaces one existing config through a provider-owned mutation.
func (t *RouteTable[C]) Update(instanceID string, update func(C) C) bool {
	if t == nil || update == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	instanceID = strings.TrimSpace(instanceID)
	current, ok := t.routes[instanceID]
	if !ok {
		return false
	}
	t.routes[instanceID] = update(current)
	t.reindexLocked()
	return true
}

// Wait returns when one config becomes available, the timeout expires, or the provider stops.
func (t *RouteTable[C]) Wait(
	ctx context.Context,
	instanceID string,
	timeout time.Duration,
	stop <-chan struct{},
) (C, bool, error) {
	var zero C
	if ctx == nil {
		return zero, false, errors.New("bridgesdk: route wait context is required")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		config, ok, changed := t.getAndChanged(instanceID)
		if ok {
			return config, true, nil
		}
		select {
		case <-ctx.Done():
			return zero, false, ctx.Err()
		case <-stop:
			return zero, false, ErrProviderStopped
		case <-timer.C:
			return zero, false, nil
		case <-changed:
		}
	}
}

func (t *RouteTable[C]) getAndChanged(instanceID string) (C, bool, <-chan struct{}) {
	var zero C
	if t == nil {
		closed := make(chan struct{})
		close(closed)
		return zero, false, closed
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	config, ok := t.routes[strings.TrimSpace(instanceID)]
	return config, ok, t.changed
}

func (t *RouteTable[C]) reindexLocked() {
	t.paths = make(map[string][]string)
	if t.pathKeys == nil {
		return
	}
	for instanceID, config := range t.routes {
		for _, path := range t.pathKeys(config) {
			path = normalizeProviderRoutePath(path)
			if path == "" {
				continue
			}
			t.paths[path] = append(t.paths[path], instanceID)
		}
	}
	for path := range t.paths {
		sort.Strings(t.paths[path])
	}
}

func normalizeProviderRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}
