package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	streamTracerName         = "compozy.stream"
	streamConnectedEvent     = "stream.connected"
	streamEventEmission      = "stream.event"
	streamClosedEvent        = "stream.closed"
	streamHeartbeatEventType = "heartbeat"
)

const (
	// StreamReasonInitializing is used while the stream is establishing initial state.
	StreamReasonInitializing = "initializing"
	// StreamReasonTerminal marks terminal status completions.
	StreamReasonTerminal = "terminal_status"
	// StreamReasonCompleted indicates successful stream completion.
	StreamReasonCompleted = "completed"
	// StreamReasonContextCanceled indicates the stream was canceled via context.
	StreamReasonContextCanceled = "context_canceled"
	// StreamReasonInitialSnapshotFailed indicates initial snapshot failed.
	StreamReasonInitialSnapshotFailed = "initial_snapshot_failed"
	// StreamReasonStreamError indicates a generic stream error occurred.
	StreamReasonStreamError = "stream_error"
)

type StreamTelemetry interface {
	Context() context.Context
	Connected(lastEventID int64, message string, fields ...any)
	RecordEvent(eventType string, countsAsFirst bool)
	RecordHeartbeat()
	Close(info *StreamCloseInfo)
}

type streamTelemetry struct {
	ctx                context.Context
	kind               string
	execID             string
	metrics            *monitoring.StreamingMetrics
	start              time.Time
	firstEventRecorded bool
	firstEventLatency  time.Duration
	events             int64
	span               trace.Span
	closeOnce          sync.Once
}

// NewStreamTelemetry initializes telemetry helpers for SSE connections.
func NewStreamTelemetry(
	ctx context.Context,
	kind string,
	execID core.ID,
	metrics *monitoring.StreamingMetrics,
) StreamTelemetry {
	if ctx == nil {
		return nil
	}
	tracer := otel.Tracer(streamTracerName)
	spanCtx, span := tracer.Start(
		ctx,
		fmt.Sprintf("stream.%s", kind),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("stream.kind", kind),
			attribute.String("stream.exec_id", execID.String()),
		),
	)
	telemetry := &streamTelemetry{
		ctx:     spanCtx,
		kind:    kind,
		execID:  execID.String(),
		metrics: metrics,
		start:   time.Now(),
		span:    span,
	}
	if metrics != nil {
		metrics.RecordConnect(spanCtx, kind)
	}
	return telemetry
}

func (t *streamTelemetry) Context() context.Context {
	if t == nil {
		return nil
	}
	return t.ctx
}

func (t *streamTelemetry) Connected(lastEventID int64, message string, fields ...any) {
	if t == nil {
		return
	}
	if log := logger.FromContext(t.ctx); log != nil {
		payload := append([]any{"exec_id", t.execID, "last_event_id", lastEventID}, fields...)
		log.Info(message, payload...)
	}
	if t.span != nil {
		t.span.AddEvent(
			streamConnectedEvent,
			trace.WithAttributes(
				attribute.Int64("stream.last_event_id", lastEventID),
			),
		)
	}
}

func (t *streamTelemetry) RecordEvent(eventType string, countsAsFirst bool) {
	if t == nil {
		return
	}
	t.events++
	if countsAsFirst && !t.firstEventRecorded {
		t.firstEventRecorded = true
		t.firstEventLatency = time.Since(t.start)
		if t.metrics != nil {
			t.metrics.RecordTimeToFirstEvent(t.ctx, t.kind, t.firstEventLatency)
		}
		if t.span != nil {
			t.span.SetAttributes(attribute.Float64("stream.time_to_first_event_seconds", t.firstEventLatency.Seconds()))
		}
	}
	if t.metrics != nil {
		t.metrics.RecordEvent(t.ctx, t.kind, eventType)
	}
	if t.span != nil {
		t.span.AddEvent(
			streamEventEmission,
			trace.WithAttributes(
				attribute.String("stream.event.type", eventType),
				attribute.Int64("stream.event.sequence", t.events),
			),
		)
	}
}

func (t *streamTelemetry) RecordHeartbeat() {
	if t == nil {
		return
	}
	t.RecordEvent(streamHeartbeatEventType, false)
}

type StreamCloseInfo struct {
	Reason      string
	Error       error
	Status      any
	LastEventID int64
	ExtraFields []any
}

func (t *streamTelemetry) Close(info *StreamCloseInfo) {
	if t == nil {
		return
	}
	t.closeOnce.Do(func() {
		t.handleClose(info)
	})
}

func (t *streamTelemetry) handleClose(info *StreamCloseInfo) {
	if info == nil {
		info = &StreamCloseInfo{Reason: "unknown"}
	}
	duration := time.Since(t.start)
	t.recordCloseMetrics(duration, info)
	fields := t.closeLogFields(duration, info)
	if info.Error != nil {
		t.logError(fields, info.Error)
	} else {
		t.logSuccess(fields)
	}
	t.finishSpan(duration, info)
}

func (t *streamTelemetry) recordCloseMetrics(duration time.Duration, info *StreamCloseInfo) {
	if t.metrics == nil || info == nil {
		return
	}
	t.metrics.RecordDisconnect(t.ctx, t.kind)
	t.metrics.RecordDuration(t.ctx, t.kind, duration)
	if info.Error != nil {
		t.metrics.RecordError(t.ctx, t.kind, info.Reason)
	}
}

func (t *streamTelemetry) closeLogFields(duration time.Duration, info *StreamCloseInfo) []any {
	if info == nil {
		return []any{
			"exec_id", t.execID,
			"duration", duration,
			"events", t.events,
		}
	}
	fields := []any{
		"exec_id", t.execID,
		"duration", duration,
		"events", t.events,
		"last_event_id", info.LastEventID,
		"reason", info.Reason,
	}
	if info.Status != nil {
		fields = append(fields, "status", info.Status)
	}
	if len(info.ExtraFields) > 0 {
		fields = append(fields, info.ExtraFields...)
	}
	return fields
}

func (t *streamTelemetry) logError(fields []any, err error) {
	if log := logger.FromContext(t.ctx); log != nil {
		log.Error("Stream terminated with error", append(fields, "error", err)...)
	}
	if t.span != nil {
		t.span.RecordError(err)
		t.span.SetStatus(codes.Error, err.Error())
	}
}

func (t *streamTelemetry) logSuccess(fields []any) {
	if log := logger.FromContext(t.ctx); log != nil {
		log.Info("Stream disconnected", fields...)
	}
	if t.span != nil {
		t.span.SetStatus(codes.Ok, "completed")
	}
}

func (t *streamTelemetry) finishSpan(duration time.Duration, info *StreamCloseInfo) {
	if t.span == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("stream.kind", t.kind),
		attribute.String("stream.exec_id", t.execID),
		attribute.Float64("stream.duration_seconds", duration.Seconds()),
		attribute.Int64("stream.events", t.events),
	}
	if info != nil {
		attrs = append(attrs,
			attribute.String("stream.reason", info.Reason),
			attribute.Int64("stream.last_event_id", info.LastEventID),
		)
		if info.Status != nil {
			attrs = append(attrs, attribute.String("stream.status", fmt.Sprint(info.Status)))
		}
	}
	if t.firstEventRecorded {
		attrs = append(attrs, attribute.Float64("stream.time_to_first_event_seconds", t.firstEventLatency.Seconds()))
	}
	t.span.AddEvent(streamClosedEvent, trace.WithAttributes(attrs...))
	t.span.End()
}
