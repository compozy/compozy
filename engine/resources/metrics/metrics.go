package metrics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	resourcesSubsystem     = "resources"
	importSubsystem        = "resources_import"
	exportSubsystem        = "resources_export"
	serializationSubsystem = "serialization"
	operationLabel         = "operation"
	resourceTypeLabel      = "resource_type"
	outcomeLabel           = "outcome"
	formatLabel            = "format"
	errorTypeLabel         = "error_type"
)

const (
	outcomeSuccessValue        = "success"
	outcomeErrorValue          = "error"
	formatJSONValue            = "json"
	formatYAMLValue            = "yaml"
	importErrorParseValue      = "parse_error"
	importErrorValidationValue = "validation_error"
	importErrorDuplicateValue  = "duplicate"
	serializationOpMarshal     = "marshal"
	serializationOpUnmarshal   = "unmarshal"
	unknownResourceType        = "unknown"
)

var (
	metricsOnce sync.Once
	initErr     error

	resourceOperationsCounter metric.Int64Counter
	resourceOperationLatency  metric.Float64Histogram
	resourceStoreGauge        metric.Int64ObservableGauge
	resourceETagConflicts     metric.Int64Counter

	resourceImportLatency metric.Float64Histogram
	resourceImportItems   metric.Int64Counter
	resourceImportErrors  metric.Int64Counter
	resourceExportLatency metric.Float64Histogram
	resourceExportItems   metric.Int64Counter

	serializationLatency metric.Float64Histogram
	serializationBytes   metric.Float64Histogram
	serializationErrors  metric.Int64Counter

	gaugeRegistration metric.Registration

	storeSizes sync.Map // map[string]*atomic.Int64
)

var (
	resourceOperationBuckets     = []float64{0.0001, 0.001, 0.01, 0.1, 1}
	importExportBuckets          = []float64{0.1, 0.5, 1, 5, 10, 30}
	serializationDurationBuckets = []float64{0.00001, 0.0001, 0.001, 0.01, 0.1}
	serializationSizeBuckets     = []float64{100, 1000, 10000, 100000, 1000000}
)

// EnsureInitialized prepares the OpenTelemetry instruments for resource metrics.
func EnsureInitialized() {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.resources")
		initErr = initInstruments(meter)
	})
}

func initInstruments(meter metric.Meter) error {
	if err := initResourceOperationInstruments(meter); err != nil {
		return err
	}
	if err := initImportExportInstruments(meter); err != nil {
		return err
	}
	if err := initSerializationInstruments(meter); err != nil {
		return err
	}
	if gaugeRegistration != nil {
		if err := gaugeRegistration.Unregister(); err != nil {
			return fmt.Errorf("resources metrics: unregister store gauge: %w", err)
		}
	}
	registration, err := meter.RegisterCallback(observeStoreSize, resourceStoreGauge)
	if err != nil {
		return err
	}
	gaugeRegistration = registration
	return nil
}

