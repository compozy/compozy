package providerusage

import (
	"context"
	"maps"
	"slices"
	"sync"
)

// TargetCatalog provides atomic profile route replacement for configuration-driven composition.
type TargetCatalog struct {
	mu       sync.RWMutex
	profiles map[string][]Target
}

var _ TargetResolver = (*TargetCatalog)(nil)

func (c *TargetCatalog) Replace(profile string, targets []Target) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.profiles == nil {
		c.profiles = make(map[string][]Target)
	}
	c.profiles[profile] = cloneTargets(targets)
}
func (c *TargetCatalog) Targets(ctx context.Context, profile string) ([]Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneTargets(c.profiles[profile]), nil
}
func cloneTargets(targets []Target) []Target {
	result := slices.Clone(targets)
	for i := range result {
		result[i].Route.Env = slices.Clone(result[i].Route.Env)
		result[i].Route.BoundSecrets = maps.Clone(result[i].Route.BoundSecrets)
		result[i].Error = clonePointer(result[i].Error)
	}
	return result
}
