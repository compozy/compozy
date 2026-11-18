package telemetry_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/version"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// capturedRequest is one OTLP/HTTP request as it left the exporter.
type capturedRequest struct {
	url    string
	header http.Header
	body   []byte
}

// fakeNetwork stands in for the exporters' HTTP transport and gRPC dialer and records every attempt.
type fakeNetwork struct {
	mu       sync.Mutex
	requests []capturedRequest
	dials    []string
	// status answers each OTLP/HTTP request path; nil answers 200.
	status func(path string) int
	// err fails every HTTP request without a response.
	err error
	// hold, when set, keeps every HTTP request waiting until it is closed or the request ends.
	hold chan struct{}
	// connect, when set, completes gRPC dials instead of refusing them.
	connect func(ctx context.Context, addr string) (net.Conn, error)
}

func (n *fakeNetwork) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		read, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = read
	}
	n.mu.Lock()
	n.requests = append(n.requests, capturedRequest{url: req.URL.String(), header: req.Header.Clone(), body: body})
	status, failure, hold := n.status, n.err, n.hold
	n.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	if failure != nil {
		return nil, failure
	}
	code := http.StatusOK
	if status != nil {
		code = status(req.URL.Path)
	}
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func (n *fakeNetwork) dial(ctx context.Context, addr string) (net.Conn, error) {
	n.mu.Lock()
	n.dials = append(n.dials, addr)
	connect := n.connect
	n.mu.Unlock()
	if connect != nil {
		return connect(ctx, addr)
	}
	return nil, connectionRefused()
}

func (n *fakeNetwork) setStatus(status func(path string) int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status = status
}

func (n *fakeNetwork) captured() ([]capturedRequest, []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]capturedRequest(nil), n.requests...), append([]string(nil), n.dials...)
}

func connectionRefused() error {
	return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
}

// fakeResolver resolves header refs from a fixed table; any other ref is missing.
type fakeResolver map[string]string

func (r fakeResolver) ResolveRef(_ context.Context, ref string) (string, error) {
	value, ok := r[ref]
	if !ok {
		return "", fmt.Errorf("missing secret: %s", ref)
	}
	return value, nil
}

// loopbackDial completes a gRPC dial over the loopback interface.
func loopbackDial(ctx context.Context, addr string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", addr)
}

// metadataReceiver is an in-process gRPC server that records the incoming metadata of every call by method and
// rejects the call as Unauthenticated, a status the exporters do not retry.
type metadataReceiver struct {
	// endpoint is the http:// base URL the exporters dial.
	endpoint string

	mu    sync.Mutex
	calls map[string]metadata.MD
}

func newMetadataReceiver(t *testing.T) *metadataReceiver {
	t.Helper()
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenConfig.Listen() error = %v", err)
	}
	receiver := &metadataReceiver{endpoint: "http://" + listener.Addr().String(), calls: make(map[string]metadata.MD)}
	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(stream)
		incoming, _ := metadata.FromIncomingContext(stream.Context())
		receiver.mu.Lock()
		receiver.calls[method] = incoming
		receiver.mu.Unlock()
		return status.Error(codes.Unauthenticated, "rejected")
	}))
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("grpc Serve() error = %v", err)
		}
	}()
	t.Cleanup(server.Stop)
	return receiver
}

// metadata returns the metadata of the last call to method.
func (r *metadataReceiver) metadata(method string) metadata.MD {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[method]
}

// exportConfig is the default observability config with OTLP export enabled.
func exportConfig(mutate func(*config.ObservabilityOTelConfig)) config.ObservabilityConfig {
	cfg := config.DefaultWithHome(config.HomePaths{}).Observability
	cfg.OTel.Enabled = true
	if mutate != nil {
		mutate(&cfg.OTel)
	}
	return cfg
}

// environmentConfig applies the OTEL_* environment layer the way the daemon loads config, then mutate as the
// config.toml overlay would.
func environmentConfig(env map[string]string, mutate func(*config.ObservabilityOTelConfig)) config.ObservabilityConfig {
	full := config.DefaultWithHome(config.HomePaths{})
	config.ApplyOTelEnvironment(&full, func(key string) string { return env[key] })
	if mutate != nil {
		mutate(&full.Observability.OTel)
	}
	return full.Observability
}

