package loop

import (
	"context"
	"errors"
	"strings"
	"sync"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// errLoopRunFailed marks a loop.run span whose run ended failed; its text is never exported.
var errLoopRunFailed = errors.New("loop: run failed")

// maxRunTraces bounds the open loop.run spans; past it an arbitrary span is ended early. ponytail: a run that never
// reports terminal (deleted, or open across a daemon restart) holds its entry until evicted; spans open at shutdown
// are not exported.
const maxRunTraces = 1024

// RunTraces owns one root loop.run span per live Loop run, from loop.started to loop.terminal, so the turns its nodes
// prompt become descendants of the run.
type RunTraces struct {
	mu    sync.Mutex
	spans map[RunID]trace.Span
}

// OnLoopStarted starts the run's loop.run root span.
func (t *RunTraces) OnLoopStarted(ctx context.Context, payload hookspkg.LoopStartedPayload) error {
	runID := RunID(strings.TrimSpace(payload.LoopRunID))
	if t == nil || runID == "" {
		return nil
	}
	_, span := telemetry.Tracer(telemetry.ScopeLoop).Start(ctx, telemetry.SpanLoopRun, trace.WithNewRoot())
	if !span.IsRecording() {
		span.End()
		return nil
	}
	span.SetAttributes(telemetry.AttrLoopID.String(string(runID)))
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.spans == nil {
		t.spans = make(map[RunID]trace.Span)
	}
	if _, ok := t.spans[runID]; ok {
		span.End()
		return nil
	}
	if len(t.spans) >= maxRunTraces {
		for id, evicted := range t.spans {
			evicted.End()
			delete(t.spans, id)
			break
		}
	}
	t.spans[runID] = span
	return nil
}

// OnLoopTerminal ends the run's loop.run span; a failed run carries an error status.
func (t *RunTraces) OnLoopTerminal(_ context.Context, payload hookspkg.LoopTerminalPayload) error {
	runID := RunID(strings.TrimSpace(payload.LoopRunID))
	if t == nil || runID == "" {
		return nil
	}
	t.mu.Lock()
	span, ok := t.spans[runID]
	delete(t.spans, runID)
	t.mu.Unlock()
	if !ok {
		return nil
	}
	if strings.TrimSpace(payload.Status) == string(StatusFailed) {
		telemetry.MarkError(span, errLoopRunFailed)
	}
	span.End()
	return nil
}

// ContextWithRun returns ctx parented under the run's loop.run span, or ctx unchanged when the run has none. Only the
// span context is attached, so cancellation and values of ctx are untouched.
func (t *RunTraces) ContextWithRun(ctx context.Context, runID RunID) context.Context {
	if t == nil {
		return ctx
	}
	t.mu.Lock()
	span, ok := t.spans[RunID(strings.TrimSpace(string(runID)))]
	t.mu.Unlock()
	if !ok {
		return ctx
	}
	return trace.ContextWithSpanContext(ctx, span.SpanContext())
}
