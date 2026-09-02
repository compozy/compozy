package daemon

import (
	"context"
	"maps"
	"slices"
	"sync"
)

type modelCatalogRefreshCancels struct {
	mu      sync.Mutex
	nextID  uint64
	stopped bool
	active  map[uint64]context.CancelFunc
}

func (c *modelCatalogRefreshCancels) register(cancel context.CancelFunc) func() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		cancel()
		return func() {}
	}
	if c.active == nil {
		c.active = make(map[uint64]context.CancelFunc)
	}
	id := c.nextID
	c.nextID++
	c.active[id] = cancel
	c.mu.Unlock()

	return sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.active, id)
	})
}

func (c *modelCatalogRefreshCancels) stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	cancels := slices.Collect(maps.Values(c.active))
	clear(c.active)
	c.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}
