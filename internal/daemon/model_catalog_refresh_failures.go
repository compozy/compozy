package daemon

import (
	"context"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/modelcatalog"
)

type modelCatalogDiscoveryPreflighter interface {
	DiscoveryPreflight(ctx context.Context) error
}

// modelCatalogFailureCauses deduplicates warnings until the cause changes or recovers.
type modelCatalogFailureCauses struct {
	mu     sync.Mutex
	causes map[string]string
}

func modelCatalogFailureScope(opts modelcatalog.RefreshOptions) string {
	return strings.TrimSpace(opts.ProviderID) + "\x00" + strings.TrimSpace(opts.SourceID)
}

// record stores cause for scope and reports whether it differs from the last one.
func (c *modelCatalogFailureCauses) record(scope string, cause string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if previous, ok := c.causes[scope]; ok && previous == cause {
		return false
	}
	if c.causes == nil {
		c.causes = make(map[string]string)
	}
	c.causes[scope] = cause
	return true
}

func (c *modelCatalogFailureCauses) clear(scope string) {
	c.mu.Lock()
	delete(c.causes, scope)
	c.mu.Unlock()
}

// modelCatalogHeldScope keeps persisted failure baselines local to their execution context.
type modelCatalogHeldScope struct {
	target           modelCatalogRefreshTarget
	executionContext modelcatalog.CatalogExecutionContext
}

type modelCatalogHeldPreflights struct {
	mu        sync.Mutex
	baselines map[modelCatalogHeldScope]string
}

// changed compares against memory, or against persisted status after a cold restart.
func (h *modelCatalogHeldPreflights) changed(
	scope modelCatalogHeldScope,
	outcome string,
	persistedChanged bool,
) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	baseline, seen := h.baselines[scope]
	if h.baselines == nil {
		h.baselines = make(map[modelCatalogHeldScope]string)
	}
	h.baselines[scope] = outcome
	if seen {
		return baseline != outcome
	}
	return persistedChanged
}

// retain drops baselines for targets no longer held in any execution context.
func (h *modelCatalogHeldPreflights) retain(held map[modelCatalogHeldScope]struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for scope := range h.baselines {
		if _, ok := held[scope]; !ok {
			delete(h.baselines, scope)
		}
	}
}

// modelCatalogHoldSweep preflights each parked target once across execution contexts.
type modelCatalogHoldSweep struct {
	runtime *modelCatalogRuntime
	results map[modelCatalogRefreshTarget]error
	held    map[modelCatalogHeldScope]struct{}
}

func (r *modelCatalogRuntime) newHoldSweep() *modelCatalogHoldSweep {
	return &modelCatalogHoldSweep{
		runtime: r,
		results: make(map[modelCatalogRefreshTarget]error),
		held:    make(map[modelCatalogHeldScope]struct{}),
	}
}

// causeChanged leaves sources without a preflight parked until an explicit refresh.
func (s *modelCatalogHoldSweep) causeChanged(
	executionContext modelcatalog.CatalogExecutionContext,
	target modelCatalogRefreshTarget,
	held []modelcatalog.SourceStatus,
) bool {
	scope := modelCatalogHeldScope{target: target, executionContext: executionContext}
	s.held[scope] = struct{}{}
	preflighter := s.runtime.targetPreflighter(target)
	if preflighter == nil {
		return false
	}
	preflightErr, checked := s.results[target]
	if !checked {
		ctx, cancel := context.WithTimeout(s.runtime.ctx, s.runtime.timeout)
		preflightErr = preflighter.DiscoveryPreflight(ctx)
		cancel()
		s.results[target] = preflightErr
	}
	outcome := ""
	if preflightErr != nil {
		outcome = modelcatalog.RedactString(preflightErr.Error())
	}
	persistedChanged := false
	for _, status := range held {
		if !status.HeldCauseUnchanged(preflightErr) {
			persistedChanged = true
			break
		}
	}
	return s.runtime.heldPreflights.changed(scope, outcome, persistedChanged)
}

func (s *modelCatalogHoldSweep) finish() {
	s.runtime.heldPreflights.retain(s.held)
}

func (r *modelCatalogRuntime) targetPreflighter(target modelCatalogRefreshTarget) modelCatalogDiscoveryPreflighter {
	r.liveSourcesMu.RLock()
	defer r.liveSourcesMu.RUnlock()
	var source modelcatalog.Source
	if live := r.liveSources[target.providerID]; live != nil && live.ID() == target.sourceID {
		source = live
	} else {
		source = r.dynamicSources[target.sourceID]
	}
	preflighter, _ := source.(modelCatalogDiscoveryPreflighter)
	return preflighter
}
