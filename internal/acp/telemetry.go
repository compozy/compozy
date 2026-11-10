package acp

import (
	"context"
	"errors"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// TracedMethods are the ACP methods the driver traces as "acp <method>" client spans; the daemon registers them with
// the telemetry sanitizer.
func TracedMethods() []string {
	return []string{
		acpsdk.AgentMethodInitialize, acpsdk.AgentMethodSessionNew, acpsdk.AgentMethodSessionLoad,
		acpsdk.AgentMethodSessionFork, acpsdk.AgentMethodSessionSetMode, acpsdk.AgentMethodSessionSetConfigOption,
		acpsdk.AgentMethodSessionPrompt, steerExtensionMethod,
	}
}

// startSpawnSpan starts acp.process.spawn, which covers launching the provider and opening its ACP session.
func startSpawnSpan(ctx context.Context, provider string) (context.Context, trace.Span) {
	ctx, span := telemetry.Tracer(telemetry.ScopeACP).Start(ctx, telemetry.SpanACPProcessSpawn)
	if span.IsRecording() {
		span.SetAttributes(telemetry.AttrGenAISystem.String(provider))
	}
	return ctx, span
}

// startRPCSpan starts the client span of one ACP request.
func startRPCSpan(ctx context.Context, method string) (context.Context, trace.Span) {
	ctx, span := telemetry.Tracer(telemetry.ScopeACP).Start(
		ctx,
		"acp "+method,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	if span.IsRecording() {
		span.SetAttributes(
			telemetry.AttrRPCSystem.String(telemetry.RPCSystemJSONRPC),
			telemetry.AttrRPCMethod.String(method),
		)
	}
	return ctx, span
}

// endRPCSpan ends an ACP request span; an error the peer returned is classified as a provider error.
func endRPCSpan(span trace.Span, err error) {
	if _, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		err = telemetry.ProviderError(err)
	}
	telemetry.EndSpan(span, err)
}

// endPromptRPCSpan ends the session/prompt span with the time of the provider's first message chunk, when one came.
func endPromptRPCSpan(span trace.Span, active *activePromptState, stopReason acpsdk.StopReason, err error) {
	if at := active.firstOutputAt.Load(); at != 0 {
		span.AddEvent(telemetry.EventFirstToken, trace.WithTimestamp(time.Unix(0, at)))
	}
	if err == nil && stopReason == acpsdk.StopReasonCancelled {
		err = context.Canceled
	}
	endRPCSpan(span, err)
}

// withTraceParent adds the prompt span's W3C traceparent to the ACP _meta next to CompozyOS's own keys, so a provider
// that propagates it joins the trace.
func withTraceParent(ctx context.Context, meta map[string]any) map[string]any {
	traceParent := telemetry.TraceParent(ctx)
	if traceParent == "" {
		return meta
	}
	if meta == nil {
		meta = make(map[string]any, 1)
	}
	meta["traceparent"] = traceParent
	return meta
}

// startPermissionWaitSpan starts acp.permission.wait under the turn that sent the active prompt. Without a turn span
// it records nothing.
func startPermissionWaitSpan(ctx context.Context, turn trace.SpanContext) trace.Span {
	if !turn.IsValid() {
		return trace.SpanFromContext(context.Background())
	}
	_, span := telemetry.Tracer(telemetry.ScopeACP).Start(
		trace.ContextWithSpanContext(ctx, turn),
		telemetry.SpanACPPermissionWait,
	)
	return span
}

// endPermissionWaitSpan ends the wait with the decision the provider received.
func endPermissionWaitSpan(span trace.Span, decision permissionDecision) {
	outcome := telemetry.OutcomeCancelled
	switch decision {
	case decisionAllowOnce, decisionAllowAlways:
		outcome = telemetry.OutcomeApproved
	case decisionRejectOnce, decisionRejectAlways:
		outcome = telemetry.OutcomeDenied
	}
	span.SetAttributes(telemetry.AttrOutcome.String(outcome))
	span.End()
}
