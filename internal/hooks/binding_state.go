package hooks

import (
	"errors"
	"fmt"
)

// BindingState captures one fully-built hook registry snapshot before it is
// atomically swapped into the live runtime.
type BindingState struct {
	snapshot    map[HookEvent][]*ResolvedHook
	fingerprint string
	hookCount   int
}

// HookCount reports how many resolved hooks the binding state contains.
func (s *BindingState) HookCount() int {
	if s == nil {
		return 0
	}
	return s.hookCount
}

// BuildBindingState validates declarations, binds executors, and computes the
// next registry snapshot without mutating the live runtime.
func (h *Hooks) BuildBindingState(decls []HookDecl) (*BindingState, error) {
	if h == nil {
		return nil, errors.New("hooks: dispatcher is required")
	}

	resolved, err := NormalizeHookDecls(decls, h.resolveExecutor)
	if err != nil {
		return nil, err
	}

	snapshot := buildHookSnapshot(resolved)
	fingerprint, err := fingerprintHookSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	return &BindingState{
		snapshot:    snapshot,
		fingerprint: fingerprint,
		hookCount:   countResolvedHooks(snapshot),
	}, nil
}

// ApplyBindingState atomically swaps a previously-built binding snapshot into
// the live runtime.
func (h *Hooks) ApplyBindingState(state *BindingState, resourceRevision int64) error {
	if h == nil {
		return errors.New("hooks: dispatcher is required")
	}
	if state == nil {
		return errors.New("hooks: binding state is required")
	}
	if resourceRevision < 0 {
		return fmt.Errorf("hooks: resource revision cannot be negative: %d", resourceRevision)
	}

	reloadStarted := h.now()

	h.mu.Lock()
	if state.fingerprint == h.fingerprint {
		h.mu.Unlock()
		return nil
	}

	oldHookCount := countResolvedHooks(h.snapshot)
	h.snapshot = state.snapshot
	h.fingerprint = state.fingerprint
	version := h.version.Add(1)
	h.mu.Unlock()

	reloadDuration := h.now().Sub(reloadStarted)
	h.metrics.observeRegistryReload(reloadDuration, state.hookCount-oldHookCount)
	h.logger.Info(
		"hook.registry.projected",
		"version", version,
		"resource_revision", resourceRevision,
		"hook_count", state.hookCount,
		"hook_count_delta", state.hookCount-oldHookCount,
		"duration_ms", reloadDuration.Milliseconds(),
	)

	return nil
}
