package memory

import (
	"context"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
)

var (
	globalHealthService *HealthService
	healthServiceMu     sync.RWMutex
	healthServiceOnce   sync.Once
)

// InitializeGlobalHealthService initializes the global memory health service
func InitializeGlobalHealthService(manager *Manager, log logger.Logger) *HealthService {
	healthServiceOnce.Do(func() {
		healthServiceMu.Lock()
		defer healthServiceMu.Unlock()

		globalHealthService = NewHealthService(manager, log)
	})

	return globalHealthService
}

// GetGlobalHealthService returns the global memory health service
func GetGlobalHealthService() *HealthService {
	healthServiceMu.RLock()
	defer healthServiceMu.RUnlock()

	return globalHealthService
}

// StartGlobalHealthService starts the global health service if it exists
func StartGlobalHealthService(ctx context.Context) {
	healthServiceMu.RLock()
	service := globalHealthService
	healthServiceMu.RUnlock()

	if service != nil {
		service.Start(ctx)
	}
}

// StopGlobalHealthService stops the global health service if it exists
func StopGlobalHealthService() {
	healthServiceMu.RLock()
	service := globalHealthService
	healthServiceMu.RUnlock()

	if service != nil {
		service.Stop()
	}
}

// RegisterInstanceGlobally registers a memory instance with the global health service
func RegisterInstanceGlobally(memoryID string) {
	healthServiceMu.RLock()
	service := globalHealthService
	healthServiceMu.RUnlock()

	if service != nil {
		service.RegisterInstance(memoryID)
	}
}

// UnregisterInstanceGlobally unregisters a memory instance from the global health service
func UnregisterInstanceGlobally(memoryID string) {
	healthServiceMu.RLock()
	service := globalHealthService
	healthServiceMu.RUnlock()

	if service != nil {
		service.UnregisterInstance(memoryID)
	}
}

// ResetGlobalHealthServiceForTesting resets the global health service for testing
func ResetGlobalHealthServiceForTesting() {
	healthServiceMu.Lock()
	defer healthServiceMu.Unlock()

	if globalHealthService != nil {
		globalHealthService.Stop()
	}

	globalHealthService = nil
	healthServiceOnce = sync.Once{}
}
