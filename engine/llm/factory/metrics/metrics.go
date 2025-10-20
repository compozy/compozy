package factorymetrics

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	labelFactoryType = "factory_type"
	labelName        = "name"
	unitSeconds      = "s"

	// TypeProvider tracks provider factory creation latency.
	TypeProvider = "provider"
	// TypeTool tracks tool factory creation latency.
	TypeTool = "tool"
)

var (
	// defaultCreateBuckets defines latency histogram buckets in seconds, spanning microseconds to seconds.
	defaultCreateBuckets = []float64{0.00001, 0.0001, 0.001, 0.01, 0.1, 1, 5}

	initOnce        sync.Once
	createHistogram metric.Float64Histogram
)

// Init registers factory metrics instruments with the provided meter.
func Init(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	initOnce.Do(func() {
		log := logger.FromContext(ctx)
		histogram, err := meter.Float64Histogram(
			metrics.MetricNameWithSubsystem("factory", "create_seconds"),
			metric.WithDescription("Factory instantiation time"),
			metric.WithUnit(unitSeconds),
			metric.WithExplicitBucketBoundaries(defaultCreateBuckets...),
		)
		if err != nil {
			log.Error("Failed to create factory histogram", "error", err)
			return
		}
		createHistogram = histogram
		log.Debug("Initialized factory metrics instruments")
	})
}

// RecordCreate records the duration spent creating a factory instance.
func RecordCreate(ctx context.Context, factoryType, name string, duration time.Duration) {
	if createHistogram == nil || duration < 0 {
		return
	}
	finalType := strings.TrimSpace(factoryType)
	if finalType == "" {
		finalType = TypeProvider
	}
	finalName := strings.TrimSpace(name)
	if finalName == "" {
		finalName = "unknown"
	}
	createHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String(labelFactoryType, finalType),
		attribute.String(labelName, finalName),
	))
}

// ResetForTesting clears metric state to allow reinitialization in tests.
func ResetForTesting() {
	createHistogram = nil
	initOnce = sync.Once{}
}
