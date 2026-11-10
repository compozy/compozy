package telemetry

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/internal/config"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

const (
	defaultHTTPEndpoint = "http://localhost:4318"
	defaultGRPCEndpoint = "http://localhost:4317"
	// exportRequestTimeout is the OTLP exporters' default per-request timeout.
	exportRequestTimeout = 10 * time.Second
)

// exportTransport is the exporters' network seam; the zero value uses the default HTTP transport and gRPC dialer.
type exportTransport struct {
	http http.RoundTripper
	dial func(context.Context, string) (net.Conn, error)
}

// exportTarget is where and how both signals are exported. Every exporter option is explicit, so OTEL_EXPORTER_OTLP_*
// variables the exporters read themselves never override what config resolved.
type exportTarget struct {
	protocol  string
	endpoint  *url.URL
	headers   map[string]string
	transport exportTransport
}

// exportEndpoint is the configured collector base URL, or the protocol's local default.
func exportEndpoint(cfg config.ObservabilityOTelConfig) (*url.URL, error) {
	raw := cfg.Endpoint
	switch {
	case raw != "":
	case cfg.Protocol == config.OTelProtocolGRPC:
		raw = defaultGRPCEndpoint
	default:
		raw = defaultHTTPEndpoint
	}
	return url.Parse(raw)
}

func (t exportTarget) spanExporter(ctx context.Context, tracker *signalTracker) (sdktrace.SpanExporter, error) {
	if t.protocol == config.OTelProtocolGRPC {
		options := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpointURL(t.endpoint.String()),
			otlptracegrpc.WithHeaders(t.headers),
		}
		if t.transport.dial != nil {
			options = append(options, otlptracegrpc.WithDialOption(grpc.WithContextDialer(t.transport.dial)))
		}
		return otlptracegrpc.New(ctx, options...)
	}
	return otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(t.endpoint.JoinPath("v1", "traces").String()),
		otlptracehttp.WithHeaders(t.headers),
		otlptracehttp.WithHTTPClient(t.httpClient(tracker)),
	)
}

func (t exportTarget) metricExporter(ctx context.Context, tracker *signalTracker) (sdkmetric.Exporter, error) {
	if t.protocol == config.OTelProtocolGRPC {
		options := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpointURL(t.endpoint.String()),
			otlpmetricgrpc.WithHeaders(t.headers),
		}
		if t.transport.dial != nil {
			options = append(options, otlpmetricgrpc.WithDialOption(grpc.WithContextDialer(t.transport.dial)))
		}
		return otlpmetricgrpc.New(ctx, options...)
	}
	return otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(t.endpoint.JoinPath("v1", "metrics").String()),
		otlpmetrichttp.WithHeaders(t.headers),
		otlpmetrichttp.WithHTTPClient(t.httpClient(tracker)),
	)
}

func (t exportTarget) httpClient(tracker *signalTracker) *http.Client {
	next := t.transport.http
	if next == nil {
		next = http.DefaultTransport
	}
	return &http.Client{Transport: &statusRecorder{next: next, tracker: tracker}, Timeout: exportRequestTimeout}
}

// statusRecorder records every response status on its signal's tracker: the OTLP/HTTP exporters report most HTTP
// failures only as text.
type statusRecorder struct {
	next    http.RoundTripper
	tracker *signalTracker
}

var _ http.RoundTripper = (*statusRecorder)(nil)

func (r *statusRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	var code int64
	if resp != nil {
		code = int64(resp.StatusCode)
	}
	r.tracker.httpStatus.Store(code)
	return resp, err
}
