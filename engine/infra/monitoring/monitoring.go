package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	interceptorpkg "github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	"github.com/compozy/compozy/engine/infra/monitoring/middleware"
	"github.com/compozy/compozy/engine/llm/usage"
	builtin "github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	executionMetrics  *ExecutionMetrics
	llmUsageMetrics   usage.Metrics
}

// newDisabledService creates a service instance with no-op implementations
func newDisabledService(cfg *Config, initErr error) *Service {
	return &Service{
		config:            cfg,
		meter:             noop.NewMeterProvider().Meter("compozy"),
		initialized:       false,
		initializationErr: initErr,
		executionMetrics:  &ExecutionMetrics{},
		llmUsageMetrics:   &llmUsageMetrics{},
	}
}

// NewMonitoringService creates a new monitoring service with Prometheus exporter
func NewMonitoringService(ctx context.Context, cfg *Config) (*Service, error) {
	log := logger.FromContext(ctx)
	startTime := time.Now()
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		log.Debug("Monitoring disabled, using no-op meter")
		return newDisabledService(cfg, nil), nil
	}
	// Check for context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	registryStart := time.Now()
	registry := prom.NewRegistry()
	log.Debug("Created Prometheus registry", "duration", time.Since(registryStart))
	exporterStart := time.Now()
	exporter, err := prometheus.New(prometheus.WithRegisterer(registry))
	if err != nil {
		// Return error to let caller decide how to handle monitoring failure
		return nil, fmt.Errorf("failed to initialize Prometheus exporter: %w", err)
	}
	log.Debug("Created Prometheus exporter", "duration", time.Since(exporterStart))
	providerStart := time.Now()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	meter := provider.Meter("compozy")
	log.Debug("Created meter provider", "duration", time.Since(providerStart))
	service := &Service{
		meter:       meter,
		exporter:    exporter,
		provider:    provider,
		registry:    registry,
		config:      cfg,
		initialized: true,
	}
	execMetrics, err := newExecutionMetrics(meter)
	if err != nil {
		return nil, err
	}
	service.executionMetrics = execMetrics
	llmMetrics, err := newLLMUsageMetrics(meter)
	if err != nil {
		return nil, err
	}
	service.llmUsageMetrics = llmMetrics
	// Check for context cancellation before system metrics
	select {
	case <-ctx.Done():
		// Clean up if context was canceled
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if shutdownErr := provider.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Debug("Failed to shutdown provider during cancellation", "error", shutdownErr)
		}
		return nil, ctx.Err()
	default:
	}
	// Initialize system health metrics synchronously but quickly
	systemMetricsStart := time.Now()
	InitSystemMetrics(ctx, meter)
	log.Debug("Initialized system metrics", "duration", time.Since(systemMetricsStart))
	// Initialize dispatcher health metrics
	dispatcherMetricsStart := time.Now()
	InitDispatcherHealthMetrics(ctx, meter)
	log.Debug("Initialized dispatcher health metrics", "duration", time.Since(dispatcherMetricsStart))
	// Initialize memory monitoring metrics
	memoryMetricsStart := time.Now()
	InitializeMemoryMonitoring(ctx, meter)
	log.Debug("Initialized memory metrics", "duration", time.Since(memoryMetricsStart))
	// Initialize builtin tools metrics
	toolsMetricsStart := time.Now()
	if err := builtin.InitMetrics(meter); err != nil {
		log.Error("Failed to initialize cp__ tool metrics", "error", err)
	} else {
		log.Debug("Initialized cp__ tool metrics", "duration", time.Since(toolsMetricsStart))
	}
	log.Info("Monitoring service initialized successfully", "total_duration", time.Since(startTime))
	return service, nil
}

// Meter returns the OpenTelemetry meter for custom instrumentation
func (s *Service) Meter() metric.Meter {
	return s.meter
}

// ExecutionMetrics exposes execution-specific instruments to request handlers.
func (s *Service) ExecutionMetrics() *ExecutionMetrics {
	if s == nil {
		return nil
	}
	return s.executionMetrics
}

// LLMUsageMetrics exposes usage aggregation instruments for collectors.
func (s *Service) LLMUsageMetrics() usage.Metrics {
	if s == nil {
		return nil
	}
	return s.llmUsageMetrics
}

// GinMiddleware returns Gin middleware for HTTP metrics.
// Note: The OpenTelemetry tracing middleware (otelgin) should be applied separately
// when building the Gin router to ensure proper middleware chaining.
func (s *Service) GinMiddleware(ctx context.Context) gin.HandlerFunc {
	if !s.initialized {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	// Return only the custom HTTP metrics middleware
	return middleware.HTTPMetrics(ctx, s.meter)
}

// TemporalInterceptor returns Temporal interceptor for workflow metrics
func (s *Service) TemporalInterceptor(ctx context.Context) interceptor.WorkerInterceptor {
	if !s.initialized {
		return &interceptor.WorkerInterceptorBase{}
	}
	return interceptorpkg.TemporalMetrics(ctx, s.meter)
}

// ExporterHandler returns an HTTP handler for the /metrics endpoint
func (s *Service) ExporterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.initialized {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("Monitoring service not initialized")); err != nil {
				log := logger.FromContext(r.Context())
				log.Error("Failed to write response", "error", err)
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
func NewMonitoringServiceWithFallback(ctx context.Context, cfg *Config) *Service {
	log := logger.FromContext(ctx)
	service, err := NewMonitoringService(ctx, cfg)
	if err != nil {
		log.Error("Failed to initialize monitoring, using no-op implementation", "error", err)
		// Return a degraded service with no-op meter
		// The cfg is guaranteed to be non-nil here because NewMonitoringService
		// only returns an error for non-nil invalid configs
		return newDisabledService(cfg, err)
	}
	return service
}
