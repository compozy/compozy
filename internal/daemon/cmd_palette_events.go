package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

const cmdPaletteEventWriteTimeout = 5 * time.Second

type cmdPaletteEventWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type cmdPaletteEventRecorder struct {
	writer   cmdPaletteEventWriter
	logger   *slog.Logger
	failures atomic.Uint64
}

var _ cmdpalette.EventRecorder = (*cmdPaletteEventRecorder)(nil)

func (r *cmdPaletteEventRecorder) RecordCmdPaletteEvent(ctx context.Context, event cmdpalette.Event) {
	if r == nil {
		return
	}
	r.logInvocation(event)
	if r.writer == nil {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		r.logFailure(event, "marshal", err)
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cmdPaletteEventWriteTimeout)
	defer cancel()
	if err := r.writer.WriteEventSummary(writeCtx, store.EventSummary{
		ProfileID:   store.DefaultProfileID,
		WorkspaceID: string(event.WorkspaceID), Type: string(event.Name),
		Outcome: string(eventspkg.OutcomeFor(string(event.Name))), Content: payload,
		Summary: string(event.Name), Timestamp: event.OccurredAt,
	}); err != nil {
		r.logFailure(event, "write", err)
	}
}

func (r *cmdPaletteEventRecorder) logInvocation(event cmdpalette.Event) {
	if r.logger == nil || event.Name != cmdpalette.EventCommandInvoked {
		return
	}
	r.logger.Info(
		"cmd palette command invoked",
		"command_id", event.CommandID,
		"source", event.Source,
		"workspace_id", event.WorkspaceID,
		"exec_site", event.ExecutionSite,
		"outcome", event.Outcome,
		"duration_ms", event.DurationMS,
		"invocation_id", event.InvocationID,
		"approval_id", event.ApprovalID,
	)
}

func (r *cmdPaletteEventRecorder) logFailure(event cmdpalette.Event, operation string, err error) {
	if err == nil {
		return
	}
	failureCount := r.failures.Add(1)
	if r.logger == nil {
		return
	}
	r.logger.Warn(
		"cmd palette event recording failed",
		"event", event.Name,
		"workspace_id", event.WorkspaceID,
		"operation", operation,
		"failure_count", failureCount,
		"error", err,
	)
}
