package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// errPromptTurnFailed classifies a turn that ended with a provider error; its text is never exported.
var errPromptTurnFailed = telemetry.ProviderError(errors.New("session: prompt turn failed"))

// promptTurnTrace owns one turn's session.turn span. The persistence pump ends it once the pump started; until then the
// submit path ends it as setup_failed. Only the first end takes effect, which also closes the turn's write account.
type promptTurnTrace struct {
	span    trace.Span
	start   time.Time
	account *store.TurnWriteAccount
	ended   atomic.Bool
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
	start := time.Now()
	options = append(options, trace.WithTimestamp(start))
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
	return ctx, &promptTurnTrace{span: span, start: start, account: store.NewTurnWriteAccount()}
}

// attributeWrites attaches the turn's write account to the pump's context as a value, so every write the pump
// persists counts toward this turn.
func (t *promptTurnTrace) attributeWrites(ctx context.Context) context.Context {
	if t == nil {
		return ctx
	}
	return store.WithWriteAttribution(ctx, store.WriteAttribution{Turn: t.account})
}

// end records the runtime that served the turn and how it ended, then ends the span and records the turn duration.
func (t *promptTurnTrace) end(session *Session, outcome string, err error) {
	if t == nil || !t.ended.CompareAndSwap(false, true) {
		return
	}
	flushes, writeTotal := t.account.Close()
	ctx := context.Background()
	histogram, record := telemetry.DurationHistogram(ctx, telemetry.ScopeSession, telemetry.MetricSessionTurnDuration)
	if !t.span.IsRecording() && !record {
		t.span.End()
		return
	}
	info := session.Info()
	attrs := []attribute.KeyValue{
		telemetry.AttrAgent.String(info.AgentName),
		telemetry.AttrGenAISystem.String(info.Provider),
		telemetry.AttrGenAIModel.String(info.Model),
		telemetry.AttrOutcome.String(outcome),
	}
	if t.span.IsRecording() {
		t.span.SetAttributes(attrs...)
		t.span.SetAttributes(
			telemetry.AttrDBFlushes.Int64(flushes),
			telemetry.AttrDBWriteMSTotal.Int64(writeTotal.Milliseconds()),
		)
		telemetry.MarkError(t.span, err)
	}
	t.span.End()
	if record {
		histogram.Record(ctx, time.Since(t.start).Seconds(), metric.WithAttributes(attrs...))
	}
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
