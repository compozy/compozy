package memory

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
)

const (
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"
	statusDegraded  = "degraded"
	componentRedis  = "Redis"
)

// HealthMonitor monitors the health of test environment dependencies
type HealthMonitor struct {
	mu               sync.RWMutex
	env              *TestEnvironment
	checkInterval    time.Duration
	isRunning        atomic.Bool
	healthStatus     map[string]HealthStatus
	alerts           []HealthAlert
	metricsCollector *MetricsCollector
	stopCh           chan struct{}
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Component    string
	Status       string // "healthy", "degraded", "unhealthy"
	LastCheck    time.Time
	LastError    error
	ResponseTime time.Duration
	Details      map[string]any
}

// HealthAlert represents a health alert
type HealthAlert struct {
	Component string
	Severity  string // "warning", "error", "critical"
	Message   string
	Timestamp time.Time
	Details   map[string]any
}

// MetricsCollector collects test environment metrics
type MetricsCollector struct {
	redisOperations     atomic.Int64
	redisErrors         atomic.Int64
	temporalOperations  atomic.Int64
	temporalErrors      atomic.Int64
	memoryOperations    atomic.Int64
	memoryErrors        atomic.Int64
	averageResponseTime atomic.Int64 // in microseconds
	startTime           time.Time
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(env *TestEnvironment) *HealthMonitor {
	return &HealthMonitor{
		env:              env,
		checkInterval:    5 * time.Second,
		healthStatus:     make(map[string]HealthStatus),
		alerts:           make([]HealthAlert, 0),
		metricsCollector: NewMetricsCollector(),
		stopCh:           make(chan struct{}),
	}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime: time.Now(),
	}
}

// Start starts the health monitoring
func (h *HealthMonitor) Start(ctx context.Context) {
	if h.isRunning.Load() {
		return
	}
	h.isRunning.Store(true)
	go h.monitorLoop(ctx)
}

// Stop stops the health monitoring
func (h *HealthMonitor) Stop() {
	if !h.isRunning.Load() {
		return
	}
	h.isRunning.Store(false)
	close(h.stopCh)
}

