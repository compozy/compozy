package instance

import (
	"context"
	"time"
)

type defaultMetrics struct {
}

func NewDefaultMetrics() Metrics {
	return &defaultMetrics{}
}

// RecordAppend is a no-op implementation for default metrics
func (m *defaultMetrics) RecordAppend(_ context.Context, _ time.Duration, _ int, _ error) {
}

// RecordRead is a no-op implementation for default metrics
func (m *defaultMetrics) RecordRead(_ context.Context, _ time.Duration, _ int, _ error) {
}

// RecordFlush is a no-op implementation for default metrics
func (m *defaultMetrics) RecordFlush(_ context.Context, _ time.Duration, _ int, _ error) {
}

// RecordScheduledFlush is a no-op implementation for default metrics
func (m *defaultMetrics) RecordScheduledFlush(_ context.Context, _ bool, _ error) {
}

// RecordTransition is a no-op implementation for default metrics
func (m *defaultMetrics) RecordTransition(_ context.Context, _ string, _ string) {
}

// RecordTokenCount is a no-op implementation for default metrics
func (m *defaultMetrics) RecordTokenCount(_ context.Context, _ int) {
}

// IncrementTokenCount is a no-op implementation for default metrics
func (m *defaultMetrics) IncrementTokenCount(_ context.Context, _ int) {
}

// RecordMessageCount is a no-op implementation for default metrics
func (m *defaultMetrics) RecordMessageCount(_ context.Context, _ int) {
}
