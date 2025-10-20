package metrics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	hashLength    = 12
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

// hashLabel creates a deterministic, fixed-length hash of a label value
// to prevent time-series cardinality explosion from high-cardinality values.
func hashLabel(value string) string {
	h := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(h[:])
	if len(encoded) > hashLength {
		return encoded[:hashLength]
	}
	return encoded
}

func createInstruments(ctx context.Context, meter metric.Meter) {
	log := logger.FromContext(ctx)
	// Build into locals to avoid partial global state.
	var (
		eh  metric.Float64Histogram
		ec  metric.Int64Counter
		ac  metric.Int64UpDownCounter
		rh  metric.Float64Histogram
		err error
	)

	eh, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "tool_execute_seconds"),
		metric.WithDescription("MCP tool execution latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30),
	)
	if err != nil {
		log.Error("Failed to create MCP execution histogram", "error", err)
		return
	}

	ec, err = meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "tool_errors_total"),
		metric.WithDescription("MCP tool errors by category"),
	)
	if err != nil {
		log.Error("Failed to create MCP error counter", "error", err)
		return
	}

	ac, err = meter.Int64UpDownCounter(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "server_connections_active"),
		metric.WithDescription("Active MCP server connections"),
	)
	if err != nil {
		log.Error("Failed to create MCP connection counter", "error", err)
		return
	}

	rh, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("mcp", "tool_registry_lookup_seconds"),
		metric.WithDescription("Tool registry lookup latency"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.00001, 0.0001, 0.001, 0.01, 0.1),
	)
	if err != nil {
		log.Error("Failed to create MCP registry histogram", "error", err)
		return
	}
	// Publish only after all succeed.
	executionHistogram = eh
	errorCounter = ec
	activeConnections = ac
	registryHistogram = rh
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
	// Hash high-cardinality labels to prevent time-series explosion.
	attrs := []attribute.KeyValue{
		attribute.String(labelServerID, hashLabel(serverID)),
		attribute.String(labelToolName, hashLabel(toolName)),
		attribute.String(labelOutcome, string(outcome)),
	}
	// Clamp negative durations to zero to protect against clock skew/bugs.
	d := duration.Seconds()
	if d < 0 {
		d = 0
	}
	executionHistogram.Record(ctx, d, metric.WithAttributes(attrs...))
}

// RecordToolError increments the error counter for the provided category.
func RecordToolError(ctx context.Context, serverID, toolName string, kind ErrorKind) {
	if errorCounter == nil {
		return
	}
	if kind == "" {
		kind = ErrorKindExecution
	}
	// Hash high-cardinality labels to prevent time-series explosion.
	errorCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String(labelServerID, hashLabel(serverID)),
		attribute.String(labelToolName, hashLabel(toolName)),
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
	// Atomically swap and compute delta from previous state to prevent race conditions.
	prev, loaded := connectionStates.Swap(serverID, connected)
	var delta int64
	if loaded {
		if p, ok := prev.(bool); ok {
			if p == connected {
				return // no change
			}
			if connected {
				delta = 1
			} else {
				delta = -1
			}
		} else {
			// Unknown previous type; reset to known state without delta.
			return
		}
	} else {
		// First observation: count +1 only when transitioning to connected.
		if !connected {
			return
		}
		delta = 1
	}
	// Hash high-cardinality server_id label.
	activeConnections.Add(ctx, delta, metric.WithAttributes(attribute.String(labelServerID, hashLabel(serverID))))
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
	// Include both string outcome for compatibility and boolean hit for simpler queries.
	registryHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String(labelOutcome, outcome),
		attribute.Bool("hit", hit),
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