// monitorLoop runs the monitoring loop
func (h *HealthMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()
	// Initial check
	h.performHealthChecks(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks performs all health checks
func (h *HealthMonitor) performHealthChecks(ctx context.Context) {
	h.checkRedisHealth(ctx)
	h.checkTemporalHealth(ctx)
	h.checkMemoryManagerHealth(ctx)
	h.checkSystemResources(ctx)
}

// checkRedisHealth checks Redis health
func (h *HealthMonitor) checkRedisHealth(ctx context.Context) {
	start := time.Now()
	status := HealthStatus{
		Component: "Redis",
		Status:    statusHealthy,
		LastCheck: start,
		Details:   make(map[string]any),
	}
	// Ping Redis
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := h.env.GetRedis().Ping(checkCtx).Err()
	status.ResponseTime = time.Since(start)
	if err != nil {
		status.Status = statusUnhealthy
		status.LastError = err
		h.recordAlert("Redis", "error", fmt.Sprintf("Redis ping failed: %v", err), nil)
		h.metricsCollector.redisErrors.Add(1)
	} else {
		// Check Redis info
		info, err := h.env.GetRedis().Info(checkCtx).Result()
		if err == nil {
			status.Details["info"] = parseRedisInfo(info)
		}
		// Check memory usage
		memInfo, err := h.env.GetRedis().Info(checkCtx, "memory").Result()
		if err == nil {
			status.Details["memory"] = parseRedisInfo(memInfo)
		}
	}
	h.updateHealthStatus(&status)
}

// checkTemporalHealth checks Temporal health
func (h *HealthMonitor) checkTemporalHealth(_ context.Context) {
	start := time.Now()
	status := HealthStatus{
		Component: "Temporal",
		Status:    statusHealthy,
		LastCheck: start,
		Details:   make(map[string]any),
	}
	// For now, just check if Temporal is available
	if h.env.temporalClient == nil {
		status.Status = statusUnhealthy
		status.LastError = fmt.Errorf("temporal client not initialized")
		h.recordAlert("Temporal", "warning", "Temporal client not available", nil)
	}
	status.ResponseTime = time.Since(start)
	h.updateHealthStatus(&status)
}

// checkMemoryManagerHealth checks memory manager health
func (h *HealthMonitor) checkMemoryManagerHealth(ctx context.Context) {
	start := time.Now()
	status := HealthStatus{
		Component: "MemoryManager",
		Status:    statusHealthy,
		LastCheck: start,
		Details:   make(map[string]any),
	}
	// Test creating a dummy instance
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	testRef := core.MemoryReference{
		ID:  "customer-support",
		Key: "health-check-{{.timestamp}}",
	}
	workflowCtx := map[string]any{
		"project.id": "test-project",
		"timestamp":  fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	instance, err := h.env.GetMemoryManager().GetInstance(checkCtx, testRef, workflowCtx)
	status.ResponseTime = time.Since(start)
	if err != nil {
		status.Status = statusUnhealthy
		status.LastError = err
		h.recordAlert("MemoryManager", "error", fmt.Sprintf("Failed to create test instance: %v", err), nil)
		h.metricsCollector.memoryErrors.Add(1)
	} else {
		// Clean up test instance
		if instance != nil {
			if clearErr := instance.Clear(ctx); clearErr != nil {
				// Log error but don't fail - test instance cleanup is best effort
				status.Details["cleanup_error"] = clearErr.Error()
			}
		}
		status.Details["canCreateInstances"] = true
	}
	h.updateHealthStatus(&status)
}

// checkSystemResources checks system resources
func (h *HealthMonitor) checkSystemResources(_ context.Context) {
	start := time.Now()
	status := HealthStatus{
		Component: "System",
		Status:    statusHealthy,
		LastCheck: start,
		Details:   make(map[string]any),
	}
	// Get goroutine count
	status.Details["goroutines"] = runtime.NumGoroutine()
	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	status.Details["memory"] = map[string]any{
		"alloc_mb":       memStats.Alloc / 1024 / 1024,
		"total_alloc_mb": memStats.TotalAlloc / 1024 / 1024,
		"sys_mb":         memStats.Sys / 1024 / 1024,
		"num_gc":         memStats.NumGC,
	}
	// Check if memory usage is high
	if memStats.Alloc > 500*1024*1024 { // 500MB threshold
		status.Status = statusDegraded
		h.recordAlert("System", "warning",
			fmt.Sprintf("High memory usage: %d MB", memStats.Alloc/1024/1024),
			map[string]any{"alloc_bytes": memStats.Alloc})
	}
	status.ResponseTime = time.Since(start)
	h.updateHealthStatus(&status)
}

// updateHealthStatus updates the health status
func (h *HealthMonitor) updateHealthStatus(status *HealthStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthStatus[status.Component] = *status
}

// recordAlert records a health alert
func (h *HealthMonitor) recordAlert(component, severity, message string, details map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	alert := HealthAlert{
		Component: component,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Details:   details,
	}
	h.alerts = append(h.alerts, alert)
	// Keep only last 100 alerts
	if len(h.alerts) > 100 {
		h.alerts = h.alerts[len(h.alerts)-100:]
	}
}

// GetHealthStatus returns the current health status
func (h *HealthMonitor) GetHealthStatus() map[string]HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]HealthStatus)
	for k, v := range h.healthStatus {
		result[k] = v
	}
	return result
}

// GetAlerts returns recent alerts
func (h *HealthMonitor) GetAlerts(since time.Time) []HealthAlert {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []HealthAlert
	for _, alert := range h.alerts {
		if alert.Timestamp.After(since) {
			result = append(result, alert)
		}
	}
	return result
}

// GetMetrics returns collected metrics
func (h *HealthMonitor) GetMetrics() map[string]any {
	collector := h.metricsCollector
	uptime := time.Since(collector.startTime)
	avgResponseTime := time.Duration(collector.averageResponseTime.Load()) * time.Microsecond
	return map[string]any{
		"uptime_seconds":        uptime.Seconds(),
		"redis_operations":      collector.redisOperations.Load(),
		"redis_errors":          collector.redisErrors.Load(),
		"temporal_operations":   collector.temporalOperations.Load(),
		"temporal_errors":       collector.temporalErrors.Load(),
		"memory_operations":     collector.memoryOperations.Load(),
		"memory_errors":         collector.memoryErrors.Load(),
		"average_response_time": avgResponseTime.String(),
	}
}

