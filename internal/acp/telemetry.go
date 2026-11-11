package acp

import (
	"context"
	"errors"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/metric"
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

// rpcSpan is one ACP request: its client span and what compozy.acp.rpc.duration records when it ends.
type rpcSpan struct {
	span     trace.Span
	provider string
	method   string
	start    time.Time
}

// startRPCSpan starts the client span of one ACP request to provider.
func startRPCSpan(ctx context.Context, provider, method string) (context.Context, rpcSpan) {
	start := time.Now()
	ctx, span := telemetry.Tracer(telemetry.ScopeACP).Start(
		ctx,
		"acp "+method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(start),
	)
	if span.IsRecording() {
		span.SetAttributes(
			telemetry.AttrRPCSystem.String(telemetry.RPCSystemJSONRPC),
			telemetry.AttrRPCMethod.String(method),
		)
	}
	return ctx, rpcSpan{span: span, provider: provider, method: method, start: start}
}

// endRPCSpan ends an ACP request span and records its duration; an error the peer returned is classified as a
// provider error.
func endRPCSpan(ctx context.Context, rpc rpcSpan, err error) {
	if _, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		err = telemetry.ProviderError(err)
	}
	telemetry.EndSpan(rpc.span, err)
	if histogram, ok := telemetry.DurationHistogram(ctx, telemetry.ScopeACP, telemetry.MetricACPRPCDuration); ok {
		histogram.Record(ctx, time.Since(rpc.start).Seconds(), metric.WithAttributes(
			telemetry.AttrGenAISystem.String(rpc.provider),
			telemetry.AttrRPCMethod.String(rpc.method),
			telemetry.AttrOutcome.String(telemetry.Outcome(err)),
		))
	}
}

// endPromptRPCSpan ends the session/prompt span with the time of the provider's first message chunk, when one came,
// and records that time to first token for the prompt's model.
func endPromptRPCSpan(
	ctx context.Context,
	rpc rpcSpan,
	active *activePromptState,
	model string,
	stopReason acpsdk.StopReason,
	err error,
) {
	if at := active.firstOutputAt.Load(); at != 0 {
		firstToken := time.Unix(0, at)
		rpc.span.AddEvent(telemetry.EventFirstToken, trace.WithTimestamp(firstToken))
		if histogram, ok := telemetry.DurationHistogram(ctx, telemetry.ScopeACP, telemetry.MetricSessionTTFT); ok {
			histogram.Record(ctx, max(firstToken.Sub(rpc.start), 0).Seconds(), metric.WithAttributes(
				telemetry.AttrGenAISystem.String(rpc.provider),
				telemetry.AttrGenAIModel.String(telemetry.ModelLabel(rpc.provider, model)),
			))
		}
	}
	if err == nil && stopReason == acpsdk.StopReasonCancelled {
		err = context.Canceled
	}
	endRPCSpan(ctx, rpc, err)
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
