package automation

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// startRunSpan starts the automation.run root span of one dispatch; the turns it prompts are its descendants.
func startRunSpan(ctx context.Context, req DispatchRequest) (context.Context, trace.Span) {
	ctx, span := telemetry.Tracer(telemetry.ScopeAutomation).Start(
		ctx,
		telemetry.SpanAutomationRun,
		trace.WithNewRoot(),
	)
	if span.IsRecording() {
		id := ""
		switch {
		case req.Job != nil:
			id = req.Job.ID
		case req.Trigger != nil:
			id = req.Trigger.ID
		}
		span.SetAttributes(telemetry.AttrAutomationID.String(id))
	}
	return ctx, span
}

// endRunSpan ends the dispatch span; a deferred admission or canceled dispatch carries no error status.
func endRunSpan(span trace.Span, err error) {
	if !errors.Is(err, errScheduledAdmissionDeferred) && !errors.Is(err, context.Canceled) {
		telemetry.MarkError(span, err)
	}
	span.End()
}
