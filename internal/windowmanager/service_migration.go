package windowmanager

import (
	"context"
	"time"
)

// MigrationRepository owns the atomic stored-snapshot upgrade and its recovery record.
type MigrationRepository interface {
	Migration(context.Context, WorkspaceID, string, bool) (MigrationStatus, error)
}

type MigrationAcknowledgementRepository interface {
	AcknowledgeMigration(context.Context, WorkspaceID, time.Time) (MigrationStatus, error)
}

// AcknowledgeMigration acknowledges only the successful conversion observed by the bell.
func (m *Manager) AcknowledgeMigration(
	ctx context.Context, workspace WorkspaceID, at time.Time,
) (MigrationStatus, error) {
	if at.IsZero() {
		return MigrationStatus{}, ErrInvalidCommand
	}
	return m.migration(ctx, workspace, "ack", false, at)
}

// Migration reads or acknowledges the upgrade notice, or retries a preserved layout.
func (m *Manager) Migration(
	ctx context.Context,
	workspace WorkspaceID,
	action string,
	replace bool,
) (MigrationStatus, error) {
	return m.migration(ctx, workspace, action, replace, time.Time{})
}

func (m *Manager) migration(
	ctx context.Context, workspace WorkspaceID, action string, replace bool, at time.Time,
) (MigrationStatus, error) {
	if err := m.resolveWorkspace(ctx, workspace); err != nil {
		return MigrationStatus{}, err
	}
	lock, err := m.lockFor(workspace)
	if err != nil {
		return MigrationStatus{}, err
	}
	lock.Lock()
	defer m.releaseWorkspaceLock(workspace, lock)
	if _, err := m.loadSnapshot(ctx, workspace); err != nil {
		return MigrationStatus{}, err
	}
	repository, ok := m.repository.(MigrationRepository)
	if !ok {
		if action == "" || action == "ack" {
			return MigrationStatus{State: "none"}, nil
		}
		return MigrationStatus{}, ErrInvalidCommand
	}
	var status MigrationStatus
	if !at.IsZero() {
		acknowledger, ok := m.repository.(MigrationAcknowledgementRepository)
		if !ok {
			return MigrationStatus{}, ErrInvalidCommand
		}
		status, err = acknowledger.AcknowledgeMigration(ctx, workspace, at)
	} else {
		status, err = repository.Migration(ctx, workspace, action, replace)
	}
	if err != nil {
		return status, err
	}
	if action != "ack" && action != "retry" {
		return status, nil
	}
	snapshot, err := m.loadSnapshot(ctx, workspace)
	if err != nil {
		return status, err
	}
	if action == "retry" && status.State == "migrated" {
		request := CommandRequest{WorkspaceID: workspace, Payload: ReplaceLayoutCommand{}}
		projection, err := m.projectClientViews(workspace, snapshot, request)
		if err != nil {
			return status, err
		}
		m.applyClientProjection(workspace, projection)
		m.publishEvent(
			Event{
				WorkspaceID: workspace,
				Revision:    snapshot.Revision,
				CommandID:   CommandLayoutReplace,
				Origin:      "window-manager.migration.retry",
				OccurredAt:  snapshot.UpdatedAt,
			},
		)
		m.publishSnapshot(snapshot)
		for _, view := range projection.changed {
			m.publishClient(view)
		}
	} else {
		// Notice-only changes refresh subscribers without advancing topology or history.
		m.publishSnapshot(snapshot)
	}
	return status, nil
}
