package telemetry

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// State is the export state reported by status.
type State string

const (
	StateDisabled                      State = "disabled"
	StateInactiveObservabilityDisabled State = "inactive_observability_disabled"
	StateOK                            State = "ok"
	StateDegraded                      State = "degraded"
)

// SignalStatus is one signal's export health.
type SignalStatus struct {
	Enabled        bool
	LastExportAt   time.Time // zero until the first successful export
	LastErrorClass string    // fixed export error class; "" when the last export succeeded
	FailedExports  int64
}

// Status is the export state, target, and per-signal health. It never carries header values.
type Status struct {
	State              State
	Protocol           string // "grpc" | "http/protobuf"
	EndpointHost       string // host[:port] only; never userinfo, path, or query
	SampleRatio        float64
	ForwardToProviders bool
	Traces             SignalStatus
	Metrics            SignalStatus
	DroppedSpans       int64
}

// Runtime is a started export.
type Runtime interface {
	Status() Status
	// ProviderEnv returns the OTLP export variables forwarded to provider processes (endpoint, protocol, and
	// resolved headers), or nil unless forward_to_providers is on and export started.
	ProviderEnv() []string
	// Shutdown flushes pending telemetry within ctx; it returns ctx.Err() when the flush outlives ctx.
	Shutdown(ctx context.Context) error
}

// SecretResolver resolves env: and vault: header refs.
type SecretResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// Tracer returns the global tracer for an instrumentation scope (one of the Scope constants).
func Tracer(scope string) trace.Tracer {
	return otel.Tracer(scope)
}

// Meter returns the global meter for an instrumentation scope (one of the Scope constants).
func Meter(scope string) metric.Meter {
	return otel.Meter(scope)
}

// Init installs the global providers for cfg. Disabled or inactive export installs no-op providers. When export
// cannot start (a header ref that does not resolve, an exporter that cannot be built) Init logs it, installs no-op
// providers, and returns a runtime reporting StateDegraded: export never blocks daemon boot.
func Init(
	ctx context.Context,
	cfg config.ObservabilityConfig,
	refs SecretResolver,
	reg Registries,
	info version.Info,
	logger *slog.Logger,
) (Runtime, error) {
	return initRuntime(ctx, cfg, runtimeOptions{refs: refs, reg: reg, version: info.Version, logger: logger})
}

type runtimeOptions struct {
	refs      SecretResolver
	reg       Registries
	version   string
	logger    *slog.Logger
	transport exportTransport
	// install receives the providers; nil installs them as the OTel globals.
	install func(trace.TracerProvider, metric.MeterProvider)
}

type exportRuntime struct {
	base    Status
	traces  *signalTracker
	metrics *signalTracker
	gate    *spanGate
	logger  *slog.Logger
	// providerEnv is fixed at Init, so header refs are never resolved again per provider spawn.
	providerEnv []string

	shutdowns     []func(context.Context) error
	forgetSecrets func()
	shutdownOnce  sync.Once
	shutdownErr   error
}

var _ Runtime = (*exportRuntime)(nil)

func initRuntime(ctx context.Context, cfg config.ObservabilityConfig, opts runtimeOptions) (Runtime, error) {
	if ctx == nil {
		return nil, errors.New("telemetry: init context is required")
	}
	if opts.logger == nil {
		opts.logger = slog.Default()
	}
	if opts.install == nil {
		opts.install = installGlobalProviders
	}
	setModelClasses(nil)
	noop := func() { opts.install(tracenoop.NewTracerProvider(), metricnoop.NewMeterProvider()) }
	otelCfg := cfg.OTel
	switch {
	case !otelCfg.Enabled || otelCfg.EnvDisabled():
		noop()
		return newRuntime(Status{State: StateDisabled}, opts.logger), nil
	case !cfg.Enabled:
		noop()
		return newRuntime(Status{State: StateInactiveObservabilityDisabled}, opts.logger), nil
	}

	rt := newRuntime(Status{
		State:              StateOK,
		Protocol:           otelCfg.Protocol,
		SampleRatio:        otelCfg.SampleRatio,
		ForwardToProviders: otelCfg.ForwardToProviders,
	}, opts.logger)
	if otelCfg.Traces {
		rt.traces = newSignalTracker("traces", opts.logger)
	}
	if otelCfg.Metrics {
		rt.metrics = newSignalTracker("metrics", opts.logger)
	}
	degrade := func(class string) (Runtime, error) {
		rt.failSignals(class)
		noop()
		return rt, nil
	}
	endpoint, err := exportEndpoint(otelCfg)
	if err != nil {
		opts.logger.WarnContext(ctx, "telemetry: exporter setup failed", "error_class", exportClassOther)
		return degrade(exportClassOther)
	}
	rt.base.EndpointHost = endpoint.Host
	headers, ok := resolveHeaders(ctx, otelCfg, opts.refs, opts.logger)
	if !ok {
		return degrade(exportClassHeaderRefUnresolved)
	}
	rt.forgetSecrets = registerHeaderSecrets(headers)
	target := exportTarget{protocol: otelCfg.Protocol, endpoint: endpoint, headers: headers, transport: opts.transport}
	if err := rt.start(ctx, otelCfg, target, opts); err != nil {
		opts.logger.WarnContext(ctx, "telemetry: exporter setup failed", "error_class", exportClassOther)
		return degrade(exportClassOther)
	}
	if otelCfg.ForwardToProviders {
		rt.providerEnv = forwardedEnv(otelCfg.Protocol, endpoint, headers)
	}
	opts.logger.InfoContext(ctx, "telemetry: initialized", "protocol", otelCfg.Protocol, "endpoint_host", endpoint.Host)
	return rt, nil
}

