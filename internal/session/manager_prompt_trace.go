package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// errPromptTurnFailed classifies a turn that ended with a provider error; its text is never exported.
var errPromptTurnFailed = telemetry.ProviderError(errors.New("session: prompt turn failed"))

// promptTurnTrace owns one turn's session.turn span. The persistence pump ends it once the pump started; until then the
// submit path ends it as setup_failed. Only the first end takes effect.
type promptTurnTrace struct {
	span  trace.Span
	ended atomic.Bool
}

// startPromptTurnTrace starts session.turn under ctx's span, or, for a queued prompt, as a root linked to the spans
// that queued its inputs.
func startPromptTurnTrace(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (context.Context, *promptTurnTrace) {
	var options []trace.SpanStartOption
	if len(req.queuedLinks) > 0 {
		options = append(options, trace.WithNewRoot(), trace.WithLinks(req.queuedLinks...))
	}
	ctx, span := telemetry.Tracer(telemetry.ScopeSession).Start(ctx, telemetry.SpanSessionTurn, options...)
	if span.IsRecording() {
		info := session.Info()
		span.SetAttributes(
			telemetry.AttrSessionID.String(info.ID),
			telemetry.AttrTurnID.String(req.turnID),
			telemetry.AttrRunID.String(req.runID),
			telemetry.AttrWorkspaceID.String(info.WorkspaceID),
			telemetry.AttrProfileID.String(info.ProfileID),
		)
	}
	return ctx, &promptTurnTrace{span: span}
}

// end records the runtime that served the turn and how it ended, then ends the span.
func (t *promptTurnTrace) end(session *Session, outcome string, err error) {
	if t == nil || !t.ended.CompareAndSwap(false, true) {
		return
	}
	if t.span.IsRecording() {
		info := session.Info()
		t.span.SetAttributes(
			telemetry.AttrAgent.String(info.AgentName),
			telemetry.AttrGenAISystem.String(info.Provider),
			telemetry.AttrGenAIModel.String(info.Model),
			telemetry.AttrOutcome.String(outcome),
		)
		telemetry.MarkError(t.span, err)
	}
	t.span.End()
}

// failSetup ends the span as setup_failed when the submit path returns *err: the turn started once it owned the prompt
// slot, before runtime preparation, and the pump that would end it never started.
func (t *promptTurnTrace) failSetup(session *Session, err *error) {
	if *err != nil {
		t.end(session, telemetry.OutcomeSetupFailed, *err)
	}
}

// recovery marks a replay of the turn's prompt on a recovered runtime.
func (t *promptTurnTrace) recovery() {
	if t != nil {
		t.span.AddEvent(telemetry.EventRecovery)
	}
}

// promptTurnOutcome classifies how a pumped turn ended; a turn without a terminal event was canceled.
func promptTurnOutcome(state *promptTurnDispatchState, fatal *promptPumpFatal) (string, error) {
	switch {
	case state == nil:
		return telemetry.OutcomeCancelled, nil
	case state.failed || (fatal != nil && fatal.failure != nil):
		return telemetry.OutcomeError, errPromptTurnFailed
	case !state.turnEnded || state.stopReason == acp.PromptStopReasonCancelled:
		return telemetry.OutcomeCancelled, nil
	default:
		return telemetry.OutcomeOK, nil
	}
}

// endPromptTurnTrace ends a pumped turn's span with the outcome classified before the pump settled the turn.
func endPromptTurnTrace(session *Session, state *promptTurnDispatchState, outcome string, err error) {
	if state != nil {
		state.turnTrace.end(session, outcome, err)
	}
}

// maxQueuedTurnLinks bounds the remembered enqueue spans; past it an arbitrary entry is evicted. ponytail: inputs that
// never dispatch (canceled, superseded) keep their entry until evicted; prune on queue removal if the bound matters.
const maxQueuedTurnLinks = 1024

// queuedTurnLinks remembers the span that queued each input, so the turn that later runs the input links to it.
type queuedTurnLinks struct {
	mu    sync.Mutex
	links map[string]trace.SpanContext
}

// remember records the recording span in ctx as the one that queued entryID.
func (q *queuedTurnLinks) remember(ctx context.Context, entryID string) {
	span := trace.SpanFromContext(ctx)
	if entryID == "" || !span.IsRecording() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.links == nil {
		q.links = make(map[string]trace.SpanContext)
	}
	if len(q.links) >= maxQueuedTurnLinks {
		for id := range q.links {
			delete(q.links, id)
			break
		}
	}
	q.links[entryID] = span.SpanContext()
}

// take returns and forgets the links of an input dispatched with its batch.
func (q *queuedTurnLinks) take(entry queuedInput) []trace.Link {
	q.mu.Lock()
	defer q.mu.Unlock()
	var links []trace.Link
	for _, input := range append([]queuedInput{entry}, entry.batch...) {
		if sc, ok := q.links[input.id]; ok {
			links = append(links, trace.Link{SpanContext: sc})
			delete(q.links, input.id)
		}
	}
	return links
}
