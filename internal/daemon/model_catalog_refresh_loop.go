package daemon

import "time"

const modelCatalogLiveRefreshSweepInterval = 30 * time.Second

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

func (r *modelCatalogRuntime) startLiveRefreshLoop() {
	if r == nil || len(r.liveSources) == 0 || r.workers == nil {
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
			ticker := newTicker(modelCatalogLiveRefreshSweepInterval)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C():
					r.refreshLiveSourcesInBackground()
				}
			}
		}()
	})
}
