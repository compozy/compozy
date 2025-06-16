package monitoring

import (
	"context"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.temporal.io/sdk/interceptor"
)

// Service encapsulates all monitoring and observability logic
type Service struct {
	meter             metric.Meter
	exporter          *prometheus.Exporter
	provider          *sdkmetric.MeterProvider
	registry          *prom.Registry
	config            *Config
	initialized       bool
	initializationErr error
}

// newDisabledService creates a service instance with no-op implementations
func newDisabledService(cfg *Config, initErr error) *Service {
	return &Service{
		config:            cfg,
		meter:             noop.NewMeterProvider().Meter("compozy"),
		initialized:       false,
		initializationErr: initErr,
	}
}

// NewMonitoringService creates a new monitoring service with Prometheus exporter
func NewMonitoringService(cfg *Config) (*Service, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		logger.Debug("Monitoring disabled, using no-op meter")
		return newDisabledService(cfg, nil), nil
	}
	registry := prom.NewRegistry()
	exporter, err := prometheus.New(prometheus.WithRegisterer(registry))
	if err != nil {
		// Return error to let caller decide how to handle monitoring failure
		return nil, fmt.Errorf("failed to initialize Prometheus exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	meter := provider.Meter("compozy")
	service := &Service{
		meter:       meter,
		exporter:    exporter,
		provider:    provider,
		registry:    registry,
		config:      cfg,
		initialized: true,
	}
	logger.Info("Monitoring service initialized successfully")
	return service, nil
}

// Meter returns the OpenTelemetry meter for custom instrumentation
func (s *Service) Meter() metric.Meter {
	return s.meter
}

// GinMiddleware returns Gin middleware for HTTP metrics
func (s *Service) GinMiddleware() gin.HandlerFunc {
	if !s.initialized {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return otelgin.Middleware("compozy")
}

// TemporalInterceptor returns Temporal interceptor for workflow metrics
func (s *Service) TemporalInterceptor() interceptor.WorkerInterceptor {
	// TODO: Implement Temporal metrics interceptor in Task 3
	// This will be implemented when we add Temporal workflow metrics collection
	return &interceptor.WorkerInterceptorBase{}
}

// ExporterHandler returns an HTTP handler for the /metrics endpoint
func (s *Service) ExporterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.initialized {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("Monitoring service not initialized")); err != nil {
				logger.Error("Failed to write response", "error", err)
			}
			return
		}
		promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
}

// Shutdown gracefully shuts down the monitoring service
func (s *Service) Shutdown(ctx context.Context) error {
	if s.provider != nil {
		return s.provider.Shutdown(ctx)
	}
	return nil
}

// IsInitialized returns whether the monitoring service was successfully initialized
func (s *Service) IsInitialized() bool {
	return s.initialized
}

// InitializationError returns any error that occurred during initialization
func (s *Service) InitializationError() error {
	return s.initializationErr
}

// SetAsGlobal sets this monitoring service's provider as the global OpenTelemetry meter provider
func (s *Service) SetAsGlobal() {
	if s.provider != nil {
		otel.SetMeterProvider(s.provider)
	}
}

// NewMonitoringServiceWithFallback creates a monitoring service with graceful degradation
// If monitoring initialization fails, it returns a service with no-op implementations
// and logs the error. This is useful for applications that should not fail due to
// monitoring initialization errors.
func NewMonitoringServiceWithFallback(cfg *Config) *Service {
	service, err := NewMonitoringService(cfg)
	if err != nil {
		logger.Error("Failed to initialize monitoring, using no-op implementation", "error", err)
		// Return a degraded service with no-op meter
		// The cfg is guaranteed to be non-nil here because NewMonitoringService
		// only returns an error for non-nil invalid configs
		return newDisabledService(cfg, err)
	}
	return service
}
