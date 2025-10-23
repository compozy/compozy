package telemetry

import (
	"context"

	"github.com/compozy/compozy/pkg/logger"
)

type ctxKey string

const (
	runKey      ctxKey = "llm_run_ctx"
	recorderKey ctxKey = "llm_run_recorder"
	observerKey ctxKey = "llm_run_observer"
)

// Observer receives telemetry events in real time when attached to a context.
type Observer interface {
	OnEvent(ctx context.Context, evt *Event)
	OnTool(ctx context.Context, entry *ToolLogEntry)
}

// Logger returns a logger enriched with run metadata (run_id) when available.
func Logger(ctx context.Context) logger.Logger {
	log := logger.FromContext(ctx)
	if run, ok := RunFromContext(ctx); ok && run != nil && run.id != "" {
		return log.With("run_id", run.id)
	}
	return log
}

// ContextWithRun stores the active run handle in the context.
func ContextWithRun(ctx context.Context, run *Run) context.Context {
	return context.WithValue(ctx, runKey, run)
}

// RunFromContext extracts the run handle from context.
func RunFromContext(ctx context.Context) (*Run, bool) {
	run, ok := ctx.Value(runKey).(*Run)
	return run, ok && run != nil
}

// ContextWithRecorder stores the recorder reference on the context.
func ContextWithRecorder(ctx context.Context, rec RunRecorder) context.Context {
	return context.WithValue(ctx, recorderKey, rec)
}

func recorderFromContext(ctx context.Context) RunRecorder {
	if ctx == nil {
		return nil
	}
	if rec, ok := ctx.Value(recorderKey).(RunRecorder); ok {
		return rec
	}
	return nil
}

// ContextWithObserver attaches a telemetry observer to the context.
func ContextWithObserver(ctx context.Context, observer Observer) context.Context {
	if observer == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey, observer)
}

func observerFromContext(ctx context.Context) Observer {
	if ctx == nil {
		return nil
	}
	if obs, ok := ctx.Value(observerKey).(Observer); ok {
		return obs
	}
	return nil
}

// RunID extracts the run identifier from the context.
func RunID(ctx context.Context) (string, bool) {
	if run, ok := RunFromContext(ctx); ok {
		return run.id, true
	}
	return "", false
}

// CaptureContentEnabled reports whether the current run should record raw prompt/response bodies.
func CaptureContentEnabled(ctx context.Context) bool {
	if run, ok := RunFromContext(ctx); ok {
		return run.captureContent
	}
	return false
}

// contextThresholdTriggered returns the threshold that has just been exceeded.
func contextThresholdTriggered(ctx context.Context, pct float64) (float64, bool) {
	if run, ok := RunFromContext(ctx); ok {
		return run.thresholdTriggered(pct)
	}
	return 0, false
}

// NotifyContextUsage returns the first threshold exceeded by pct.
func NotifyContextUsage(ctx context.Context, pct float64) (float64, bool) {
	return contextThresholdTriggered(ctx, pct)
}
