package memory

import (
	"fmt"

	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/compozy/compozy/pkg/logger"
)

// validateManagerOptions validates the required manager options
func validateManagerOptions(opts *ManagerOptions) error {
	if opts.ResourceRegistry == nil {
		return fmt.Errorf("resource registry cannot be nil")
	}
	if opts.TplEngine == nil {
		return fmt.Errorf("template engine cannot be nil")
	}
	if opts.BaseLockManager == nil {
		return fmt.Errorf("base lock manager cannot be nil")
	}
	if opts.BaseRedisClient == nil {
		return fmt.Errorf("base redis client cannot be nil")
	}
	if opts.TemporalClient == nil {
		return fmt.Errorf("temporal client cannot be nil")
	}
	return nil
}

// setDefaultManagerOptions sets default values for optional manager options
func setDefaultManagerOptions(opts *ManagerOptions) {
	if opts.TemporalTaskQueue == "" {
		opts.TemporalTaskQueue = "memory-operations"
	}
	if opts.Logger == nil {
		opts.Logger = logger.NewForTests()
	}
}

// getOrCreatePrivacyManager gets existing privacy manager or creates a new one
func getOrCreatePrivacyManager(existing *privacy.Manager) *privacy.Manager {
	if existing != nil {
		return existing
	}
	return privacy.NewManager()
}

// createComponentCache creates a component cache if not disabled
func createComponentCache(opts *ManagerOptions) *componentCache {
	if opts.DisableComponentCache {
		return nil
	}
	cacheConfig := opts.ComponentCacheConfig
	if cacheConfig == nil {
		defaultConfig := DefaultComponentCacheConfig()
		cacheConfig = &defaultConfig
	}
	cache, err := newComponentCache(*cacheConfig)
	if err != nil {
		opts.Logger.Warn("Failed to create component cache, proceeding without cache", "error", err)
		return nil
	}
	return cache
}