// startedRuntime is a runtime started over a fake network with the providers it installed.
type startedRuntime struct {
	runtime        telemetry.Runtime
	network        *fakeNetwork
	logs           *syncBuffer
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

// startRuntime runs Init over network, capturing the providers it installs instead of replacing the globals, and
// shuts the runtime down at cleanup.
func startRuntime(
	t *testing.T,
	cfg config.ObservabilityConfig,
	refs telemetry.SecretResolver,
	network *fakeNetwork,
) *startedRuntime {
	t.Helper()
	return startRuntimeWithRegistries(t, cfg, refs, network, testRegistries(nil))
}

// startRuntimeWithRegistries is startRuntime with explicit sanitizer registries.
func startRuntimeWithRegistries(
	t *testing.T,
	cfg config.ObservabilityConfig,
	refs telemetry.SecretResolver,
	network *fakeNetwork,
	reg telemetry.Registries,
) *startedRuntime {
	t.Helper()
	started := &startedRuntime{network: network, logs: &syncBuffer{}}
	runtime, err := telemetry.InitWithNetwork(
		t.Context(),
		cfg,
		refs,
		reg,
		version.Info{Version: testServiceVersion},
		slog.New(slog.NewTextHandler(started.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		telemetry.Network{HTTP: network, Dial: network.dial},
		func(tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) {
			started.tracerProvider, started.meterProvider = tracerProvider, meterProvider
		},
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("Runtime.Shutdown() error = %v", err)
		}
	})
	started.runtime = runtime
	return started
}

// exportSpan ends one registered span and flushes the SDK tracer provider the runtime installed.
func (s *startedRuntime) exportSpan(t *testing.T) {
	t.Helper()
	s.exportSpanWith(t)
}

// exportSpanWith is exportSpan with span start options, such as attributes.
func (s *startedRuntime) exportSpanWith(t *testing.T, options ...trace.SpanStartOption) {
	t.Helper()
	_, span := s.tracerProvider.Tracer(telemetry.ScopeSession).Start(t.Context(), telemetry.SpanSessionTurn, options...)
	span.End()
	provider, ok := s.tracerProvider.(*sdktrace.TracerProvider)
	if !ok {
		t.Fatalf("installed tracer provider = %T, want the SDK provider", s.tracerProvider)
	}
	if err := provider.ForceFlush(t.Context()); err != nil {
		t.Fatalf("TracerProvider.ForceFlush() error = %v", err)
	}
}

// attemptSpanExport ends one span and flushes it within ctx, ignoring the flush error a retrying exporter returns
// when ctx ends. Shutdown then waits for the export still in flight, so its result is in Status afterwards.
func (s *startedRuntime) attemptSpanExport(ctx context.Context, t *testing.T) {
	t.Helper()
	_, span := s.tracerProvider.Tracer(telemetry.ScopeSession).Start(ctx, telemetry.SpanSessionTurn)
	span.End()
	provider, ok := s.tracerProvider.(*sdktrace.TracerProvider)
	if !ok {
		t.Fatalf("installed tracer provider = %T, want the SDK provider", s.tracerProvider)
	}
	if err := provider.ForceFlush(ctx); err != nil && ctx.Err() == nil {
		t.Fatalf("TracerProvider.ForceFlush() error = %v", err)
	}
	if err := s.runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Runtime.Shutdown() error = %v", err)
	}
}

// exportMetrics pushes one metric collection through the SDK meter provider the runtime installed.
func (s *startedRuntime) exportMetrics(ctx context.Context, t *testing.T) {
	t.Helper()
	provider, ok := s.meterProvider.(*sdkmetric.MeterProvider)
	if !ok {
		t.Fatalf("installed meter provider = %T, want the SDK provider", s.meterProvider)
	}
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("MeterProvider.ForceFlush() error = %v", err)
	}
}

// syncBuffer is a log sink safe for the exporters' goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
