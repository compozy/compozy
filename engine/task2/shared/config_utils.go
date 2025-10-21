package shared

import (
	"context"
	"sync"

	"github.com/compozy/compozy/pkg/config"
)

// ConfigLimits holds configurable limits for the task2 engine
type ConfigLimits struct {
	MaxNestingDepth  int
	MaxStringLength  int
	MaxContextDepth  int
	MaxParentDepth   int
	MaxChildrenDepth int
	MaxConfigDepth   int
	MaxTemplateDepth int
}

// ConfigProvider defines the interface for accessing configuration
type ConfigProvider interface {
	GetConfig(ctx context.Context) *config.Config
}

// defaultConfigProvider loads configuration lazily using a singleton pattern
type defaultConfigProvider struct {
	config     *config.Config
	configOnce sync.Once
	configErr  error
}

// GetConfig returns the configuration, loading it once on first access
func (p *defaultConfigProvider) GetConfig(ctx context.Context) *config.Config {
	p.configOnce.Do(func() {
		service := config.NewService()
		p.config, p.configErr = service.Load(ctx)
		if p.configErr != nil {
			// TODO: Inject logger for proper error logging
			p.config = config.Default()
		}
	})
	return p.config
}

// Error returns the error encountered during configuration loading, if any
func (p *defaultConfigProvider) Error(ctx context.Context) error {
	_ = p.GetConfig(ctx)
	return p.configErr
}

// Global default config provider instance
var globalConfigProvider = &defaultConfigProvider{}

// GetConfigLimits returns the configuration limits from pkg/config
// with fallback to default values. This loads configuration from all sources
// including environment variables.
func GetConfigLimits(ctx context.Context) *ConfigLimits {
	return GetConfigLimitsWithConfig(ctx, nil)
}

// GetConfigLimitsWithConfig returns the configuration limits using the provided config
// or loads from all sources if config is nil
func GetConfigLimitsWithConfig(ctx context.Context, appConfig *config.Config) *ConfigLimits {
	limits := &ConfigLimits{
		MaxNestingDepth:  DefaultMaxParentDepth,
		MaxStringLength:  DefaultMaxStringLength,
		MaxContextDepth:  DefaultMaxContextDepth,
		MaxParentDepth:   DefaultMaxParentDepth,
		MaxChildrenDepth: DefaultMaxChildrenDepth,
		MaxConfigDepth:   DefaultMaxConfigDepth,
		MaxTemplateDepth: DefaultMaxTemplateDepth,
	}
	if appConfig == nil {
		appConfig = globalConfigProvider.GetConfig(ctx)
	}
	if appConfig.Limits.MaxNestingDepth > 0 {
		limits.MaxNestingDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxParentDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxChildrenDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxContextDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxConfigDepth = appConfig.Limits.MaxNestingDepth
	}
	if appConfig.Limits.MaxStringLength > 0 {
		limits.MaxStringLength = appConfig.Limits.MaxStringLength
	}
	if appConfig.Limits.MaxTaskContextDepth > 0 {
		limits.MaxContextDepth = appConfig.Limits.MaxTaskContextDepth
	}
	return limits
}

// Global config limits instance
var (
	globalConfigLimits *ConfigLimits
	configLimitsMutex  sync.RWMutex
)

// GetGlobalConfigLimits returns the singleton instance of configuration limits
func GetGlobalConfigLimits(ctx context.Context) *ConfigLimits {
	configLimitsMutex.RLock()
	if globalConfigLimits != nil {
		defer configLimitsMutex.RUnlock()
		return globalConfigLimits
	}
	configLimitsMutex.RUnlock()
	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	if globalConfigLimits == nil {
		globalConfigLimits = GetConfigLimits(ctx)
	}
	return globalConfigLimits
}

// RefreshGlobalConfigLimits refreshes the global configuration limits from environment
func RefreshGlobalConfigLimits(ctx context.Context) {
	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	globalConfigProvider = &defaultConfigProvider{}
	globalConfigLimits = GetConfigLimits(ctx)
}

// resetGlobalConfigLimits resets the global configuration limits to nil (for testing)
func resetGlobalConfigLimits() {
	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	globalConfigProvider = &defaultConfigProvider{}
	globalConfigLimits = nil
}
