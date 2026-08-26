package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const profileLifecycleEventWriteTimeout = 5 * time.Second

type daemonProfileEventRecorder struct {
	writer store.EventSummaryStore
	logger *slog.Logger
	now    func() time.Time
	state  *bootState
}

var _ profile.EventRecorder = (*daemonProfileEventRecorder)(nil)

func (d *Daemon) bootProfiles(
	state *bootState,
	database *globaldb.GlobalDB,
) (*profile.Manager, error) {
	recorder := &daemonProfileEventRecorder{writer: database, logger: state.logger, now: d.now, state: state}
	manager, err := profile.NewManager(
		profile.WithStore(database),
		profile.WithHomePaths(d.homePaths),
		profile.WithClock(d.now),
		profile.WithLogger(state.logger),
		profile.WithEventRecorder(recorder),
		profile.WithPlacementCatalog(extensionpkg.NewRegistry(database.DB())),
		profile.WithDesktopPartitionCatalog(profileDesktopPartitions{state: state}),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create profile manager: %w", err)
	}
	return manager, nil
}

// recoverProfiles resumes interrupted lifecycle operations.
//
// It runs after every dependency a journaled step can touch is wired — a delete
// that crashed between apply and finalize still owes a desktop purge, and resuming
// it before the window managers exist would fail the operation instead of
// completing it. Boot still reaches this well before any listener accepts traffic.
func recoverProfiles(ctx context.Context, state *bootState) error {
	if state == nil || state.profiles == nil {
		return nil
	}
	return state.profiles.Recover(ctx)
}

// profileDesktopPartitions bridges profile deletion to the per-profile window
// managers, which are composed after the profile manager itself.
type profileDesktopPartitions struct {
	state *bootState
}

var _ profile.DesktopPartitionCatalog = profileDesktopPartitions{}

func (p profileDesktopPartitions) CountDesktopPartitions(ctx context.Context, profileID string) (int, error) {
	registry, err := p.registry()
	if err != nil {
		return 0, err
	}
	return registry.CountDesktopPartitions(ctx, profileID)
}

func (p profileDesktopPartitions) PurgeDesktopPartitions(ctx context.Context, profileID string) error {
	registry, err := p.registry()
	if err != nil {
		return err
	}
	return registry.PurgeDesktopPartitions(ctx, profileID)
}

func (p profileDesktopPartitions) registry() (*windowManagerRegistry, error) {
	if p.state == nil || p.state.windowManagers == nil {
		return nil, errors.New("daemon: window managers are unavailable")
	}
	return p.state.windowManagers, nil
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
	if err := r.writer.WriteEventSummary(writeCtx, daemonEventSummary(store.EventSummary{
		ProfileID: profileEventSummaryOwnerID(event),
		Type:      event.Name,
		Outcome:   string(eventspkg.OutcomeFor(event.Name)),
		Summary:   event.Name,
		Timestamp: r.timestamp(),
	}, payload)); err != nil {
		r.warn(event, "write", err)
		return
	}
	r.republishProfileSkills(event)
}

func (r *daemonProfileEventRecorder) republishProfileSkills(event profile.Event) {
	if r == nil || r.state == nil || r.state.agentSkillResources == nil {
		return
	}
	if r.state.workspaceResolver != nil {
		r.state.workspaceResolver.InvalidateAll()
	}
	syncCtx, cancel := context.WithTimeout(context.Background(), profileLifecycleEventWriteTimeout)
	defer cancel()
	if err := syncSkillResources(syncCtx, r.state.agentSkillResources); err != nil {
		r.warn(event, "skill_resources", err)
	}
}

func profileEventSummaryOwnerID(event profile.Event) string {
	if event.Name == "profile.deleted" {
		return store.DefaultProfileID
	}
	return event.ProfileID
}

func (r *daemonProfileEventRecorder) timestamp() time.Time {
	if r != nil && r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
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
