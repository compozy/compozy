package monitoring

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/version"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	// defaultVersion is the default version when not set via ldflags
	defaultVersion = "unknown"
	// defaultCommit is the default commit hash when not set via ldflags
	defaultCommit = "unknown"
)

var (
	buildInfo             metric.Float64ObservableGauge
	buildInfoRegistration metric.Registration
	uptimeGauge           metric.Float64ObservableGauge
	uptimeRegistration    metric.Registration
	startTime             time.Time
	systemInitOnce        sync.Once
	systemResetMutex      sync.Mutex
	// Build info cache with thread safety
	buildInfoCache      buildInfoData
	buildInfoOnce       sync.Once
	buildInfoCacheMutex sync.RWMutex
)

type buildInfoData struct {
	version   string
	commit    string
	goVersion string
}

// initSystemMetrics initializes system health metrics
func initSystemMetrics(ctx context.Context, meter metric.Meter) {
	log := logger.FromContext(ctx)
	if meter == nil {
		return
	}
	systemInitOnce.Do(func() {
		var err error
		// Record start time immediately for accurate uptime
		startTime = time.Now()
		// Create gauges first (fast operations)
		buildInfo, err = meter.Float64ObservableGauge(
			metrics.MetricName("build_info"),
			metric.WithDescription("Build information (value=1)"),
		)
		if err != nil {
			log.Error("Failed to create build info gauge", "error", err)
			return
		}
		// Create observable gauge for uptime
		uptimeGauge, err = meter.Float64ObservableGauge(
			metrics.MetricName("uptime_seconds"),
			metric.WithDescription("Service uptime in seconds"),
		)
		if err != nil {
			log.Error("Failed to create uptime gauge", "error", err)
			return
		}
		// Register callbacks (also fast)
		uptimeRegistration, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			uptime := time.Since(startTime).Seconds()
			o.ObserveFloat64(uptimeGauge, uptime)
			return nil
		}, uptimeGauge)
		if err != nil {
			log.Error("Failed to register uptime callback", "error", err)
		}
		// Register callback for build info (values loaded dynamically)
		buildInfoRegistration, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			// Get build info dynamically to pick up async loaded values
			version, commit, goVersion := getBuildInfo()
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
			log.Error("Failed to register build info callback", "error", err)
		}
		// Build info will be logged asynchronously when loaded
		// No need for a separate goroutine here
	})
}

// getBuildInfo returns build information with lazy loading and caching
func getBuildInfo() (version, commit, goVersion string) {
	buildInfoOnce.Do(func() {
		buildInfoCacheMutex.Lock()
		buildInfoCache = loadBuildInfo()
		buildInfoCacheMutex.Unlock()
	})
	buildInfoCacheMutex.RLock()
	defer buildInfoCacheMutex.RUnlock()
	return buildInfoCache.version, buildInfoCache.commit, buildInfoCache.goVersion
}

// loadBuildInfo loads build information from various sources
func loadBuildInfo() buildInfoData {
	versionStr := version.GetVersion()
	commit := version.GetCommitHash()
	// If injected build variables are set to non-default values, use them
	useLdflags := versionStr != defaultVersion && commit != defaultCommit
	// For tests, avoid slow I/O operations
	if !useLdflags && !testing.Testing() {
		// Try to get build info from runtime
		if info, ok := debug.ReadBuildInfo(); ok {
			if versionStr == defaultVersion && info.Main.Version != "" && info.Main.Version != "(devel)" {
				versionStr = info.Main.Version
			}
			if commit == defaultCommit {
				for _, setting := range info.Settings {
					if setting.Key == "vcs.revision" {
						commit = setting.Value
						break
					}
				}
			}
		}
	}
	return buildInfoData{
		version:   versionStr,
		commit:    commit,
		goVersion: runtime.Version(),
	}
}

// InitSystemMetrics initializes system health metrics and records build info
func InitSystemMetrics(ctx context.Context, meter metric.Meter) {
	initSystemMetrics(ctx, meter)
}

// resetSystemMetrics is used for testing purposes only
func resetSystemMetrics(ctx context.Context) {
	log := logger.FromContext(ctx)
	// Unregister callbacks if they exist
	if uptimeRegistration != nil {
		err := uptimeRegistration.Unregister()
		if err != nil {
			log.Debug("Failed to unregister uptime callback during reset", "error", err)
		}
		uptimeRegistration = nil
	}
	if buildInfoRegistration != nil {
		err := buildInfoRegistration.Unregister()
		if err != nil {
			log.Debug("Failed to unregister build info callback during reset", "error", err)
		}
		buildInfoRegistration = nil
	}
	buildInfo = nil
	uptimeGauge = nil
	startTime = time.Time{}
	systemInitOnce = sync.Once{}
	buildInfoCacheMutex.Lock()
	buildInfoOnce = sync.Once{}
	buildInfoCache = buildInfoData{}
	buildInfoCacheMutex.Unlock()
}

// ResetSystemMetricsForTesting resets the system metrics initialization state for testing
// This should only be used in tests to ensure clean state between test runs
func ResetSystemMetricsForTesting(ctx context.Context) {
	systemResetMutex.Lock()
	defer systemResetMutex.Unlock()
	resetSystemMetrics(ctx)
}
