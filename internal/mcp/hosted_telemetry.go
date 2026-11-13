package mcp

import (
	"context"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// hostedRunTrace is the span context of the run a launch is bound to and of its last released run. It is guarded by
// HostedService.mu together with the run identity it belongs to.
type hostedRunTrace struct {
	runSpanCtx     trace.SpanContext
	lastRunSpanCtx trace.SpanContext
}

func (t *hostedRunTrace) bind(spanCtx trace.SpanContext) {
	t.runSpanCtx = spanCtx
}

func (t *hostedRunTrace) release() {
	t.lastRunSpanCtx = t.runSpanCtx
	t.runSpanCtx = trace.SpanContext{}
}

// startHostedCallSpan starts mcp.hosted.call parented by the run the daemon reports to the tool as RunID: the bound
// run's span, a root linked to the last run after it ended, or a root for an unknown bind. record is the snapshot
// read with that RunID, so the parent and RunID always agree.
func startHostedCallSpan(
	ctx context.Context,
	record *hostedBindRecord,
	toolID string,
) (context.Context, trace.Span, string) {
	opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindServer)}
	outcome := telemetry.OutcomeUnbound
	switch {
	case record != nil && record.runID != "":
		ctx = trace.ContextWithSpanContext(ctx, record.runTrace.runSpanCtx)
		outcome = ""
	case record != nil && record.runTrace.lastRunSpanCtx.IsValid():
		opts = append(
			opts,
			trace.WithNewRoot(),
			trace.WithLinks(trace.Link{SpanContext: record.runTrace.lastRunSpanCtx}),
		)
		outcome = telemetry.OutcomeAfterRun
	default:
		opts = append(opts, trace.WithNewRoot())
	}
	ctx, span := telemetry.Tracer(telemetry.ScopeMCP).Start(ctx, telemetry.SpanMCPHostedCall, opts...)
	if span.IsRecording() {
		span.SetAttributes(telemetry.AttrTool.String(toolID))
		if record != nil && record.runID != "" {
			span.SetAttributes(telemetry.AttrRunID.String(record.runID))
		}
	}
	return ctx, span, outcome
}

// endHostedCallSpan ends mcp.hosted.call. A fixed outcome (after_run, unbound) replaces the error-derived outcome;
// errors are still classified.
func endHostedCallSpan(span trace.Span, outcome string, err error) {
	if outcome == "" {
		telemetry.EndSpan(span, err)
		return
	}
	telemetry.MarkError(span, err)
	span.SetAttributes(telemetry.AttrOutcome.String(outcome))
	span.End()
}
