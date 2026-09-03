package daemon

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
)

const modelCatalogDynamicRefreshSweepInterval = 30 * time.Second

type modelCatalogRefreshTarget struct {
	sourceID   string
	providerID string
}

func (t modelCatalogRefreshTarget) requestID() string {
	id := strings.TrimSpace(t.sourceID)
	if providerID := strings.TrimSpace(t.providerID); providerID != "" {
		id += "-" + providerID
	}
	return "model-catalog-background-" + id
}

type modelCatalogRefreshTicker interface {
	C() <-chan time.Time
	Stop()
}

type realModelCatalogRefreshTicker struct {
	*time.Ticker
}

func newRealModelCatalogRefreshTicker(interval time.Duration) modelCatalogRefreshTicker {
	return &realModelCatalogRefreshTicker{Ticker: time.NewTicker(interval)}
}

func (t *realModelCatalogRefreshTicker) C() <-chan time.Time {
	return t.Ticker.C
}

func (r *modelCatalogRuntime) startDynamicRefreshLoop() {
	if r == nil || len(r.dynamicRefreshTargets()) == 0 || r.workers == nil {
		return
	}
	r.refreshLoopOnce.Do(func() {
		complete, admitted := r.workers.Begin()
		if !admitted {
			return
		}
		newTicker := r.newRefreshTicker
		if newTicker == nil {
			newTicker = newRealModelCatalogRefreshTicker
		}
		go func() {
			defer complete()
			ticker := newTicker(modelCatalogDynamicRefreshSweepInterval)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C():
					r.refreshDynamicSourcesInBackground()
				}
			}
		}()
	})
}

func (r *modelCatalogRuntime) dynamicRefreshTargets() []modelCatalogRefreshTarget {
	if r == nil {
		return nil
	}
	sources := make(map[string]modelcatalog.Source, len(r.dynamicSources)+len(r.liveSources))
	maps.Copy(sources, r.dynamicSources)
	for _, source := range r.liveSources {
		if source != nil {
			sources[source.ID()] = source
		}
	}
	targets := make([]modelCatalogRefreshTarget, 0, len(sources))
	for sourceID, source := range sources {
		providers := []string(nil)
		if providerSource, ok := source.(interface{ ProviderIDs() []string }); ok {
			providers = providerSource.ProviderIDs()
		}
		if len(providers) == 0 {
			targets = append(targets, modelCatalogRefreshTarget{sourceID: sourceID})
			continue
		}
		for _, providerID := range providers {
			targets = append(targets, modelCatalogRefreshTarget{
				sourceID:   sourceID,
				providerID: strings.TrimSpace(providerID),
			})
		}
	}
	slices.SortFunc(targets, func(left, right modelCatalogRefreshTarget) int {
		if sourceOrder := cmp.Compare(left.sourceID, right.sourceID); sourceOrder != 0 {
			return sourceOrder
		}
		return cmp.Compare(left.providerID, right.providerID)
	})
	return targets
}

func (r *modelCatalogRuntime) refreshDynamicSourcesInBackground() {
	targets := r.dynamicRefreshTargets()
	if len(targets) == 0 {
		return
	}
	if !r.beginBackgroundRefresh() {
		return
	}
	complete, admitted := r.workers.Begin()
	if !admitted {
		r.finishBackgroundRefresh()
		return
	}
	go func() {
		defer complete()
		defer r.finishBackgroundRefresh()
		var refreshes sync.WaitGroup
		for _, executionContext := range r.catalogExecutionContexts() {
			for _, target := range targets {
				refreshes.Go(func() {
					if _, err := r.Refresh(r.ctx, modelcatalog.RefreshOptions{
						ProviderID:       target.providerID,
						SourceID:         target.sourceID,
						ExecutionContext: executionContext,
						RequestID:        target.requestID(),
					}); err != nil {
						// Refresh owns source-failure logging; the background cycle has no synchronous caller.
						return
					}
				})
			}
		}
		refreshes.Wait()
	}()
}

func (r *modelCatalogRuntime) beginBackgroundRefresh() bool {
	r.backgroundRefreshMu.Lock()
	defer r.backgroundRefreshMu.Unlock()
	if r.backgroundRefreshRunning {
		return false
	}
	r.backgroundRefreshRunning = true
	return true
}

func (r *modelCatalogRuntime) finishBackgroundRefresh() {
	r.backgroundRefreshMu.Lock()
	r.backgroundRefreshRunning = false
	r.backgroundRefreshMu.Unlock()
}
