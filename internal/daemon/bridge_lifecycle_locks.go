package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"
)

var errBridgeLifecycleContextRequired = errors.New("daemon: bridge lifecycle lock context is required")

type bridgeLifecycleScope uint8

const (
	bridgeLifecycleScopeInstance bridgeLifecycleScope = iota
	bridgeLifecycleScopeExtension
)

type bridgeLifecycleLock struct {
	gate lifecycleRWGate
	refs int
}

type bridgeLifecycleContextKey struct{}

type bridgeLifecycleContextState struct {
	extensions       map[string]struct{}
	sharedExtensions map[string]struct{}
	instances        map[string]struct{}
}

// lockInstanceLifecycle serializes lifecycle transitions for one bridge
// instance so reload-triggered rollbacks cannot overwrite newer persisted state.
func (r *bridgeRuntime) lockInstanceLifecycle(id string) func() {
	return r.lockLifecycleUncancelable(strings.TrimSpace(id), bridgeLifecycleScopeInstance, false)
}

func (r *bridgeRuntime) lockExtensionLifecycle(extensionName string) func() {
	return r.lockLifecycleUncancelable(
		strings.TrimSpace(extensionName),
		bridgeLifecycleScopeExtension,
		false,
	)
}

func (r *bridgeRuntime) lockLifecycleUncancelable(
	key string,
	scope bridgeLifecycleScope,
	shared bool,
) func() {
	lock, releaseRef := r.referenceLifecycleLock(key, scope)
	if lock == nil {
		return func() {}
	}
	unlockGate := lock.gate.lockUncancelable(shared)
	return func() {
		unlockGate()
		releaseRef()
	}
}

func (r *bridgeRuntime) lockLifecycle(
	ctx context.Context,
	key string,
	scope bridgeLifecycleScope,
	shared bool,
) (func(), error) {
	lock, releaseRef := r.referenceLifecycleLock(strings.TrimSpace(key), scope)
	if lock == nil {
		return func() {}, nil
	}
	unlockGate, err := lock.gate.lock(ctx, shared)
	if err != nil {
		releaseRef()
		return nil, err
	}
	return func() {
		unlockGate()
		releaseRef()
	}, nil
}

func (r *bridgeRuntime) referenceLifecycleLock(
	key string,
	scope bridgeLifecycleScope,
) (*bridgeLifecycleLock, func()) {
	if r == nil || key == "" {
		return nil, func() {}
	}

	r.lifecycleMu.Lock()
	locks := r.lifecycleLockMapLocked(scope)
	lock := locks[key]
	if lock == nil {
		lock = &bridgeLifecycleLock{}
		locks[key] = lock
	}
	lock.refs++
	r.lifecycleMu.Unlock()

	return lock, func() {
		r.lifecycleMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(r.lifecycleLockMapLocked(scope), key)
		}
		r.lifecycleMu.Unlock()
	}
}

func (r *bridgeRuntime) lifecycleLockMapLocked(scope bridgeLifecycleScope) map[string]*bridgeLifecycleLock {
	if scope == bridgeLifecycleScopeExtension {
		if r.extensionLifecycleLocks == nil {
			r.extensionLifecycleLocks = make(map[string]*bridgeLifecycleLock)
		}
		return r.extensionLifecycleLocks
	}
	if r.lifecycleLocks == nil {
		r.lifecycleLocks = make(map[string]*bridgeLifecycleLock)
	}
	return r.lifecycleLocks
}

func (r *bridgeRuntime) lockManagedInstanceLifecycleSet(
	ctx context.Context,
	ids []string,
) (context.Context, func()) {
	if len(ids) == 0 {
		return ctx, func() {}
	}

	normalized := append([]string(nil), ids...)
	for idx := range normalized {
		normalized[idx] = strings.TrimSpace(normalized[idx])
	}
	normalized = slices.DeleteFunc(normalized, func(id string) bool { return id == "" })
	if len(normalized) == 0 {
		return ctx, func() {}
	}

	slices.Sort(normalized)
	normalized = slices.Compact(normalized)

	unlocks := make([]func(), 0, len(normalized))
	lockedIDs := make([]string, 0, len(normalized))
	for _, id := range normalized {
		if bridgeLifecycleContextHasInstance(ctx, id) {
			continue
		}
		unlocks = append(unlocks, r.lockInstanceLifecycle(id))
		lockedIDs = append(lockedIDs, id)
	}

	updatedCtx := withBridgeLifecycleContextInstances(ctx, lockedIDs...)
	return updatedCtx, reverseUnlock(unlocks)
}

func (r *bridgeRuntime) lockExtensionLifecycleContext(
	ctx context.Context,
	extensionName string,
) (context.Context, func()) {
	trimmed := strings.TrimSpace(extensionName)
	if trimmed == "" || bridgeLifecycleContextHasExtension(ctx, trimmed) {
		return ctx, func() {}
	}

	unlock := r.lockExtensionLifecycle(trimmed)
	return withBridgeLifecycleContextExtensions(ctx, trimmed), unlock
}

func (r *bridgeRuntime) lockInstanceLifecycleContext(ctx context.Context, id string) (context.Context, func()) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" || bridgeLifecycleContextHasInstance(ctx, trimmed) {
		return ctx, func() {}
	}

	unlock := r.lockInstanceLifecycle(trimmed)
	return withBridgeLifecycleContextInstances(ctx, trimmed), unlock
}