func initResourceOperationInstruments(meter metric.Meter) error {
	var err error
	resourceOperationsCounter, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(resourcesSubsystem, "operations_total"),
		metric.WithDescription("Resource store operations"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	resourceOperationLatency, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(resourcesSubsystem, "operation_duration_seconds"),
		metric.WithDescription("Resource operation latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(resourceOperationBuckets...),
	)
	if err != nil {
		return err
	}
	resourceStoreGauge, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem(resourcesSubsystem, "store_size"),
		metric.WithDescription("Number of resources stored by type"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	resourceETagConflicts, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(resourcesSubsystem, "etag_mismatches_total"),
		metric.WithDescription("Optimistic locking conflicts (ETag mismatches)"),
		metric.WithUnit("1"),
	)
	return err
}

func initImportExportInstruments(meter metric.Meter) error {
	var err error
	resourceImportLatency, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(importSubsystem, "duration_seconds"),
		metric.WithDescription("Resource import operation duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(importExportBuckets...),
	)
	if err != nil {
		return err
	}
	resourceImportItems, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(importSubsystem, "items_total"),
		metric.WithDescription("Number of resources imported"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	resourceImportErrors, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(importSubsystem, "errors_total"),
		metric.WithDescription("Resource import errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return err
	}
	resourceExportLatency, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(exportSubsystem, "duration_seconds"),
		metric.WithDescription("Resource export operation duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(importExportBuckets...),
	)
	if err != nil {
		return err
	}
	resourceExportItems, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(exportSubsystem, "items_total"),
		metric.WithDescription("Number of resources exported"),
		metric.WithUnit("1"),
	)
	return err
}

func initSerializationInstruments(meter metric.Meter) error {
	var err error
	serializationLatency, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(serializationSubsystem, "duration_seconds"),
		metric.WithDescription("Serialization operation duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(serializationDurationBuckets...),
	)
	if err != nil {
		return err
	}
	serializationBytes, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(serializationSubsystem, "bytes"),
		metric.WithDescription("Serialized data size"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(serializationSizeBuckets...),
	)
	if err != nil {
		return err
	}
	serializationErrors, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem(serializationSubsystem, "errors_total"),
		metric.WithDescription("Serialization errors"),
		metric.WithUnit("1"),
	)
	return err
}

func observeStoreSize(_ context.Context, observer metric.Observer) error {
	storeSizes.Range(func(key, value any) bool {
		typ, ok := key.(string)
		if !ok || typ == "" {
			return true
		}
		counter, ok := value.(*atomic.Int64)
		if !ok || counter == nil {
			return true
		}
		current := counter.Load()
		if current < 0 {
			current = 0
		}
		observer.ObserveInt64(
			resourceStoreGauge,
			current,
			metric.WithAttributes(attribute.String(resourceTypeLabel, typ)),
		)
		return true
	})
	return nil
}

func metricsContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// RecordOperation captures a resource store operation attempt.
func RecordOperation(
	ctx context.Context,
	operation string,
	resourceType string,
	outcome string,
	duration time.Duration,
) {
	EnsureInitialized()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(operationLabel, strings.TrimSpace(operation)),
		attribute.String(resourceTypeLabel, strings.TrimSpace(resourceType)),
		attribute.String(outcomeLabel, normalizeOutcome(outcome)),
	}
	mctx := metricsContext(ctx)
	if resourceOperationsCounter != nil {
		resourceOperationsCounter.Add(mctx, 1, metric.WithAttributes(attrs...))
	}
	if duration > 0 && resourceOperationLatency != nil {
		resourceOperationLatency.Record(mctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}

func normalizeOutcome(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case outcomeSuccessValue:
		return outcomeSuccessValue
	case outcomeErrorValue:
		return outcomeErrorValue
	default:
		return outcomeErrorValue
	}
}

// RecordETagMismatch increments the conflict counter for the given resource type.
func RecordETagMismatch(ctx context.Context, resourceType string) {
	EnsureInitialized()
	if initErr != nil || resourceETagConflicts == nil {
		return
	}
	resourceETagConflicts.Add(
		metricsContext(ctx),
		1,
		metric.WithAttributes(attribute.String(resourceTypeLabel, strings.TrimSpace(resourceType))),
	)
}

// AdjustStoreSize applies a delta to the tracked resource count for the specified type.
func AdjustStoreSize(resourceType string, delta int64) {
	if delta == 0 {
		return
	}
	counter := getStoreSizeCounter(resourceType)
	for {
		current := counter.Load()
		next := current + delta
		if next < 0 {
			next = 0
		}
		if counter.CompareAndSwap(current, next) {
			return
		}
	}
}

// SetStoreSize sets the tracked value for the specified resource type.
func SetStoreSize(resourceType string, value int64) {
	if value < 0 {
		value = 0
	}
	counter := getStoreSizeCounter(resourceType)
	counter.Store(value)
}

