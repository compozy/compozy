package monitoring

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Build variables to be set via ldflags during compilation
// Example: go build -ldflags "-X 'github.com/compozy/compozy/engine/infra/monitoring.Version=v1.0.0'"
var (
	Version    = "unknown"
	CommitHash = "unknown"
)

var (
	buildInfo          metric.Float64Gauge
	uptimeGauge        metric.Float64ObservableGauge
	uptimeRegistration metric.Registration
	startTime          time.Time
	systemInitOnce     sync.Once
	systemResetMutex   sync.Mutex
)

// initSystemMetrics initializes system health metrics
func initSystemMetrics(meter metric.Meter) {
	systemInitOnce.Do(func() {
		var err error
		buildInfo, err = meter.Float64Gauge(
			"compozy_build_info",
			metric.WithDescription("Build information (value=1)"),
		)
		if err != nil {
			logger.Error("Failed to create build info gauge", "error", err)
		}
		// Create observable gauge for uptime
		uptimeGauge, err = meter.Float64ObservableGauge(
			"compozy_uptime_seconds",
			metric.WithDescription("Service uptime in seconds"),
		)
		if err != nil {
			logger.Error("Failed to create uptime gauge", "error", err)
			return
		}
		// Record start time for uptime calculation
		startTime = time.Now()
		// Register callback to observe uptime
		uptimeRegistration, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			uptime := time.Since(startTime).Seconds()
			o.ObserveFloat64(uptimeGauge, uptime)
			return nil
		}, uptimeGauge)
		if err != nil {
			logger.Error("Failed to register uptime callback", "error", err)
		}
	})
}

// getBuildInfo returns build information with fallback strategies
func getBuildInfo() (version, commit, goVersion string) {
	// Primary: Use injected build variables
	version = Version
	commit = CommitHash
	// Fallback: Try to get from runtime
	if info, ok := debug.ReadBuildInfo(); ok {
		if version == "unknown" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
		if commit == "unknown" {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					commit = setting.Value
					break
				}
			}
		}
	}
	// Go version from runtime
	goVersion = runtime.Version()
	return version, commit, goVersion
}

// recordBuildInfo records build information as a gauge metric with labels
func recordBuildInfo(ctx context.Context) {
	if buildInfo == nil {
		return
	}
	version, commit, goVersion := getBuildInfo()
	buildInfo.Record(ctx, 1,
		metric.WithAttributes(
			attribute.String("version", version),
			attribute.String("commit_hash", commit),
			attribute.String("go_version", goVersion),
		),
	)
	logger.Info("System metrics initialized",
		"version", version,
		"commit", commit,
		"go_version", goVersion,
	)
}

// InitSystemMetrics initializes system health metrics and records build info
func InitSystemMetrics(ctx context.Context, meter metric.Meter) {
	initSystemMetrics(meter)
	recordBuildInfo(ctx)
}

// resetSystemMetrics is used for testing purposes only
func resetSystemMetrics() {
	// Unregister callback if it exists
	if uptimeRegistration != nil {
		err := uptimeRegistration.Unregister()
		if err != nil {
			logger.Error("Failed to unregister uptime callback during reset", "error", err)
		}
		uptimeRegistration = nil
	}
	buildInfo = nil
	uptimeGauge = nil
	startTime = time.Time{}
	systemInitOnce = sync.Once{}
}

// ResetSystemMetricsForTesting resets the system metrics initialization state for testing
// This should only be used in tests to ensure clean state between test runs
func ResetSystemMetricsForTesting() {
	systemResetMutex.Lock()
	defer systemResetMutex.Unlock()
	resetSystemMetrics()
}
