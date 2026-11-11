package daemon

import (
	"context"
	"errors"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/telemetry"
)

// traceHookDispatch starts hook.dispatch under the caller's span; a denial is an outcome, not an error.
func traceHookDispatch(ctx context.Context, event hookspkg.HookEvent) (context.Context, func(bool, error)) {
	ctx, span := telemetry.Tracer(telemetry.ScopeHooks).Start(ctx, telemetry.SpanHookDispatch)
	if span.IsRecording() {
		span.SetAttributes(telemetry.AttrHookEvent.String(event.String()))
	}
	return ctx, func(denied bool, err error) {
		outcome := telemetry.OutcomeOK
		switch {
		case denied:
			outcome = telemetry.OutcomeDenied
		case errors.Is(err, context.Canceled):
			outcome = telemetry.OutcomeCancelled
		case err != nil:
			outcome = telemetry.OutcomeError
			telemetry.MarkError(span, err)
		}
		span.SetAttributes(telemetry.AttrOutcome.String(outcome))
		span.End()
	}
}