func getStoreSizeCounter(resourceType string) *atomic.Int64 {
	key := strings.TrimSpace(resourceType)
	if key == "" {
		key = unknownResourceType
	}
	if existing, ok := storeSizes.Load(key); ok {
		if counter, ok2 := existing.(*atomic.Int64); ok2 && counter != nil {
			return counter
		}
	}
	counter := &atomic.Int64{}
	actual, _ := storeSizes.LoadOrStore(key, counter)
	if typed, ok := actual.(*atomic.Int64); ok && typed != nil {
		return typed
	}
	return counter
}

// RecordImportDuration records the total duration of an import batch.
func RecordImportDuration(ctx context.Context, format string, duration time.Duration) {
	EnsureInitialized()
	if initErr != nil || resourceImportLatency == nil || duration <= 0 {
		return
	}
	resourceImportLatency.Record(
		metricsContext(ctx),
		duration.Seconds(),
		metric.WithAttributes(attribute.String(formatLabel, normalizeFormat(format))),
	)
}

// RecordImportItems increments the imported items counter by resource type.
func RecordImportItems(ctx context.Context, resourceType string, count int) {
	if count <= 0 {
		return
	}
	EnsureInitialized()
	if initErr != nil || resourceImportItems == nil {
		return
	}
	resourceImportItems.Add(
		metricsContext(ctx),
		int64(count),
		metric.WithAttributes(attribute.String(resourceTypeLabel, strings.TrimSpace(resourceType))),
	)
}

// RecordImportError tracks an import error grouped by category.
func RecordImportError(ctx context.Context, errorType string) {
	EnsureInitialized()
	if initErr != nil || resourceImportErrors == nil {
		return
	}
	resourceImportErrors.Add(
		metricsContext(ctx),
		1,
		metric.WithAttributes(attribute.String(errorTypeLabel, normalizeImportError(errorType))),
	)
}

// RecordExportDuration records export batch duration.
func RecordExportDuration(ctx context.Context, format string, duration time.Duration) {
	EnsureInitialized()
	if initErr != nil || resourceExportLatency == nil || duration <= 0 {
		return
	}
	resourceExportLatency.Record(
		metricsContext(ctx),
		duration.Seconds(),
		metric.WithAttributes(attribute.String(formatLabel, normalizeFormat(format))),
	)
}

// RecordExportItems increments the exported items counter by resource type.
func RecordExportItems(ctx context.Context, resourceType string, count int) {
	if count <= 0 {
		return
	}
	EnsureInitialized()
	if initErr != nil || resourceExportItems == nil {
		return
	}
	resourceExportItems.Add(
		metricsContext(ctx),
		int64(count),
		metric.WithAttributes(attribute.String(resourceTypeLabel, strings.TrimSpace(resourceType))),
	)
}

// RecordSerialization captures serialization/deserialization metrics.
func RecordSerialization(
	ctx context.Context,
	format string,
	operation string,
	duration time.Duration,
	bytes int,
	err error,
) {
	EnsureInitialized()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(formatLabel, normalizeFormat(format)),
		attribute.String(operationLabel, normalizeSerializationOperation(operation)),
	}
	mctx := metricsContext(ctx)
	if duration > 0 && serializationLatency != nil {
		serializationLatency.Record(mctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
	if bytes > 0 && serializationBytes != nil {
		serializationBytes.Record(mctx, float64(bytes), metric.WithAttributes(attrs...))
	}
	if err != nil && serializationErrors != nil {
		serializationErrors.Add(mctx, 1, metric.WithAttributes(attrs...))
	}
}

func normalizeFormat(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case formatJSONValue:
		return formatJSONValue
	case formatYAMLValue:
		return formatYAMLValue
	default:
		return formatYAMLValue
	}
}

func normalizeImportError(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case importErrorParseValue:
		return importErrorParseValue
	case importErrorValidationValue:
		return importErrorValidationValue
	case importErrorDuplicateValue:
		return importErrorDuplicateValue
	default:
		return importErrorValidationValue
	}
}

func normalizeSerializationOperation(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case serializationOpMarshal:
		return serializationOpMarshal
	case serializationOpUnmarshal:
		return serializationOpUnmarshal
	default:
		return serializationOpMarshal
	}
}
