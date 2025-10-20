package monitoring

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
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
	buildInfoCache        atomic.Pointer[buildInfoData]
	buildInfoOnce         sync.Once
)

type buildInfoData struct {
	version   string
	commit    string
	goVersion string
}

// initSystemMetrics initializes system health metrics
func initSystemMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	systemResetMutex.Lock()
	defer systemResetMutex.Unlock()
	systemInitOnce.Do(func() {
		initializeSystemInstruments(ctx, meter)
	})
}

// initializeSystemInstruments creates system metrics instruments and callbacks.
func initializeSystemInstruments(ctx context.Context, meter metric.Meter) {
	log := logger.FromContext(ctx)
	startTime = time.Now()
	var err error
	buildInfo, err = createBuildInfoGauge(meter)
	if err != nil {
		log.Error("Failed to create build info gauge", "error", err)
		return
	}
	uptimeGauge, err = createUptimeGauge(meter)
	if err != nil {
		log.Error("Failed to create uptime gauge", "error", err)
		return
	}
	uptimeRegistration, err = registerUptimeCallback(meter)
	if err != nil {
		log.Error("Failed to register uptime callback", "error", err)
	}
	buildInfoRegistration, err = registerBuildInfoCallback(meter)
	if err != nil {
		log.Error("Failed to register build info callback", "error", err)
	}
}

// createBuildInfoGauge creates the build info observable gauge.
func createBuildInfoGauge(meter metric.Meter) (metric.Float64ObservableGauge, error) {
	return meter.Float64ObservableGauge(
		metrics.MetricNameWithSubsystem("system", "build_info"),
		metric.WithDescription("Build information (value=1)"),
	)
}

// createUptimeGauge creates the uptime observable gauge.
func createUptimeGauge(meter metric.Meter) (metric.Float64ObservableGauge, error) {
	return meter.Float64ObservableGauge(
		metrics.MetricNameWithSubsystem("system", "uptime_seconds"),
		metric.WithDescription("Service uptime in seconds"),
	)
}

// registerUptimeCallback registers the uptime observer callback.
func registerUptimeCallback(meter metric.Meter) (metric.Registration, error) {
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		uptime := time.Since(startTime).Seconds()
		o.ObserveFloat64(uptimeGauge, uptime)
		return nil
	}, uptimeGauge)
}

// registerBuildInfoCallback registers the build info observer callback.
func registerBuildInfoCallback(meter metric.Meter) (metric.Registration, error) {
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		versionVal, commit, goVersion := getBuildInfo()
		o.ObserveFloat64(buildInfo, 1,
			metric.WithAttributes(
				attribute.String("version", versionVal),
				attribute.String("commit_hash", commit),
				attribute.String("go_version", goVersion),
			),
		)
		return nil
	}, buildInfo)
}

// getBuildInfo returns build information with lazy loading and caching
func getBuildInfo() (version, commit, goVersion string) {
	buildInfoOnce.Do(func() {
		data := loadBuildInfo()
		buildInfoCache.Store(&data)
	})
	if data := buildInfoCache.Load(); data != nil {
		return data.version, data.commit, data.goVersion
	}
	fallback := loadBuildInfo()
	buildInfoCache.Store(&fallback)
	return fallback.version, fallback.commit, fallback.goVersion
}

// loadBuildInfo loads build information from various sources
func loadBuildInfo() buildInfoData {
	versionStr := version.GetVersion()
	commit := version.GetCommitHash()
	useLdflags := versionStr != defaultVersion && commit != defaultCommit
	if !useLdflags && !testing.Testing() {
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
	buildInfoOnce = sync.Once{}
	buildInfoCache = atomic.Pointer[buildInfoData]{}
}

// ResetSystemMetricsForTesting resets the system metrics initialization state for testing
// This should only be used in tests to ensure clean state between test runs
func ResetSystemMetricsForTesting(ctx context.Context) {
	systemResetMutex.Lock()
	defer systemResetMutex.Unlock()
	resetSystemMetrics(ctx)
}