// RecordOperation records an operation metric
func (h *HealthMonitor) RecordOperation(component string, success bool, duration time.Duration) {
	collector := h.metricsCollector
	switch component {
	case componentRedis:
		collector.redisOperations.Add(1)
		if !success {
			collector.redisErrors.Add(1)
		}
	case "Temporal":
		collector.temporalOperations.Add(1)
		if !success {
			collector.temporalErrors.Add(1)
		}
	case "Memory":
		collector.memoryOperations.Add(1)
		if !success {
			collector.memoryErrors.Add(1)
		}
	}
	// Update average response time (simplified moving average)
	currentAvg := collector.averageResponseTime.Load()
	newAvg := (currentAvg*9 + duration.Microseconds()) / 10
	collector.averageResponseTime.Store(newAvg)
}

// PrintHealthReport prints a health report
func (h *HealthMonitor) PrintHealthReport(t *testing.T) {
	t.Helper()
	status := h.GetHealthStatus()
	metrics := h.GetMetrics()
	recentAlerts := h.GetAlerts(time.Now().Add(-5 * time.Minute))
	t.Log("=== Test Environment Health Report ===")
	t.Log("Component Status:")
	for component, health := range status {
		t.Logf("  %s: %s (response time: %v)", component, health.Status, health.ResponseTime)
		if health.LastError != nil {
			t.Logf("    Last error: %v", health.LastError)
		}
	}
	t.Log("\nMetrics:")
	t.Logf("  Uptime: %.2f seconds", metrics["uptime_seconds"])
	t.Logf("  Redis: %d operations, %d errors", metrics["redis_operations"], metrics["redis_errors"])
	t.Logf("  Memory: %d operations, %d errors", metrics["memory_operations"], metrics["memory_errors"])
	t.Logf("  Average response time: %v", metrics["average_response_time"])
	if len(recentAlerts) > 0 {
		t.Log("\nRecent Alerts:")
		for _, alert := range recentAlerts {
			t.Logf("  [%s] %s: %s", alert.Severity, alert.Component, alert.Message)
		}
	}
	t.Log("=====================================")
}

// parseRedisInfo parses Redis INFO output
func parseRedisInfo(info string) map[string]any {
	result := make(map[string]any)
	// Simplified parsing - just extract a few key metrics
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Parse specific metrics
			switch key {
			case "used_memory_human", "used_memory_peak_human", "connected_clients", "uptime_in_seconds":
				result[key] = value
			}
		}
	}
	return result
}

// HealthCheckHelper provides test helper functions for health monitoring
type HealthCheckHelper struct {
	monitor *HealthMonitor
}

// NewHealthCheckHelper creates a new health check helper
func NewHealthCheckHelper(env *TestEnvironment) *HealthCheckHelper {
	return &HealthCheckHelper{
		monitor: NewHealthMonitor(env),
	}
}

// RequireHealthy requires all components to be healthy
func (h *HealthCheckHelper) RequireHealthy(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	h.monitor.performHealthChecks(ctx)
	status := h.monitor.GetHealthStatus()
	for component, health := range status {
		if health.Status != statusHealthy {
			t.Fatalf("Component %s is not healthy: %s (error: %v)",
				component, health.Status, health.LastError)
		}
	}
}

// WaitForHealthy waits for all components to become healthy
func (h *HealthCheckHelper) WaitForHealthy(t *testing.T, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.monitor.PrintHealthReport(t)
			t.Fatalf("Timeout waiting for components to become healthy")
		case <-ticker.C:
			h.monitor.performHealthChecks(ctx)
			status := h.monitor.GetHealthStatus()
			allHealthy := true
			for _, health := range status {
				if health.Status != statusHealthy {
					allHealthy = false
					break
				}
			}
			if allHealthy {
				return
			}
		}
	}
}

// MonitorTest monitors a test execution
func (h *HealthCheckHelper) MonitorTest(t *testing.T, testFunc func()) {
	t.Helper()
	ctx := context.Background()
	h.monitor.Start(ctx)
	defer h.monitor.Stop()
	defer h.monitor.PrintHealthReport(t)
	// Run the test
	testFunc()
}
