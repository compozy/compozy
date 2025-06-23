package memory

import (
	"fmt"

	"github.com/compozy/compozy/engine/memory/tokens"
	"github.com/dgraph-io/ristretto"
)

// ComponentCacheConfig holds configuration for the component cache
type ComponentCacheConfig struct {
	MaxCost     int64 // Maximum cost of cache (approximately memory in bytes)
	NumCounters int64 // Number of counters for tracking frequency
	BufferItems int64 // Number of keys per Get buffer
}

// DefaultComponentCacheConfig returns sensible defaults for component caching
func DefaultComponentCacheConfig() ComponentCacheConfig {
	return ComponentCacheConfig{
		MaxCost:     50 << 20, // 50 MB (smaller than ref evaluator)
		NumCounters: 1e6,      // 1 million (fewer than ref evaluator)
		BufferItems: 64,       // Standard buffer size
	}
}

// CacheableComponent represents a component that can be cached
type CacheableComponent interface {
	// GetCacheKey returns a unique key for this component
	GetCacheKey() string
	// EstimateCost returns the approximate memory cost of this component
	EstimateCost() int64
}

// componentCache wraps a ristretto cache for caching stateless components
type componentCache struct {
	cache *ristretto.Cache[string, CacheableComponent]
}

// newComponentCache creates a new component cache with the given configuration
func newComponentCache(config ComponentCacheConfig) (*componentCache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config[string, CacheableComponent]{
		NumCounters: config.NumCounters,
		MaxCost:     config.MaxCost,
		BufferItems: config.BufferItems,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}
	return &componentCache{cache: cache}, nil
}

// Get retrieves a component from the cache
func (cc *componentCache) Get(key string) (CacheableComponent, bool) {
	if cc == nil || cc.cache == nil {
		return nil, false
	}
	value, found := cc.cache.Get(key)
	return value, found
}

// Set stores a component in the cache
func (cc *componentCache) Set(key string, component CacheableComponent) {
	if cc == nil || cc.cache == nil {
		return
	}
	_ = cc.cache.Set(key, component, component.EstimateCost())
}

// CacheableTiktokenCounter wraps TiktokenCounter to make it cacheable
type CacheableTiktokenCounter struct {
	*tokens.TiktokenCounter
	model string
}

// NewCacheableTiktokenCounter creates a new cacheable token counter
func NewCacheableTiktokenCounter(model string) (*CacheableTiktokenCounter, error) {
	counter, err := tokens.NewTiktokenCounter(model)
	if err != nil {
		return nil, err
	}
	return &CacheableTiktokenCounter{
		TiktokenCounter: counter,
		model:           model,
	}, nil
}

// GetCacheKey returns a unique cache key for this token counter
func (ctc *CacheableTiktokenCounter) GetCacheKey() string {
	return fmt.Sprintf("token-counter:%s", ctc.model)
}

// EstimateCost returns the approximate memory cost of this token counter
func (ctc *CacheableTiktokenCounter) EstimateCost() int64 {
	// TiktokenCounter is relatively lightweight, mainly contains encoding tables
	// Estimate around 1MB for the encoding data and associated structures
	return 1 << 20 // 1 MB
}