func newRuntime(base Status, logger *slog.Logger) *exportRuntime {
	return &exportRuntime{base: base, logger: logger, forgetSecrets: func() {}}
}

func installGlobalProviders(tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) {
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
}

// start builds the exporters behind the sanitizer and installs the SDK providers for the enabled signals.
func (r *exportRuntime) start(
	ctx context.Context,
	cfg config.ObservabilityOTelConfig,
	target exportTarget,
	opts runtimeOptions,
) error {
	var spanExporter sdktrace.SpanExporter
	if r.traces != nil {
		exporter, err := target.spanExporter(ctx, r.traces)
		if err != nil {
			return err
		}
		spanExporter = exporter
	}
	var metricExporter sdkmetric.Exporter
	if r.metrics != nil {
		exporter, err := target.metricExporter(ctx, r.metrics)
		if err != nil {
			if spanExporter != nil {
				return errors.Join(err, spanExporter.Shutdown(ctx))
			}
			return err
		}
		metricExporter = exporter
	}

	res := newResource(cfg.ServiceName, opts.version, rand.Text())
	sanitizer := newSanitizer(opts.reg, res, opts.version)
	setModelClasses(opts.reg.Models)
	var tracerProvider trace.TracerProvider = tracenoop.NewTracerProvider()
	if spanExporter != nil {
		r.gate = newSpanGate(sdktrace.NewBatchSpanProcessor(
			newSanitizingSpanExporter(newTrackingSpanExporter(spanExporter, r.traces, r.releaseSpans), sanitizer),
			sdktrace.WithMaxQueueSize(spanQueueSize),
		))
		provider := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(newSampler(cfg.SampleRatio)),
			sdktrace.WithResource(res),
			sdktrace.WithSpanProcessor(r.gate),
		)
		r.shutdowns = append(r.shutdowns, provider.Shutdown)
		tracerProvider = provider
	}
	var meterProvider metric.MeterProvider = metricnoop.NewMeterProvider()
	if metricExporter != nil {
		reader := sdkmetric.NewPeriodicReader(
			newSanitizingMetricExporter(newTrackingMetricExporter(metricExporter, r.metrics), sanitizer),
			sdkmetric.WithInterval(cfg.MetricInterval),
		)
		provider := sdkmetric.NewMeterProvider(append(meterProviderOptions(res), sdkmetric.WithReader(reader))...)
		r.shutdowns = append(r.shutdowns, provider.Shutdown)
		meterProvider = provider
	}
	opts.install(tracerProvider, meterProvider)
	return nil
}

func (r *exportRuntime) releaseSpans(n int) {
	if r.gate != nil {
		r.gate.release(n)
	}
}

// failSignals marks every enabled signal failed before any export ran.
func (r *exportRuntime) failSignals(class string) {
	for _, tracker := range []*signalTracker{r.traces, r.metrics} {
		if tracker != nil {
			tracker.fail(class)
		}
	}
}

