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
	buildInfo             metric.Float64ObservableGauge
	buildInfoRegistration metric.Registration
	uptimeGauge           metric.Float64ObservableGauge
	uptimeRegistration    metric.Registration
	startTime             time.Time
	systemInitOnce        sync.Once
	systemResetMutex      sync.Mutex
)

// initSystemMetrics initializes system health metrics
func initSystemMetrics(meter metric.Meter) {
	if meter == nil {
		return
	}
	systemInitOnce.Do(func() {
		var err error
		buildInfo, err = meter.Float64ObservableGauge(
			"compozy_build_info",
			metric.WithDescription("Build information (value=1)"),
		)
		if err != nil {
			logger.Error("Failed to create build info gauge", "error", err)
			return
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
		// Register callback for build info
		version, commit, goVersion := getBuildInfo()
		buildInfoRegistration, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			o.ObserveFloat64(buildInfo, 1,
				metric.WithAttributes(
					attribute.String("version", version),
					attribute.String("commit_hash", commit),
					attribute.String("go_version", goVersion),
				),
			)
			return nil
		}, buildInfo)
		if err != nil {
			logger.Error("Failed to register build info callback", "error", err)
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

// recordBuildInfo logs the build info (callback is registered in initSystemMetrics)
func recordBuildInfo() {
	version, commit, goVersion := getBuildInfo()
	logger.Info("System metrics initialized",
		"version", version,
		"commit", commit,
		"go_version", goVersion,
	)
}

// InitSystemMetrics initializes system health metrics and records build info
func InitSystemMetrics(_ context.Context, meter metric.Meter) {
	initSystemMetrics(meter)
	recordBuildInfo()
}

// resetSystemMetrics is used for testing purposes only
func resetSystemMetrics() {
	// Unregister callbacks if they exist
	if uptimeRegistration != nil {
		err := uptimeRegistration.Unregister()
		if err != nil {
			logger.Error("Failed to unregister uptime callback during reset", "error", err)
		}
		uptimeRegistration = nil
	}
	if buildInfoRegistration != nil {
		err := buildInfoRegistration.Unregister()
		if err != nil {
			logger.Error("Failed to unregister build info callback during reset", "error", err)
		}
		buildInfoRegistration = nil
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
