package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const profileLifecycleEventWriteTimeout = 5 * time.Second

type profileLifecycleSurface interface {
	Recover(context.Context) error
	ListOps(context.Context) ([]profile.LifecycleOp, error)
	RetryOp(context.Context, string) (profile.LifecycleOp, error)
}

var _ profileLifecycleSurface = (*profile.Manager)(nil)

type daemonProfileEventRecorder struct {
	writer store.EventSummaryStore
	logger *slog.Logger
}

var _ profile.EventRecorder = (*daemonProfileEventRecorder)(nil)

func (d *Daemon) bootProfiles(
	ctx context.Context,
	state *bootState,
	database *globaldb.GlobalDB,
) (*profile.Manager, error) {
	recorder := &daemonProfileEventRecorder{writer: database, logger: state.logger}
	manager, err := profile.NewManager(
		profile.WithStore(database),
		profile.WithHomePaths(d.homePaths),
		profile.WithLogger(state.logger),
		profile.WithEventRecorder(recorder),
	)
	if err != nil {
		return nil, err
	}
	if err := manager.Recover(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}

func (r *daemonProfileEventRecorder) RecordProfileEvent(event profile.Event) {
	if r == nil || r.writer == nil {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		r.warn(event, "marshal", err)
		return
	}
	writeCtx, cancel := context.WithTimeout(context.Background(), profileLifecycleEventWriteTimeout)
	defer cancel()
	if err := r.writer.WriteEventSummary(writeCtx, store.EventSummary{
		ProfileID: event.ProfileID,
		Type:      event.Name,
		Outcome:   string(eventspkg.OutcomeFor(event.Name)),
		Content:   payload,
		Summary:   event.Name,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		r.warn(event, "write", err)
	}
}

func (r *daemonProfileEventRecorder) warn(event profile.Event, operation string, err error) {
	if r.logger == nil {
		return
	}
	r.logger.Warn(
		"profile lifecycle event recording failed",
		"event", event.Name,
		"operation_id", event.OperationID,
		"operation", operation,
		"error", err,
	)
}