// Status reports the export state; State turns degraded while any enabled signal's last export failed.
func (r *exportRuntime) Status() Status {
	status := r.base
	if r.traces != nil {
		status.Traces = r.traces.snapshot()
	}
	if r.metrics != nil {
		status.Metrics = r.metrics.snapshot()
	}
	if r.gate != nil {
		status.DroppedSpans = r.gate.dropped.Load()
	}
	if status.State == StateOK && (status.Traces.LastErrorClass != "" || status.Metrics.LastErrorClass != "") {
		status.State = StateDegraded
	}
	return status
}

// ProviderEnv returns the forwarded OTLP variables; nil when forwarding is off or export did not start.
func (r *exportRuntime) ProviderEnv() []string {
	return slices.Clone(r.providerEnv)
}

// forwardedEnv renders the export target as OTEL_EXPORTER_OTLP_* variables; header values are percent-encoded as
// the OTLP exporter specification requires.
func forwardedEnv(protocol string, endpoint *url.URL, headers map[string]string) []string {
	env := []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT=" + endpoint.String(),
		"OTEL_EXPORTER_OTLP_PROTOCOL=" + protocol,
	}
	if len(headers) == 0 {
		return env
	}
	pairs := make([]string, 0, len(headers))
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		pairs = append(pairs, name+"="+escapeHeaderValue(headers[name]))
	}
	return append(env, "OTEL_EXPORTER_OTLP_HEADERS="+strings.Join(pairs, ","))
}

// escapeHeaderValue percent-encodes a forwarded header value; PathEscape keeps ',' literal, which would split the
// comma-separated OTEL_EXPORTER_OTLP_HEADERS list.
func escapeHeaderValue(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), ",", "%2C")
}

// Shutdown flushes both providers concurrently so a stalled signal cannot consume the other's share of ctx.
func (r *exportRuntime) Shutdown(ctx context.Context) error {
	r.shutdownOnce.Do(func() {
		r.shutdownErr = r.shutdown(ctx)
	})
	return r.shutdownErr
}

func (r *exportRuntime) shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("telemetry: shutdown context is required")
	}
	defer r.forgetSecrets()
	// Provider shutdowns honor ctx; one that outlives it is left to finish on its own.
	results := make(chan error, len(r.shutdowns))
	for _, shutdown := range r.shutdowns {
		go func() { results <- shutdown(ctx) }()
	}
	errs := make([]error, 0, len(r.shutdowns))
	for range r.shutdowns {
		select {
		case err := <-results:
			errs = append(errs, err)
		case <-ctx.Done():
		}
	}
	if err := ctx.Err(); err != nil {
		r.logger.Warn("telemetry: shutdown flush timed out")
		return err
	}
	return errors.Join(errs...)
}

// resolveHeaders returns the export headers: config.toml refs resolved, or OTEL_EXPORTER_OTLP_HEADERS when the file
// sets none. A ref that does not resolve is logged by name, never by value.
func resolveHeaders(
	ctx context.Context,
	cfg config.ObservabilityOTelConfig,
	refs SecretResolver,
	logger *slog.Logger,
) (map[string]string, bool) {
	if len(cfg.Headers) == 0 {
		return cfg.EnvHeaders(), true
	}
	headers := make(map[string]string, len(cfg.Headers))
	for _, name := range slices.Sorted(maps.Keys(cfg.Headers)) {
		ref := cfg.Headers[name]
		value, err := resolveRef(ctx, refs, ref)
		if err != nil {
			logger.WarnContext(ctx, "telemetry: header ref unresolved", "header", name, "ref", ref)
			return nil, false
		}
		headers[name] = value
	}
	return headers, true
}

func resolveRef(ctx context.Context, refs SecretResolver, ref string) (string, error) {
	if refs == nil {
		return "", errors.New("telemetry: no secret resolver")
	}
	return refs.ResolveRef(ctx, ref)
}

// registerHeaderSecrets registers resolved header values for redaction until shutdown. They are classified secrets, so
// short values register too (dynamic discovery skips values under eight characters).
func registerHeaderSecrets(headers map[string]string) func() {
	forget := make([]func(), 0, len(headers))
	for _, value := range headers {
		forget = append(forget, redact.RegisterRequiredSecret(value))
	}
	return func() {
		for _, cleanup := range forget {
			cleanup()
		}
	}
}

// newSampler keeps a sampled parent, and applies ratio to new traces and to unsampled remote parents, so the
// unsampled parents CompozyOS clients send never override daemon sampling.
func newSampler(ratio float64) sdktrace.Sampler {
	root := sdktrace.TraceIDRatioBased(ratio)
	return sdktrace.ParentBased(root, sdktrace.WithRemoteParentNotSampled(root))
}
