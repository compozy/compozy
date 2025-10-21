package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	interceptorpkg "github.com/compozy/compozy/engine/infra/monitoring/interceptor"
	"github.com/compozy/compozy/engine/infra/monitoring/middleware"
	factorymetrics "github.com/compozy/compozy/engine/llm/factory/metrics"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/usage"
	mcpmetrics "github.com/compozy/compozy/engine/mcp/metrics"
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
	meter              metric.Meter
	exporter           *prometheus.Exporter
	provider           *sdkmetric.MeterProvider
	registry           *prom.Registry
	config             *Config
	initialized        bool
	initializationErr  error
	executionMetrics   *ExecutionMetrics
	llmUsageMetrics    usage.Metrics
	llmProviderMetrics providermetrics.Recorder
}

// newDisabledService creates a service instance with no-op implementations
func newDisabledService(cfg *Config, initErr error) *Service {
	m := noop.NewMeterProvider().Meter("compozy")
	execMetrics, execErr := newExecutionMetrics(m)
	if execErr != nil || execMetrics == nil {
		execMetrics = &ExecutionMetrics{}
	}
	llmMetrics, llmErr := newLLMUsageMetrics(m)
	if llmErr != nil || llmMetrics == nil {
		llmMetrics = noopLLMUsageMetrics{}
	}
	providerMetrics, providerErr := providermetrics.NewRecorder(m)
	if providerErr != nil || providerMetrics == nil {
		providerMetrics = providermetrics.Nop()
	}
	return &Service{
		config:             cfg,
		meter:              m,
		initialized:        false,
		initializationErr:  initErr,
		executionMetrics:   execMetrics,
		llmUsageMetrics:    llmMetrics,
		llmProviderMetrics: providerMetrics,
	}
}

type otelComponents struct {
	registry *prom.Registry
	exporter *prometheus.Exporter
	provider *sdkmetric.MeterProvider
	meter    metric.Meter
}

func initOTEL(ctx context.Context) (*otelComponents, error) {
	log := logger.FromContext(ctx)
	registryStart := time.Now()
	registry := prom.NewRegistry()
	log.Debug("Created Prometheus registry", "duration", time.Since(registryStart))
	exporterStart := time.Now()
	exporter, err := prometheus.New(prometheus.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Prometheus exporter: %w", err)
	}
	log.Debug("Created Prometheus exporter", "duration", time.Since(exporterStart))
	providerStart := time.Now()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	meter := provider.Meter("compozy")
	log.Debug("Created meter provider", "duration", time.Since(providerStart))
	return &otelComponents{
		registry: registry,
		exporter: exporter,
		provider: provider,
		meter:    meter,
	}, nil
}

func buildMonitoringMetrics(meter metric.Meter) (*ExecutionMetrics, usage.Metrics, providermetrics.Recorder, error) {
	execMetrics, err := newExecutionMetrics(meter)
	if err != nil {
		return nil, nil, nil, err
	}
	llmMetrics, err := newLLMUsageMetrics(meter)
	if err != nil {
		return nil, nil, nil, err
	}
	providerMetrics, err := providermetrics.NewRecorder(meter)
	if err != nil {
		return nil, nil, nil, err
	}
	return execMetrics, llmMetrics, providerMetrics, nil
}

func initServiceMetrics(service *Service) error {
	execMetrics, llmMetrics, providerMetrics, err := buildMonitoringMetrics(service.meter)
	if err != nil {
		return err
	}
	service.executionMetrics = execMetrics
	service.llmUsageMetrics = llmMetrics
	service.llmProviderMetrics = providerMetrics
	return nil
}

func initSubsystemMetrics(ctx context.Context, meter metric.Meter, log logger.Logger) {
	systemMetricsStart := time.Now()
	InitSystemMetrics(ctx, meter)
	log.Debug("Initialized system metrics", "duration", time.Since(systemMetricsStart))
	dispatcherMetricsStart := time.Now()
	InitDispatcherHealthMetrics(ctx, meter)
	log.Debug("Initialized dispatcher health metrics", "duration", time.Since(dispatcherMetricsStart))
	factoryMetricsStart := time.Now()
	factorymetrics.Init(ctx, meter)
	log.Debug("Initialized factory metrics", "duration", time.Since(factoryMetricsStart))
	mcpMetricsStart := time.Now()
	mcpmetrics.Init(ctx, meter)
	log.Debug("Initialized MCP metrics", "duration", time.Since(mcpMetricsStart))
	memoryMetricsStart := time.Now()
	InitializeMemoryMonitoring(ctx, meter)
	log.Debug("Initialized memory metrics", "duration", time.Since(memoryMetricsStart))
	toolsMetricsStart := time.Now()
	if err := builtin.InitMetrics(meter); err != nil {
		log.Error("Failed to initialize cp__ tool metrics", "error", err)
	} else {
		log.Debug("Initialized cp__ tool metrics", "duration", time.Since(toolsMetricsStart))
	}
}

// NewMonitoringService creates a new monitoring service with Prometheus exporter.
func NewMonitoringService(ctx context.Context, cfg *Config) (*Service, error) {
	log := logger.FromContext(ctx)
	startTime := time.Now()
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(ctx); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		log.Debug("Monitoring disabled, using no-op meter")
		return newDisabledService(cfg, nil), nil
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	otel, err := initOTEL(ctx)
	if err != nil {
		return nil, err
	}
	service := &Service{
		meter:       otel.meter,
		exporter:    otel.exporter,
		provider:    otel.provider,
		registry:    otel.registry,
		config:      cfg,
		initialized: true,
	}
	if err := initServiceMetrics(service); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if shutdownErr := otel.provider.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Debug("Failed to shutdown provider during cancellation", "error", shutdownErr)
		}
		return nil, ctx.Err()
	default:
	}
	initSubsystemMetrics(ctx, otel.meter, log)
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

// LLMProviderMetrics exposes provider-call instruments to orchestrator components.
func (s *Service) LLMProviderMetrics() providermetrics.Recorder {
	if s == nil || s.llmProviderMetrics == nil {
		return providermetrics.Nop()
	}
	return s.llmProviderMetrics
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
		return newDisabledService(cfg, err)
	}
	return service
}
