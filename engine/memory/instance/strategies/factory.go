package strategies

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/memory/core"
)

// StrategyFactory creates flush strategies based on configuration
type StrategyFactory struct {
	strategies       map[core.FlushingStrategyType]StrategyConstructor
	coreTokenCounter core.TokenCounter // System-level token counter for dependency injection
}

// StrategyConstructor is a function type for creating strategies
type StrategyConstructor func(*core.FlushingStrategyConfig, *StrategyOptions) (core.FlushStrategy, error)

// StrategyOptions contains options for strategy construction
type StrategyOptions struct {
	// CacheSize for LRU-based strategies
	CacheSize int
	// MaxCacheSize enforces maximum cache size to prevent excessive memory usage
	MaxCacheSize int
	// MaxTokens for token-aware strategies
	MaxTokens int
	// DefaultThreshold for strategies that support thresholds
	DefaultThreshold float64

	// LRU Strategy configurable options
	LRUTargetCapacityPercent float64 // Target capacity after flush (default: 0.5)
	LRUMinFlushPercent       float64 // Minimum flush percentage (default: 0.25)

	// Token-aware LRU configurable options
	TokenLRUTargetCapacityPercent float64 // Target capacity after flush (default: 0.5)

	// Priority-based strategy configurable options
	PriorityTargetCapacityPercent float64 // Target capacity after flush (default: 0.6)
	PriorityConservativePercent   float64 // Conservative fallback percentage (default: 0.75)
	PriorityRecentThreshold       float64 // Recent message threshold (default: 0.8)
	PriorityMaxFlushRatio         float64 // Maximum flush ratio (default: 0.33)

	// Token estimation strategy for fallback counting
	TokenEstimationStrategy core.TokenEstimationStrategy // Default: EnglishEstimation
}

// NewStrategyFactory creates a new strategy factory with all registered strategies
func NewStrategyFactory() *StrategyFactory {
	return NewStrategyFactoryWithTokenCounter(nil)
}

// NewStrategyFactoryWithTokenCounter creates a new strategy factory with a core token counter
func NewStrategyFactoryWithTokenCounter(coreTokenCounter core.TokenCounter) *StrategyFactory {
	factory := &StrategyFactory{
		strategies:       make(map[core.FlushingStrategyType]StrategyConstructor),
		coreTokenCounter: coreTokenCounter,
	}
	factory.registerDefaultStrategies()
	return factory
}

// registerDefaultStrategies registers all built-in strategies
func (f *StrategyFactory) registerDefaultStrategies() {
	fifoConstructor := func(config *core.FlushingStrategyConfig, opts *StrategyOptions) (core.FlushStrategy, error) {
		threshold := 0.8
		if config != nil && config.SummarizeThreshold > 0 {
			threshold = config.SummarizeThreshold
		} else if opts != nil && opts.DefaultThreshold > 0 {
			threshold = opts.DefaultThreshold
		}

		if f.coreTokenCounter != nil {
			return NewFIFOStrategyWithTokenCounter(threshold, f.coreTokenCounter), nil
		}
		return NewFIFOStrategy(threshold), nil
	}
	f.Register(core.SimpleFIFOFlushing, fifoConstructor)
	f.Register(core.FIFOFlushing, fifoConstructor) // Register alias
	f.Register(
		core.LRUFlushing,
		func(config *core.FlushingStrategyConfig, opts *StrategyOptions) (core.FlushStrategy, error) {
			return NewLRUStrategy(config, opts)
		},
	)
	f.Register(
		core.TokenAwareLRUFlushing,
		func(config *core.FlushingStrategyConfig, opts *StrategyOptions) (core.FlushStrategy, error) {
			return NewTokenAwareLRUStrategy(config, opts)
		},
	)
}

// Register adds a new strategy constructor to the factory
func (f *StrategyFactory) Register(strategyType core.FlushingStrategyType, constructor StrategyConstructor) {
	f.strategies[strategyType] = constructor
}

