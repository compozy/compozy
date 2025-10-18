package metrics

import (
	"context"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	labelServerID = "server_id"
	labelToolName = "tool_name"
	labelOutcome  = "outcome"
	labelError    = "error_kind"
	labelHit      = "hit"
	labelMiss     = "miss"
)

var (
	once sync.Once

	executionHistogram metric.Float64Histogram
	errorCounter       metric.Int64Counter
	activeConnections  metric.Int64UpDownCounter
	registryHistogram  metric.Float64Histogram

	connectionStates sync.Map
	resetMu          sync.Mutex
)

// Init configures and registers MCP instrumentation with the provided meter.
func Init(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	once.Do(func() {
		createInstruments(ctx, meter)
	})
}

func createInstruments(ctx context.Context, meter metric.Meter) {
	log := logger.FromContext(ctx)
	var err error

	executionHistogram, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "tool_execute_seconds"),
		metric.WithDescription("MCP tool execution latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30),
	)
	if err != nil {
		log.Error("Failed to create MCP execution histogram", "error", err)
		return
	}

	errorCounter, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "tool_errors_total"),
		metric.WithDescription("MCP tool errors by category"),
	)
	if err != nil {
		log.Error("Failed to create MCP error counter", "error", err)
		return
	}

	activeConnections, err = meter.Int64UpDownCounter(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "server_connections_active"),
		metric.WithDescription("Active MCP server connections"),
	)
	if err != nil {
		log.Error("Failed to create MCP connection counter", "error", err)
		return
	}

	registryHistogram, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "tool_registry_lookup_seconds"),
		metric.WithDescription("Tool registry lookup latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.00001, 0.0001, 0.001, 0.01, 0.1),
	)
	if err != nil {
		log.Error("Failed to create MCP registry histogram", "error", err)
		return
	}
	log.Info("Initialized MCP metrics instruments")
}

// ExecutionOutcome describes the result of a tool execution attempt.
type ExecutionOutcome string

const (
	// OutcomeSuccess indicates the tool completed without error.
	OutcomeSuccess ExecutionOutcome = "success"
	// OutcomeError indicates the tool returned an error.
	OutcomeError ExecutionOutcome = "error"
	// OutcomeTimeout indicates the tool timed out.
	OutcomeTimeout ExecutionOutcome = "timeout"
)

// ErrorKind categorizes the type of failure observed during execution.
type ErrorKind string

const (
	// ErrorKindConnection covers transport and connectivity failures.
	ErrorKindConnection ErrorKind = "connection"
	// ErrorKindValidation covers client-side validation errors.
	ErrorKindValidation ErrorKind = "validation"
	// ErrorKindExecution covers remote execution failures.
	ErrorKindExecution ErrorKind = "execution"
	// ErrorKindTimeout covers timeouts at any layer.
	ErrorKindTimeout ErrorKind = "timeout"
)

// RecordToolExecution emits latency metrics for tool invocations.
func RecordToolExecution(
	ctx context.Context,
	serverID, toolName string,
	duration time.Duration,
	outcome ExecutionOutcome,
) {
	if executionHistogram == nil {
		return
	}
	if outcome == "" {
		outcome = OutcomeSuccess
	}
	attrs := []attribute.KeyValue{
		attribute.String(labelServerID, serverID),
		attribute.String(labelToolName, toolName),
		attribute.String(labelOutcome, string(outcome)),
	}
	executionHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordToolError increments the error counter for the provided category.
func RecordToolError(ctx context.Context, serverID, toolName string, kind ErrorKind) {
	if errorCounter == nil {
		return
	}
	if kind == "" {
		kind = ErrorKindExecution
	}
	errorCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String(labelServerID, serverID),
		attribute.String(labelToolName, toolName),
		attribute.String(labelError, string(kind)),
	))
}

// MarkServerConnected records an active connection for the given server.
func MarkServerConnected(ctx context.Context, serverID string) {
	updateConnectionState(ctx, serverID, true)
}

// MarkServerDisconnected records the loss of an active connection.
func MarkServerDisconnected(ctx context.Context, serverID string) {
	updateConnectionState(ctx, serverID, false)
}

func updateConnectionState(ctx context.Context, serverID string, connected bool) {
	if activeConnections == nil || serverID == "" {
		return
	}
	prev, loaded := connectionStates.Load(serverID)
	prevBool, ok := prev.(bool)
	if loaded && ok && prevBool == connected {
		return
	}
	var delta int64
	if connected {
		delta = 1
		connectionStates.Store(serverID, true)
	} else {
		delta = -1
		connectionStates.Store(serverID, false)
	}
	activeConnections.Add(ctx, delta, metric.WithAttributes(attribute.String(labelServerID, serverID)))
}

// RecordRegistryLookup captures latency and hit/miss data for registry lookups.
func RecordRegistryLookup(ctx context.Context, duration time.Duration, hit bool) {
	if registryHistogram == nil {
		return
	}
	outcome := labelMiss
	if hit {
		outcome = labelHit
	}
	registryHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String(labelOutcome, outcome),
	))
}

// ResetForTesting clears metric state to allow reinitialization in tests.
func ResetForTesting() {
	resetMu.Lock()
	defer resetMu.Unlock()

	executionHistogram = nil
	errorCounter = nil
	activeConnections = nil
	registryHistogram = nil

	connectionStates = sync.Map{}
	once = sync.Once{}
}
