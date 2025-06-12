package mcp

import (
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

// Metrics provides basic observability for MCP operations
type Metrics struct {
	RegistrationAttempts  int64
	RegistrationSuccesses int64
	RegistrationFailures  int64
	ToolExecutions        int64
	ToolExecutionFailures int64
	ProxyHealthChecks     int64
	ProxyHealthFailures   int64
}

// Global metrics instance
var globalMetrics Metrics

// IncrementRegistrationAttempt increments the registration attempt counter
func IncrementRegistrationAttempt() {
	atomic.AddInt64(&globalMetrics.RegistrationAttempts, 1)
}

// IncrementRegistrationSuccess increments the registration success counter
func IncrementRegistrationSuccess() {
	atomic.AddInt64(&globalMetrics.RegistrationSuccesses, 1)
}

// IncrementRegistrationFailure increments the registration failure counter
func IncrementRegistrationFailure() {
	atomic.AddInt64(&globalMetrics.RegistrationFailures, 1)
}

// IncrementToolExecution increments the tool execution counter
func IncrementToolExecution() {
	atomic.AddInt64(&globalMetrics.ToolExecutions, 1)
}

// IncrementToolExecutionFailure increments the tool execution failure counter
func IncrementToolExecutionFailure() {
	atomic.AddInt64(&globalMetrics.ToolExecutionFailures, 1)
}

// IncrementProxyHealthCheck increments the proxy health check counter
func IncrementProxyHealthCheck() {
	atomic.AddInt64(&globalMetrics.ProxyHealthChecks, 1)
}

// IncrementProxyHealthFailure increments the proxy health check failure counter
func IncrementProxyHealthFailure() {
	atomic.AddInt64(&globalMetrics.ProxyHealthFailures, 1)
}

// GetMetrics returns the current metrics values
func GetMetrics() Metrics {
	return Metrics{
		RegistrationAttempts:  atomic.LoadInt64(&globalMetrics.RegistrationAttempts),
		RegistrationSuccesses: atomic.LoadInt64(&globalMetrics.RegistrationSuccesses),
		RegistrationFailures:  atomic.LoadInt64(&globalMetrics.RegistrationFailures),
		ToolExecutions:        atomic.LoadInt64(&globalMetrics.ToolExecutions),
		ToolExecutionFailures: atomic.LoadInt64(&globalMetrics.ToolExecutionFailures),
		ProxyHealthChecks:     atomic.LoadInt64(&globalMetrics.ProxyHealthChecks),
		ProxyHealthFailures:   atomic.LoadInt64(&globalMetrics.ProxyHealthFailures),
	}
}

// LogMetrics logs the current metrics state for debugging
func LogMetrics(component string) {
	metrics := GetMetrics()
	logger.Info("MCP metrics",
		"component", component,
		"registration_attempts", metrics.RegistrationAttempts,
		"registration_successes", metrics.RegistrationSuccesses,
		"registration_failures", metrics.RegistrationFailures,
		"tool_executions", metrics.ToolExecutions,
		"tool_execution_failures", metrics.ToolExecutionFailures,
		"proxy_health_checks", metrics.ProxyHealthChecks,
		"proxy_health_failures", metrics.ProxyHealthFailures,
	)
}

// TimeOperation logs the duration of an operation
func TimeOperation(operation string, start time.Time) {
	duration := time.Since(start)
	logger.Debug("MCP operation completed",
		"operation", operation,
		"duration_ms", duration.Milliseconds(),
	)
}
