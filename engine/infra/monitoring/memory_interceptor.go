package monitoring

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/metrics"
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
) (memcore.Memory, error) {
	start := time.Now()
	projectID := getProjectIDFromContext(ctx)

	instance, err := m.Manager.GetInstance(ctx, ref, workflowContext)

	duration := time.Since(start)
	metrics.RecordMemoryOp(ctx, ref.ID, projectID, "get_instance", duration, 0, err)

	if err != nil {
		metrics.UpdateHealthState(ref.ID, false, 1)
	} else {
		metrics.UpdateHealthState(ref.ID, true, 0)
	}

	return instance, err
}

// InitializeMemoryMonitoring initializes memory-specific monitoring
func InitializeMemoryMonitoring(ctx context.Context, meter metric.Meter) {
	metrics.InitMemoryMetrics(ctx, meter)
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
