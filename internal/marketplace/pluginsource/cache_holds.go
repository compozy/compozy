package pluginsource

import (
	"context"
	"sync"
)

type cacheHolds struct {
	mu       sync.Mutex
	readers  int
	sweeping bool
	idle     chan struct{}
}

// Hold keeps packages available until a refresh or installation publishes its references.
func (c *PackageCache) Hold(ctx context.Context) (func(), error) {
	if err := c.holds.acquire(ctx, false); err != nil {
		return nil, err
	}
	return func() { c.holds.release(false) }, nil
}

// A waiting sweep leaves admission open. Only a sweep that can start immediately excludes readers.
func (h *cacheHolds) acquire(ctx context.Context, sweep bool) error {
	for {
		h.mu.Lock()
		if err := ctx.Err(); err != nil {
			h.mu.Unlock()
			return err
		}
		if !h.sweeping && (!sweep || h.readers == 0) {
			if sweep {
				h.sweeping = true
			} else {
				h.readers++
			}
			h.mu.Unlock()
			return nil
		}
		if h.idle == nil {
			h.idle = make(chan struct{})
		}
		idle := h.idle
		h.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-idle:
		}
	}
}

func (h *cacheHolds) release(sweep bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if sweep {
		h.sweeping = false
	} else {
		h.readers--
	}
	if h.readers == 0 && h.idle != nil {
		close(h.idle)
		h.idle = nil
	}
}
