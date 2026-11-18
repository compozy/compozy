package telemetry_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/version"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UT-016 (HTTP and gRPC unavailable), UT-028 (per-signal degradation and recovery)
func TestRuntimeStatusTracksExportFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should report an HTTP connection refusal as unavailable", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(nil), nil, &fakeNetwork{err: connectionRefused()})
		started.exportSpan(t)

		status := started.runtime.Status()
		if status.State != telemetry.StateDegraded || status.Traces.LastErrorClass != "unavailable" ||
			status.Traces.FailedExports != 1 {
			t.Fatalf("Status() = %+v, want degraded with one unavailable trace export", status)
		}
	})

	t.Run("Should report a gRPC Unavailable status as unavailable", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) {
			otel.Protocol = config.OTelProtocolGRPC
			otel.Metrics = false
		}), nil, &fakeNetwork{})
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		started.attemptSpanExport(ctx, t)

		status := started.runtime.Status()
		if status.State != telemetry.StateDegraded || status.Traces.LastErrorClass != "unavailable" ||
			status.Traces.FailedExports != 1 {
			t.Fatalf("Status() = %+v, want degraded with one unavailable trace export", status)
		}
	})

	t.Run("Should degrade per signal and recover once every signal succeeds", func(t *testing.T) {
		t.Parallel()

		network := &fakeNetwork{status: func(path string) int {
			if strings.HasSuffix(path, "/v1/metrics") {
				return http.StatusUnauthorized
			}
			return http.StatusOK
		}}
		started := startRuntime(t, exportConfig(nil), nil, network)
		started.exportSpan(t)
		started.exportMetrics(t.Context(), t)

		status := started.runtime.Status()
		if status.State != telemetry.StateDegraded || status.Traces.LastErrorClass != "" ||
			status.Traces.LastExportAt.IsZero() || status.Metrics.LastErrorClass != "http_401" ||
			status.Metrics.FailedExports != 1 {
			t.Fatalf("Status() = %+v, want degraded with ok traces and http_401 metrics", status)
		}

		network.setStatus(nil)
		started.exportMetrics(t.Context(), t)
		status = started.runtime.Status()
		if status.State != telemetry.StateOK || status.Metrics.LastErrorClass != "" ||
			status.Metrics.LastExportAt.IsZero() || status.Metrics.FailedExports != 1 {
			t.Fatalf("Status() = %+v, want ok with the metric class cleared and one failure kept", status)
		}
	})

	t.Run("Should log a failing signal at most once per minute with its class only", func(t *testing.T) {
		t.Parallel()

		network := &fakeNetwork{status: func(string) int { return http.StatusUnauthorized }}
		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Traces = false }),
			nil, network)
		started.exportMetrics(t.Context(), t)
		started.exportMetrics(t.Context(), t)

		logs := started.logs.String()
		if got := strings.Count(logs, "telemetry: export failed"); got != 1 {
			t.Fatalf("logs = %s, want one export failure line for two failures", logs)
		}
		if !strings.Contains(logs, "signal=metrics") || !strings.Contains(logs, "error_class=http_401") {
			t.Fatalf("logs = %s, want the signal and error class", logs)
		}
		if failed := started.runtime.Status().Metrics.FailedExports; failed != 2 {
			t.Fatalf("Status().Metrics.FailedExports = %d, want 2", failed)
		}
	})

	t.Run("Should count spans dropped while the export queue is full", func(t *testing.T) {
		t.Parallel()

		network := &fakeNetwork{hold: make(chan struct{})}
		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Metrics = false }),
			nil, network)
		t.Cleanup(func() { close(network.hold) })
		tracer := started.tracerProvider.Tracer(telemetry.ScopeSession)
		for range telemetry.SpanQueueSize + 10 {
			_, span := tracer.Start(t.Context(), telemetry.SpanSessionTurn)
			span.End()
		}

		if dropped := started.runtime.Status().DroppedSpans; dropped != 10 {
			t.Fatalf("Status().DroppedSpans = %d, want the 10 spans beyond the queue", dropped)
		}
	})

	// Dropping is only for a full queue: slots must come back once spans are exported, or a long-running daemon
	// would silently drop every span after its first SpanQueueSize.
	t.Run("Should keep admitting spans once earlier exports release their queue slots", func(t *testing.T) {
		t.Parallel()

		const batches, perBatch = 3, telemetry.SpanQueueSize / 2
		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Metrics = false }),
			nil, &fakeNetwork{})
		provider, ok := started.tracerProvider.(*sdktrace.TracerProvider)
		if !ok {
			t.Fatalf("installed tracer provider = %T, want the SDK provider", started.tracerProvider)
		}
		tracer := provider.Tracer(telemetry.ScopeSession)
		for range batches {
			for range perBatch {
				_, span := tracer.Start(t.Context(), telemetry.SpanSessionTurn)
				span.End()
			}
			if err := provider.ForceFlush(t.Context()); err != nil {
				t.Fatalf("TracerProvider.ForceFlush() error = %v", err)
			}
		}

		requests, _ := started.network.captured()
		exported := 0
		for _, request := range requests {
			exported += bytes.Count(request.body, []byte(telemetry.SpanSessionTurn))
		}
		if dropped := started.runtime.Status().DroppedSpans; dropped != 0 || exported != batches*perBatch {
			t.Fatalf("exported %d spans with %d dropped, want all %d exported across %d queue sizes",
				exported, dropped, batches*perBatch, batches*perBatch/telemetry.SpanQueueSize)
		}
	})

	t.Run("Should not queue or count unsampled spans", func(t *testing.T) {
		t.Parallel()

		network := &fakeNetwork{hold: make(chan struct{})}
		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) {
			otel.Metrics = false
			otel.SampleRatio = 0
		}), nil, network)
		t.Cleanup(func() { close(network.hold) })
		tracer := started.tracerProvider.Tracer(telemetry.ScopeSession)
		for range telemetry.SpanQueueSize + 10 {
			_, span := tracer.Start(t.Context(), telemetry.SpanSessionTurn)
			span.End()
		}
		started.exportSpan(t)

		requests, _ := started.network.captured()
		if dropped := started.runtime.Status().DroppedSpans; dropped != 0 || len(requests) != 0 {
			t.Fatalf("Status().DroppedSpans = %d with %d exports, want unsampled spans neither queued nor dropped",
				dropped, len(requests))
		}
	})
}

