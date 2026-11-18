package telemetry

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/version"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Hooks for the external test package; production code builds the wrappers only inside this package.
var (
	NewSanitizer                = newSanitizer
	NewSanitizingSpanExporter   = newSanitizingSpanExporter
	NewSanitizingMetricExporter = newSanitizingMetricExporter
	NewResource                 = newResource
	MeterProviderOptions        = meterProviderOptions
	NewSampler                  = newSampler
	ExportErrorClass            = exportErrorClass
)

// SpanQueueSize is the bounded span queue; spans beyond it are dropped and counted.
const SpanQueueSize = spanQueueSize

// Network replaces the exporters' HTTP transport and gRPC dialer.
type Network struct {
	HTTP http.RoundTripper
	Dial func(context.Context, string) (net.Conn, error)
}

// InitWithNetwork is Init over network that hands the providers to install instead of the global setters.
func InitWithNetwork(
	ctx context.Context,
	cfg config.ObservabilityConfig,
	refs SecretResolver,
	reg Registries,
	info version.Info,
	logger *slog.Logger,
	network Network,
	install func(trace.TracerProvider, metric.MeterProvider),
) (Runtime, error) {
	return initRuntime(ctx, cfg, runtimeOptions{
		refs:      refs,
		reg:       reg,
		version:   info.Version,
		logger:    logger,
		transport: exportTransport{http: network.HTTP, dial: network.Dial},
		install:   install,
	})
}