// CreateStrategy creates a flush strategy based on the configuration
func (f *StrategyFactory) CreateStrategy(
	config *core.FlushingStrategyConfig,
	opts *StrategyOptions,
) (core.FlushStrategy, error) {
	if config == nil {
		config = &core.FlushingStrategyConfig{
			Type: core.SimpleFIFOFlushing,
		}
	}
	constructor, exists := f.strategies[config.Type]
	if !exists {
		return nil, fmt.Errorf("unknown flush strategy type: %s", config.Type)
	}
	strategy, err := constructor(config, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s strategy: %w", config.Type, err)
	}
	return strategy, nil
}

// GetAvailableStrategies returns a list of all registered strategy types
func (f *StrategyFactory) GetAvailableStrategies() []core.FlushingStrategyType {
	strategies := make([]core.FlushingStrategyType, 0, len(f.strategies))
	for strategyType := range f.strategies {
		strategies = append(strategies, strategyType)
	}
	return strategies
}

// IsStrategySupported checks if a strategy type is supported
func (f *StrategyFactory) IsStrategySupported(strategyType core.FlushingStrategyType) bool {
	_, exists := f.strategies[strategyType]
	return exists
}

// CreateDefaultStrategy creates a default FIFO strategy
func (f *StrategyFactory) CreateDefaultStrategy() (core.FlushStrategy, error) {
	return f.CreateStrategy(&core.FlushingStrategyConfig{
		Type: core.SimpleFIFOFlushing,
	}, &StrategyOptions{
		DefaultThreshold: 0.8,
	})
}

// GetDefaultStrategyOptions returns default options for strategy creation
func GetDefaultStrategyOptions() *StrategyOptions {
	return &StrategyOptions{
		CacheSize:        1000,
		MaxCacheSize:     10000, // Default maximum cache size
		MaxTokens:        4000,
		DefaultThreshold: 0.8,

		LRUTargetCapacityPercent: 0.5,  // Reduce to 50% capacity after flush
		LRUMinFlushPercent:       0.25, // Minimum 25% flush when no specific limits

		TokenLRUTargetCapacityPercent: 0.5, // Target 50% of max capacity

		PriorityTargetCapacityPercent: 0.6,  // Target 60% capacity (more conservative)
		PriorityConservativePercent:   0.75, // Keep 75% when no limits (fallback)
		PriorityRecentThreshold:       0.8,  // Last 20% of messages are "recent"
		PriorityMaxFlushRatio:         0.33, // Never flush more than 1/3 of messages
	}
}

// ValidateStrategyConfig validates a strategy configuration
func (f *StrategyFactory) ValidateStrategyConfig(config *core.FlushingStrategyConfig) error {
	if config == nil {
		return fmt.Errorf("strategy config cannot be nil")
	}
	if !config.Type.IsValid() {
		return fmt.Errorf("invalid strategy type: %s", config.Type)
	}
	if !f.IsStrategySupported(config.Type) {
		return fmt.Errorf("strategy type %s is not supported by this factory", config.Type)
	}
	if config.SummarizeThreshold != 0 && (config.SummarizeThreshold <= 0 || config.SummarizeThreshold > 1) {
		return fmt.Errorf("summarize threshold must be between 0 and 1, got %f", config.SummarizeThreshold)
	}
	return nil
}

// ValidateStrategyType validates a string strategy type.
// It checks if the strategy type is valid and supported by the factory.
// An empty string is considered valid and will use the default strategy.
func (f *StrategyFactory) ValidateStrategyType(strategyType string) error {
	if strategyType == "" {
		return nil
	}
	flushType := core.FlushingStrategyType(strategyType)
	if !flushType.IsValid() {
		return fmt.Errorf("invalid strategy type: %s", strategyType)
	}
	if !f.IsStrategySupported(flushType) {
		return fmt.Errorf("strategy type %s is not supported", strategyType)
	}
	return nil
}

// GetSupportedStrategies returns all valid strategy types as strings for API use.
// The strategies are returned in alphabetical order for consistency.
func (f *StrategyFactory) GetSupportedStrategies() []string {
	strategies := make([]string, 0, len(f.strategies))
	for strategyType := range f.strategies {
		strategies = append(strategies, string(strategyType))
	}
	sort.Strings(strategies)
	return strategies
}

// IsValidStrategy is a convenience method for validation.
// It returns true if the strategy type is valid and supported, false otherwise.
func (f *StrategyFactory) IsValidStrategy(strategyType string) bool {
	return f.ValidateStrategyType(strategyType) == nil
}
