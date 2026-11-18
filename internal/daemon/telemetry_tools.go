package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/telemetry"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// toolCallSpan is tool.execute for one registry call and its compozy.tool.call.duration point; internal/tools cannot
// import telemetry, so the composition root supplies it.
type toolCallSpan struct {
	span  trace.Span
	id    toolspkg.ToolID
	start time.Time
}

func traceToolCall(ctx context.Context, id toolspkg.ToolID) (context.Context, toolspkg.CallSpan) {
	start := time.Now()
	ctx, span := telemetry.Tracer(telemetry.ScopeTools).Start(
		ctx,
		telemetry.SpanToolExecute,
		trace.WithTimestamp(start),
	)
	if span.IsRecording() {
		span.SetAttributes(telemetry.AttrTool.String(string(id)))
	}
	return ctx, toolCallSpan{span: span, id: id, start: start}
}

func (s toolCallSpan) SetRisk(risk toolspkg.RiskClass) {
	if s.span.IsRecording() {
		s.span.SetAttributes(telemetry.AttrToolRisk.String(string(risk)))
	}
}

func (s toolCallSpan) End(err error) {
	telemetry.EndSpan(s.span, err)
	ctx := context.Background()
	if histogram, ok := telemetry.DurationHistogram(ctx, telemetry.ScopeTools, telemetry.MetricToolCallDuration); ok {
		histogram.Record(ctx, time.Since(s.start).Seconds(), metric.WithAttributes(
			telemetry.AttrTool.String(string(s.id)),
			telemetry.AttrOutcome.String(telemetry.Outcome(err)),
		))
	}
}

// nativeToolMembership admits the daemon's native tool ids. Extension and MCP server tool names are user-authored, so
// they export as "other".
func nativeToolMembership(
	ctx context.Context,
	native toolspkg.Provider,
	logger *slog.Logger,
) func(string) bool {
	ids := make(map[string]struct{})
	if native != nil {
		descriptors, err := native.List(ctx, toolspkg.Scope{})
		if err != nil {
			logger.WarnContext(ctx, "daemon: list native tools for telemetry", "error", err)
		}
		for i := range descriptors {
			ids[descriptors[i].ID.String()] = struct{}{}
		}
	}
	return func(id string) bool {
		_, ok := ids[id]
		return ok
	}
}
