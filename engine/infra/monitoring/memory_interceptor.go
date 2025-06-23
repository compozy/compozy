package monitoring

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/metric"
)

// MemoryMonitoringInterceptor wraps memory operations with monitoring
type MemoryMonitoringInterceptor struct {
	*memory.Manager
}

// NewMemoryMonitoringInterceptor creates a new memory monitoring interceptor
func NewMemoryMonitoringInterceptor(manager *memory.Manager) *MemoryMonitoringInterceptor {
	return &MemoryMonitoringInterceptor{
		Manager: manager,
	}
}

// GetInstance wraps the GetInstance method with monitoring
func (m *MemoryMonitoringInterceptor) GetInstance(
	ctx context.Context,
	ref core.MemoryReference,
	workflowContext map[string]any,
) (memory.Memory, error) {
	start := time.Now()
	projectID := getProjectIDFromContext(ctx)

	instance, err := m.Manager.GetInstance(ctx, ref, workflowContext)

	duration := time.Since(start)
	memory.RecordMemoryOp(ctx, "get_instance", ref.ID, projectID, duration)

	if err != nil {
		memory.UpdateHealthState(ref.ID, false, 1)
	} else {
		memory.UpdateHealthState(ref.ID, true, 0)
	}

	return instance, err
}

// InitializeMemoryMonitoring initializes memory-specific monitoring
func InitializeMemoryMonitoring(ctx context.Context, meter metric.Meter) {
	memory.InitMemoryMetrics(ctx, meter)
	log := logger.FromContext(ctx)
	log.Info("Memory monitoring initialized")
}

const defaultProjectID = "unknown"

// getProjectIDFromContext extracts project ID from context
func getProjectIDFromContext(ctx context.Context) string {
	if projectName, err := core.GetProjectName(ctx); err == nil {
		return projectName
	}
	return defaultProjectID
}
