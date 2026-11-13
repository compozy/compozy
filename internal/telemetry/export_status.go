package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Export error classes reported in status, besides the http_<status> classes.
const (
	exportClassUnavailable         = "unavailable"
	exportClassDeadlineExceeded    = "deadline_exceeded"
	exportClassTLS                 = "tls"
	exportClassHeaderRefUnresolved = "header_ref_unresolved"
	exportClassOther               = "other"
)

const (
	// exportLogInterval rate-limits each signal's export failure log.
	exportLogInterval = time.Minute
	// spanQueueSize bounds the spans waiting for export (the SDK batch processor default).
	spanQueueSize = 2048
)

// grpcCodeClasses maps gRPC status codes to the class of their HTTP counterpart.
var grpcCodeClasses = map[codes.Code]string{
	codes.InvalidArgument:   "http_400",
	codes.Unauthenticated:   "http_401",
	codes.PermissionDenied:  "http_403",
	codes.NotFound:          "http_404",
	codes.ResourceExhausted: "http_429",
	codes.Unavailable:       exportClassUnavailable,
	codes.DeadlineExceeded:  exportClassDeadlineExceeded,
}

// signalTracker records one signal's export results for status and rate-limits its failure log.
type signalTracker struct {
	name   string
	logger *slog.Logger
	// httpStatus is the last HTTP response status of the current export; 0 for gRPC or no response.
	httpStatus atomic.Int64

	mu       sync.Mutex
	status   SignalStatus
	loggedAt time.Time
}

func newSignalTracker(name string, logger *slog.Logger) *signalTracker {
	return &signalTracker{name: name, logger: logger, status: SignalStatus{Enabled: true}}
}

func (t *signalTracker) snapshot() SignalStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

func (t *signalTracker) fail(class string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.LastErrorClass = class
}

func (t *signalTracker) beginExport() {
	t.httpStatus.Store(0)
}

// record stores one export result. The tracking exporters never return err to the SDK, which would print the raw
// error text on every failed export; status and this rate-limited log carry the class instead.
func (t *signalTracker) record(err error) {
	now := time.Now()
	t.mu.Lock()
	if err == nil {
		t.status.LastExportAt = now.UTC()
		t.status.LastErrorClass = ""
		t.mu.Unlock()
		return
	}
	class := exportErrorClass(err, t.httpStatus.Load())
	t.status.LastErrorClass = class
	t.status.FailedExports++
	logFailure := now.Sub(t.loggedAt) >= exportLogInterval
	if logFailure {
		t.loggedAt = now
	}
	t.mu.Unlock()
	if logFailure {
		t.logger.Warn("telemetry: export failed", "signal", t.name, "error_class", class)
	}
}

// trackingSpanExporter records each export result and releases the exported spans from the gate.
type trackingSpanExporter struct {
	sdktrace.SpanExporter
	tracker *signalTracker
	release func(int)
}

var _ sdktrace.SpanExporter = (*trackingSpanExporter)(nil)

func newTrackingSpanExporter(
	next sdktrace.SpanExporter,
	tracker *signalTracker,
	release func(int),
) *trackingSpanExporter {
	return &trackingSpanExporter{SpanExporter: next, tracker: tracker, release: release}
}

func (e *trackingSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	defer e.release(len(spans))
	e.tracker.beginExport()
	e.tracker.record(e.SpanExporter.ExportSpans(ctx, spans))
	return nil
}

// trackingMetricExporter records each export result.
type trackingMetricExporter struct {
	sdkmetric.Exporter
	tracker *signalTracker
}

var _ sdkmetric.Exporter = (*trackingMetricExporter)(nil)

func newTrackingMetricExporter(next sdkmetric.Exporter, tracker *signalTracker) *trackingMetricExporter {
	return &trackingMetricExporter{Exporter: next, tracker: tracker}
}

func (e *trackingMetricExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	e.tracker.beginExport()
	e.tracker.record(e.Exporter.Export(ctx, metrics))
	return nil
}

// spanGate admits ended spans into the batch processor only while its queue has room and counts the rest: the SDK
// batch processor drops overflowing spans without reporting them. Exported spans release their slots. Only sampled
// spans end here, because the sampler never records a span it does not sample.
type spanGate struct {
	sdktrace.SpanProcessor
	inflight atomic.Int64
	dropped  atomic.Int64
}

var _ sdktrace.SpanProcessor = (*spanGate)(nil)

func newSpanGate(processor sdktrace.SpanProcessor) *spanGate {
	return &spanGate{SpanProcessor: processor}
}

func (g *spanGate) OnEnd(span sdktrace.ReadOnlySpan) {
	if g.inflight.Add(1) > spanQueueSize {
		g.inflight.Add(-1)
		g.dropped.Add(1)
		return
	}
	g.SpanProcessor.OnEnd(span)
}

func (g *spanGate) release(spans int) {
	g.inflight.Add(-int64(spans))
}

// exportErrorClass maps an export failure to its fixed status class. httpStatus is the last HTTP response status the
// exporter received, 0 for gRPC or when no response arrived. A gRPC status wins over the retry deadline wrapping it.
func exportErrorClass(err error, httpStatus int64) string {
	if httpStatus >= http.StatusBadRequest {
		return httpStatusClass(httpStatus)
	}
	if isTLSError(err) {
		return exportClassTLS
	}
	if grpcStatus, ok := status.FromError(err); ok {
		if class, mapped := grpcCodeClasses[grpcStatus.Code()]; mapped {
			return class
		}
		return exportClassOther
	}
	timeout, isTimeout := errors.AsType[interface {
		error
		Timeout() bool
	}](err)
	switch {
	case errors.Is(err, context.DeadlineExceeded), isTimeout && timeout.Timeout():
		return exportClassDeadlineExceeded
	case isNetworkError(err):
		return exportClassUnavailable
	default:
		return exportClassOther
	}
}

func httpStatusClass(code int64) string {
	switch code {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusRequestEntityTooLarge, http.StatusTooManyRequests:
		return "http_" + strconv.FormatInt(code, 10)
	}
	if code >= http.StatusInternalServerError && code <= 599 {
		return "http_5xx"
	}
	return exportClassOther
}

func isTLSError(err error) bool {
	_, verification := errors.AsType[*tls.CertificateVerificationError](err)
	_, record := errors.AsType[tls.RecordHeaderError](err)
	_, alert := errors.AsType[tls.AlertError](err)
	_, authority := errors.AsType[x509.UnknownAuthorityError](err)
	_, hostname := errors.AsType[x509.HostnameError](err)
	_, invalid := errors.AsType[x509.CertificateInvalidError](err)
	return verification || record || alert || authority || hostname || invalid
}

func isNetworkError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	_, operation := errors.AsType[*net.OpError](err)
	_, dns := errors.AsType[*net.DNSError](err)
	return operation || dns
}
