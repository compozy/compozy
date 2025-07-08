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
	GetConfig() *config.Config
}

// defaultConfigProvider loads configuration lazily using a singleton pattern
type defaultConfigProvider struct {
	config     *config.Config
	configOnce sync.Once
	configErr  error
}

// GetConfig returns the configuration, loading it once on first access
func (p *defaultConfigProvider) GetConfig() *config.Config {
	p.configOnce.Do(func() {
		service := config.NewService()
		ctx := context.Background()
		p.config, p.configErr = service.Load(ctx)
		if p.configErr != nil {
			// TODO: Inject logger for proper error logging
			// log := logger.FromContext(ctx)
			// log.Error("Failed to load configuration, using defaults", "error", p.configErr)
			// Fallback to defaults if loading fails
			p.config = config.Default()
		}
	})
	return p.config
}

// Error returns the error encountered during configuration loading, if any
func (p *defaultConfigProvider) Error() error {
	// Ensure config has been loaded before returning error
	_ = p.GetConfig()
	return p.configErr
}

// Global default config provider instance
var globalConfigProvider = &defaultConfigProvider{}

// GetConfigLimits returns the configuration limits from pkg/config
// with fallback to default values. This loads configuration from all sources
// including environment variables.
func GetConfigLimits() *ConfigLimits {
	return GetConfigLimitsWithConfig(nil)
}

// GetConfigLimitsWithConfig returns the configuration limits using the provided config
// or loads from all sources if config is nil
func GetConfigLimitsWithConfig(appConfig *config.Config) *ConfigLimits {
	limits := &ConfigLimits{
		MaxNestingDepth:  DefaultMaxParentDepth,
		MaxStringLength:  DefaultMaxStringLength,
		MaxContextDepth:  DefaultMaxContextDepth,
		MaxParentDepth:   DefaultMaxParentDepth,
		MaxChildrenDepth: DefaultMaxChildrenDepth,
		MaxConfigDepth:   DefaultMaxConfigDepth,
		MaxTemplateDepth: DefaultMaxTemplateDepth,
	}

	// Use provided config or get from global provider
	if appConfig == nil {
		appConfig = globalConfigProvider.GetConfig()
	}

	// Use MaxNestingDepth from config (used by project config)
	if appConfig.Limits.MaxNestingDepth > 0 {
		limits.MaxNestingDepth = appConfig.Limits.MaxNestingDepth
		// Use the same limit for all depth-related configurations
		limits.MaxParentDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxChildrenDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxContextDepth = appConfig.Limits.MaxNestingDepth
		limits.MaxConfigDepth = appConfig.Limits.MaxNestingDepth
	}

	// Use MaxStringLength from config
	if appConfig.Limits.MaxStringLength > 0 {
		limits.MaxStringLength = appConfig.Limits.MaxStringLength
	}

	// Use specific task context depth if set
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
func GetGlobalConfigLimits() *ConfigLimits {
	configLimitsMutex.RLock()
	if globalConfigLimits != nil {
		defer configLimitsMutex.RUnlock()
		return globalConfigLimits
	}
	configLimitsMutex.RUnlock()

	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	// Double-check after acquiring write lock
	if globalConfigLimits == nil {
		globalConfigLimits = GetConfigLimits()
	}
	return globalConfigLimits
}

// RefreshGlobalConfigLimits refreshes the global configuration limits from environment
func RefreshGlobalConfigLimits() {
	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	// Reset the provider to force reload on next access
	globalConfigProvider = &defaultConfigProvider{}
	globalConfigLimits = GetConfigLimits()
}

// resetGlobalConfigLimits resets the global configuration limits to nil (for testing)
func resetGlobalConfigLimits() {
	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	// Reset both the provider and limits
	globalConfigProvider = &defaultConfigProvider{}
	globalConfigLimits = nil
}
