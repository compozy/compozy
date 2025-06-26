package instance

import (
	"context"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

type defaultMetrics struct {
	logger logger.Logger
}

func NewDefaultMetrics(log logger.Logger) Metrics {
	return &defaultMetrics{
		logger: log,
	}
}

// RecordAppend is a no-op implementation for default metrics
func (m *defaultMetrics) RecordAppend(_ context.Context, _ time.Duration, _ int, _ error) {
	// No-op implementation for default metrics
}

// RecordRead is a no-op implementation for default metrics
func (m *defaultMetrics) RecordRead(_ context.Context, _ time.Duration, _ int, _ error) {
	// No-op implementation for default metrics
}

// RecordFlush is a no-op implementation for default metrics
func (m *defaultMetrics) RecordFlush(_ context.Context, _ time.Duration, _ int, _ error) {
	// No-op implementation for default metrics
}

// RecordScheduledFlush is a no-op implementation for default metrics
func (m *defaultMetrics) RecordScheduledFlush(_ context.Context, _ bool, _ error) {
	// No-op implementation for default metrics
}

// RecordTransition is a no-op implementation for default metrics
func (m *defaultMetrics) RecordTransition(_ context.Context, _ string, _ string) {
	// No-op implementation for default metrics
}

// RecordTokenCount is a no-op implementation for default metrics
func (m *defaultMetrics) RecordTokenCount(_ context.Context, _ int) {
	// No-op implementation for default metrics
}

// IncrementTokenCount is a no-op implementation for default metrics
func (m *defaultMetrics) IncrementTokenCount(_ context.Context, _ int) {
	// No-op implementation for default metrics
}

// RecordMessageCount is a no-op implementation for default metrics
func (m *defaultMetrics) RecordMessageCount(_ context.Context, _ int) {
	// No-op implementation for default metrics
}