// UT-019
func TestRuntimeShutdownIsBoundedByItsContext(t *testing.T) {
	t.Parallel()

	t.Run("Should return DeadlineExceeded at the deadline when the exporter blocks", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			logs := &syncBuffer{}
			network := &fakeNetwork{hold: make(chan struct{})}
			var tracerProvider trace.TracerProvider
			runtime, err := telemetry.InitWithNetwork(
				t.Context(),
				exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Metrics = false }),
				nil,
				testRegistries(nil),
				version.Info{Version: testServiceVersion},
				slog.New(slog.NewTextHandler(logs, nil)),
				telemetry.Network{HTTP: network, Dial: network.dial},
				func(tp trace.TracerProvider, _ metric.MeterProvider) { tracerProvider = tp },
			)
			if err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			_, span := tracerProvider.Tracer(telemetry.ScopeSession).Start(t.Context(), telemetry.SpanSessionTurn)
			span.End()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			start := time.Now()
			err = runtime.Shutdown(ctx)
			// The bubble's clock stops once this function returns; the held export must finish before then.
			defer close(network.hold)

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Shutdown() error = %v, want context.DeadlineExceeded", err)
			}
			if elapsed := time.Since(start); elapsed != 5*time.Second {
				t.Fatalf("Shutdown() returned after %s, want the 5s deadline", elapsed)
			}
			if !strings.Contains(logs.String(), "telemetry: shutdown flush timed out") {
				t.Fatalf("logs = %s, want the flush timeout warning", logs.String())
			}
		})
	})
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestExportErrorClassUsesTheFixedClassList(t *testing.T) {
	t.Parallel()

	failure := errors.New("export failed")
	tests := []struct {
		name   string
		err    error
		status int64
		want   string
	}{
		{name: "Should classify HTTP 400", err: failure, status: http.StatusBadRequest, want: "http_400"},
		{name: "Should classify HTTP 401", err: failure, status: http.StatusUnauthorized, want: "http_401"},
		{name: "Should classify HTTP 403", err: failure, status: http.StatusForbidden, want: "http_403"},
		{name: "Should classify HTTP 404", err: failure, status: http.StatusNotFound, want: "http_404"},
		{name: "Should classify HTTP 413", err: failure, status: http.StatusRequestEntityTooLarge, want: "http_413"},
		{name: "Should classify HTTP 429", err: failure, status: http.StatusTooManyRequests, want: "http_429"},
		{
			name:   "Should classify HTTP 500 as http_5xx",
			err:    failure,
			status: http.StatusInternalServerError,
			want:   "http_5xx",
		},
		{
			name:   "Should classify HTTP 503 as http_5xx",
			err:    failure,
			status: http.StatusServiceUnavailable,
			want:   "http_5xx",
		},
		{name: "Should classify another HTTP 4xx as other", err: failure, status: http.StatusTeapot, want: "other"},
		{
			name: "Should classify gRPC Unavailable",
			err:  fmt.Errorf("export: %w", status.Error(codes.Unavailable, "down")),
			want: "unavailable",
		},
		{
			name: "Should prefer the gRPC status over the retry deadline",
			err:  fmt.Errorf("%w: %w", context.DeadlineExceeded, status.Error(codes.Unavailable, "down")),
			want: "unavailable",
		},
		{
			name: "Should classify gRPC DeadlineExceeded",
			err:  status.Error(codes.DeadlineExceeded, "slow"),
			want: "deadline_exceeded",
		},
		{
			name: "Should classify gRPC Unauthenticated",
			err:  status.Error(codes.Unauthenticated, "key"),
			want: "http_401",
		},
		{
			name: "Should classify gRPC PermissionDenied",
			err:  status.Error(codes.PermissionDenied, "key"),
			want: "http_403",
		},
		{
			name: "Should classify gRPC InvalidArgument",
			err:  status.Error(codes.InvalidArgument, "bad"),
			want: "http_400",
		},
		{name: "Should classify gRPC NotFound", err: status.Error(codes.NotFound, "path"), want: "http_404"},
		{
			name: "Should classify gRPC ResourceExhausted",
			err:  status.Error(codes.ResourceExhausted, "busy"),
			want: "http_429",
		},
		{name: "Should classify another gRPC code as other", err: status.Error(codes.Internal, "bug"), want: "other"},
		{
			name: "Should classify a certificate failure as tls",
			err: &url.Error{
				Op:  "Post",
				URL: "https://collector",
				Err: &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}},
			},
			want: "tls",
		},
		{name: "Should classify a TLS record failure as tls", err: tls.RecordHeaderError{Msg: "not tls"}, want: "tls"},
		{name: "Should classify a context deadline", err: context.DeadlineExceeded, want: "deadline_exceeded"},
		{
			name: "Should classify a client timeout",
			err:  &url.Error{Op: "Post", URL: "http://collector", Err: timeoutError{}},
			want: "deadline_exceeded",
		},
		{
			name: "Should classify a refused connection as unavailable",
			err:  &url.Error{Op: "Post", URL: "http://collector", Err: connectionRefused()},
			want: "unavailable",
		},
		{
			name: "Should classify a DNS failure as unavailable",
			err:  &net.DNSError{Err: "no such host", Name: "collector"},
			want: "unavailable",
		},
		{name: "Should classify a dropped connection as unavailable", err: io.ErrUnexpectedEOF, want: "unavailable"},
		{name: "Should classify anything else as other", err: failure, want: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := telemetry.ExportErrorClass(tt.err, tt.status); got != tt.want {
				t.Fatalf("ExportErrorClass(%v, %d) = %q, want %q", tt.err, tt.status, got, tt.want)
			}
		})
	}
}
