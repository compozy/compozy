package telemetry

import "context"

type nopRecorder struct{}

// NopRecorder returns a recorder that does nothing.
func NopRecorder() RunRecorder {
	return nopRecorder{}
}

func (nopRecorder) StartRun(ctx context.Context, _ RunMetadata) (context.Context, *Run, error) {
	return ctx, nil, nil
}

func (nopRecorder) RecordEvent(context.Context, *Event) {}

func (nopRecorder) RecordTool(context.Context, *ToolLogEntry) {}

func (nopRecorder) CloseRun(context.Context, *Run, RunResult) error { return nil }
