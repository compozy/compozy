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
	wrapped *memory.Manager
}

// NewMemoryMonitoringInterceptor creates a new memory monitoring interceptor
func NewMemoryMonitoringInterceptor(manager *memory.Manager) *MemoryMonitoringInterceptor {
	return &MemoryMonitoringInterceptor{
		wrapped: manager,
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

	instance, err := m.wrapped.GetInstance(ctx, ref, workflowContext)

	duration := time.Since(start)
	memory.RecordMemoryOperation(ctx, "get_instance", ref.ID, projectID, duration)

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
	// This is a placeholder - in real implementation, extract from context
	// based on your application's context structure
	if projectID, ok := ctx.Value("project_id").(string); ok {
		return projectID
	}
	return defaultProjectID
}
