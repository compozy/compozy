package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	defaultPoolLabel             = "default"
	postgresMeterName            = "compozy.postgres"
	maxWaitSamplesPerObservation = 128
)

var (
	postgresMetricsOnce            sync.Once
	postgresMetricsErr             error
	postgresConnectionsOpen        metric.Int64ObservableGauge
	postgresConnectionsInUse       metric.Int64ObservableGauge
	postgresConnectionsIdle        metric.Int64ObservableGauge
	postgresMaxConfiguredConns     metric.Int64ObservableGauge
	postgresConnectionWaitDuration metric.Float64Histogram
	postgresPools                  sync.Map
)

// poolMetrics tracks per-pool statistics required for async gauge observation and wait histograms.
type poolMetrics struct {
	label                 string
	pool                  atomic.Pointer[pgxpool.Pool]
	mu                    sync.Mutex
	lastEmptyAcquireCount int64
	lastEmptyAcquireWait  time.Duration
}

// configurePostgresMetrics prepares instrumentation hooks for the pgx pool configuration.
func configurePostgresMetrics(cfg *Config, poolCfg *pgxpool.Config) (*poolMetrics, error) {
	if cfg == nil || poolCfg == nil {
		return nil, nil
	}
	if err := ensurePostgresMetrics(); err != nil {
		return nil, fmt.Errorf("postgres: init metrics: %w", err)
	}
	metrics := &poolMetrics{label: computePoolLabel(cfg)}
	prevPrepare := poolCfg.PrepareConn
	poolCfg.PrepareConn = func(ctx context.Context, conn *pgx.Conn) (bool, error) {
		if prevPrepare != nil {
			ok, err := prevPrepare(ctx, conn)
			if !ok || err != nil {
				return ok, err
			}
		}
		metrics.recordWait(ctx)
		return true, nil
	}
	return metrics, nil
}

func ensurePostgresMetrics() error {
	postgresMetricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter(postgresMeterName)
		if err := initPostgresGauges(meter); err != nil {
			postgresMetricsErr = err
			return
		}
		if err := initPostgresHistogram(meter); err != nil {
			postgresMetricsErr = err
			return
		}
		postgresMetricsErr = registerPostgresCallback(meter)
	})
	return postgresMetricsErr
}

func initPostgresGauges(meter metric.Meter) error {
	var err error
	postgresConnectionsOpen, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem("postgres", "connections_open"),
		metric.WithDescription("Number of open Postgres connections"),
	)
	if err != nil {
		return err
	}
	postgresConnectionsInUse, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem("postgres", "connections_in_use"),
		metric.WithDescription("Number of Postgres connections currently in use"),
	)
	if err != nil {
		return err
	}
	postgresConnectionsIdle, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem("postgres", "connections_idle"),
		metric.WithDescription("Number of idle Postgres connections"),
	)
	if err != nil {
		return err
	}
	postgresMaxConfiguredConns, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem("postgres", "max_open_connections"),
		metric.WithDescription("Configured Postgres connection pool size"),
	)
	return err
}

func initPostgresHistogram(meter metric.Meter) error {
	var err error
	postgresConnectionWaitDuration, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("postgres", "connection_wait_duration_seconds"),
		metric.WithDescription("Time spent waiting for a connection from the pool"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2),
	)
	return err
}

func registerPostgresCallback(meter metric.Meter) error {
	if postgresConnectionsOpen == nil ||
		postgresConnectionsInUse == nil ||
		postgresConnectionsIdle == nil ||
		postgresMaxConfiguredConns == nil {
		return fmt.Errorf("postgres: metrics instruments not initialized")
	}
	_, err := meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			postgresPools.Range(func(_, value any) bool {
				poolMetrics, ok := value.(*poolMetrics)
				if !ok || poolMetrics == nil {
					return true
				}
				pool := poolMetrics.pool.Load()
				if pool == nil {
					return true
				}
				stats := pool.Stat()
				attrs := metric.WithAttributes(attribute.String("pool", poolMetrics.label))
				observer.ObserveInt64(postgresConnectionsOpen, int64(stats.TotalConns()), attrs)
				observer.ObserveInt64(postgresConnectionsInUse, int64(stats.AcquiredConns()), attrs)
				observer.ObserveInt64(postgresConnectionsIdle, int64(stats.IdleConns()), attrs)
				observer.ObserveInt64(postgresMaxConfiguredConns, int64(stats.MaxConns()), attrs)
				return true
			})
			return nil
		},
		postgresConnectionsOpen,
		postgresConnectionsInUse,
		postgresConnectionsIdle,
		postgresMaxConfiguredConns,
	)
	return err
}

func (p *poolMetrics) attach(pool *pgxpool.Pool) {
	if p == nil || pool == nil {
		return
	}
	p.pool.Store(pool)
	stats := pool.Stat()
	p.mu.Lock()
	p.lastEmptyAcquireCount = stats.EmptyAcquireCount()
	p.lastEmptyAcquireWait = stats.EmptyAcquireWaitTime()
	p.mu.Unlock()
	postgresPools.Store(p, p)
}

func (p *poolMetrics) unregister() {
	if p == nil {
		return
	}
	postgresPools.Delete(p)
	p.pool.Store(nil)
}

func (p *poolMetrics) recordWait(ctx context.Context) {
	if p == nil || postgresConnectionWaitDuration == nil {
		return
	}
	pool := p.pool.Load()
	if pool == nil {
		return
	}
	stats := pool.Stat()
	p.mu.Lock()
	defer p.mu.Unlock()
	currentCount := stats.EmptyAcquireCount()
	currentWait := stats.EmptyAcquireWaitTime()
	deltaCount := currentCount - p.lastEmptyAcquireCount
	deltaWait := currentWait - p.lastEmptyAcquireWait
	p.lastEmptyAcquireCount = currentCount
	p.lastEmptyAcquireWait = currentWait
	if deltaCount <= 0 || deltaWait <= 0 {
		return
	}
	averageSeconds := deltaWait.Seconds() / float64(deltaCount)
	if averageSeconds <= 0 {
		return
	}
	attrs := metric.WithAttributes(attribute.String("pool", p.label))
	if deltaCount > maxWaitSamplesPerObservation {
		postgresConnectionWaitDuration.Record(ctx, averageSeconds, attrs)
		return
	}
	for i := int64(0); i < deltaCount; i++ {
		postgresConnectionWaitDuration.Record(ctx, averageSeconds, attrs)
	}
}

func computePoolLabel(cfg *Config) string {
	if cfg == nil {
		return defaultPoolLabel
	}
	raw := []string{cfg.Host, cfg.Port, cfg.DBName}
	parts := make([]string, 0, len(raw))
	for _, c := range raw {
		if s := sanitizeLabelComponent(c); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return defaultPoolLabel
	}
	joined := strings.Join(parts, "-")
	return strings.Trim(strings.Trim(joined, "-"), "_")
}

func sanitizeLabelComponent(component string) string {
	trimmed := strings.TrimSpace(component)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	var builder strings.Builder
	for _, r := range lower {
		if isLabelRune(r) {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('_')
	}
	return strings.Trim(builder.String(), "_")
}

func isLabelRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '-', '.', ':':
		return true
	default:
		return false
	}
}
