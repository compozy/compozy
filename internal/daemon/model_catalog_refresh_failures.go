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
	return strings.Join([]string{
		catalogExecutionScopeKey(opts.ExecutionContext),
		strings.TrimSpace(opts.ProviderID), strings.TrimSpace(opts.SourceID),
	}, "\x00")
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

// modelCatalogHoldSweep memoizes preflight; persisted status decides whether recovery remains due.
type modelCatalogHoldSweep struct {
	runtime *modelCatalogRuntime
	results map[modelCatalogRefreshTarget]error
}

func (r *modelCatalogRuntime) newHoldSweep() *modelCatalogHoldSweep {
	return &modelCatalogHoldSweep{
		runtime: r,
		results: make(map[modelCatalogRefreshTarget]error),
	}
}

// causeChanged leaves sources without a preflight parked until an explicit refresh.
func (s *modelCatalogHoldSweep) causeChanged(
	target modelCatalogRefreshTarget,
	held []modelcatalog.SourceStatus,
) bool {
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
	for _, status := range held {
		if !status.HeldCauseUnchanged(preflightErr) {
			return true
		}
	}
	return false
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