func (r *bridgeRuntime) lockBridgeControlLifecycleContext(
	ctx context.Context,
	extensionName string,
	instanceID string,
) (context.Context, func(), error) {
	if ctx == nil {
		return nil, nil, errBridgeLifecycleContextRequired
	}
	name := strings.TrimSpace(extensionName)
	id := strings.TrimSpace(instanceID)
	updatedCtx := ctx
	unlocks := make([]func(), 0, 2)

	if name != "" && !bridgeLifecycleContextHasExtension(updatedCtx, name) &&
		!bridgeLifecycleContextHasSharedExtension(updatedCtx, name) {
		unlock, err := r.lockLifecycle(ctx, name, bridgeLifecycleScopeExtension, true)
		if err != nil {
			return ctx, nil, err
		}
		unlocks = append(unlocks, unlock)
		updatedCtx = withBridgeLifecycleContextSharedExtensions(updatedCtx, name)
	}
	if id != "" && !bridgeLifecycleContextHasInstance(updatedCtx, id) {
		unlock, err := r.lockLifecycle(ctx, id, bridgeLifecycleScopeInstance, false)
		if err != nil {
			reverseUnlock(unlocks)()
			return ctx, nil, err
		}
		unlocks = append(unlocks, unlock)
		updatedCtx = withBridgeLifecycleContextInstances(updatedCtx, id)
	}
	return updatedCtx, reverseUnlock(unlocks), nil
}

func reverseUnlock(unlocks []func()) func() {
	return func() {
		for _, unlock := range slices.Backward(unlocks) {
			unlock()
		}
	}
}

func bridgeLifecycleContextHasExtension(ctx context.Context, extensionName string) bool {
	state, ok := bridgeLifecycleStateFromContext(ctx)
	if !ok || len(state.extensions) == 0 {
		return false
	}
	_, present := state.extensions[strings.TrimSpace(extensionName)]
	return present
}

func bridgeLifecycleContextHasSharedExtension(ctx context.Context, extensionName string) bool {
	state, ok := bridgeLifecycleStateFromContext(ctx)
	if !ok || len(state.sharedExtensions) == 0 {
		return false
	}
	_, present := state.sharedExtensions[strings.TrimSpace(extensionName)]
	return present
}

func bridgeLifecycleContextHasInstance(ctx context.Context, id string) bool {
	state, ok := bridgeLifecycleStateFromContext(ctx)
	if !ok || len(state.instances) == 0 {
		return false
	}
	_, present := state.instances[strings.TrimSpace(id)]
	return present
}

func withBridgeLifecycleContextExtensions(ctx context.Context, extensionNames ...string) context.Context {
	if ctx == nil {
		return nil
	}

	state, _ := bridgeLifecycleStateFromContext(ctx)
	next := bridgeLifecycleContextState{
		extensions:       cloneBridgeLifecycleContextSet(state.extensions),
		sharedExtensions: cloneBridgeLifecycleContextSet(state.sharedExtensions),
		instances:        cloneBridgeLifecycleContextSet(state.instances),
	}
	for _, name := range extensionNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if next.extensions == nil {
			next.extensions = make(map[string]struct{})
		}
		next.extensions[trimmed] = struct{}{}
	}
	return context.WithValue(ctx, bridgeLifecycleContextKey{}, next)
}

func withBridgeLifecycleContextSharedExtensions(ctx context.Context, extensionNames ...string) context.Context {
	if ctx == nil {
		return nil
	}

	state, _ := bridgeLifecycleStateFromContext(ctx)
	next := bridgeLifecycleContextState{
		extensions:       cloneBridgeLifecycleContextSet(state.extensions),
		sharedExtensions: cloneBridgeLifecycleContextSet(state.sharedExtensions),
		instances:        cloneBridgeLifecycleContextSet(state.instances),
	}
	for _, name := range extensionNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if next.sharedExtensions == nil {
			next.sharedExtensions = make(map[string]struct{})
		}
		next.sharedExtensions[trimmed] = struct{}{}
	}
	return context.WithValue(ctx, bridgeLifecycleContextKey{}, next)
}

func withBridgeLifecycleContextInstances(ctx context.Context, ids ...string) context.Context {
	if ctx == nil {
		return nil
	}

	state, _ := bridgeLifecycleStateFromContext(ctx)
	next := bridgeLifecycleContextState{
		extensions:       cloneBridgeLifecycleContextSet(state.extensions),
		sharedExtensions: cloneBridgeLifecycleContextSet(state.sharedExtensions),
		instances:        cloneBridgeLifecycleContextSet(state.instances),
	}
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if next.instances == nil {
			next.instances = make(map[string]struct{})
		}
		next.instances[trimmed] = struct{}{}
	}
	return context.WithValue(ctx, bridgeLifecycleContextKey{}, next)
}

func bridgeLifecycleStateFromContext(ctx context.Context) (bridgeLifecycleContextState, bool) {
	if ctx == nil {
		return bridgeLifecycleContextState{}, false
	}
	state, ok := ctx.Value(bridgeLifecycleContextKey{}).(bridgeLifecycleContextState)
	return state, ok
}

func cloneBridgeLifecycleContextSet(source map[string]struct{}) map[string]struct{} {
	if len(source) == 0 {
		return nil
	}

	cloned := make(map[string]struct{}, len(source))
	for key := range source {
		cloned[key] = struct{}{}
	}
	return cloned
}
