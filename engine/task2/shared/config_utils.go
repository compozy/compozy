package shared

import (
	"os"
	"strconv"
	"sync"
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

// GetConfigLimits returns the configuration limits from environment variables
// with fallback to default values
func GetConfigLimits() *ConfigLimits {
	limits := &ConfigLimits{
		MaxNestingDepth:  DefaultMaxParentDepth,
		MaxStringLength:  DefaultMaxStringLength,
		MaxContextDepth:  DefaultMaxContextDepth,
		MaxParentDepth:   DefaultMaxParentDepth,
		MaxChildrenDepth: DefaultMaxChildrenDepth,
		MaxConfigDepth:   DefaultMaxConfigDepth,
		MaxTemplateDepth: DefaultMaxTemplateDepth,
	}

	// Get MaxNestingDepth from environment (used by project config)
	if envValue := os.Getenv(EnvMaxNestingDepth); envValue != "" {
		if val, err := strconv.Atoi(envValue); err == nil && val > 0 {
			limits.MaxNestingDepth = val
			// Use the same limit for all depth-related configurations
			limits.MaxParentDepth = val
			limits.MaxChildrenDepth = val
			limits.MaxContextDepth = val
			limits.MaxConfigDepth = val
		}
	}

	// Get MaxStringLength from environment
	if envValue := os.Getenv(EnvMaxStringLength); envValue != "" {
		if val, err := strconv.Atoi(envValue); err == nil && val > 0 {
			limits.MaxStringLength = val
		}
	}

	// Get specific task context depth if set
	if envValue := os.Getenv(EnvMaxTaskContextDepth); envValue != "" {
		if val, err := strconv.Atoi(envValue); err == nil && val > 0 {
			limits.MaxContextDepth = val
		}
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
	globalConfigLimits = GetConfigLimits()
}

// resetGlobalConfigLimits resets the global configuration limits to nil (for testing)
func resetGlobalConfigLimits() {
	configLimitsMutex.Lock()
	defer configLimitsMutex.Unlock()
	globalConfigLimits = nil
}
