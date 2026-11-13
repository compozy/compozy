package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

const (
	windowManagerMigrationMigrated = "migrated"
	windowManagerMigrationFailed   = "failed"
)

func windowManagerLegacyKey(profile string) string    { return statekeys.Legacy(profile) }
func windowManagerMigrationKey(profile string) string { return statekeys.Migration(profile) }

func (r *windowManagerRepository) readMigration(
	ctx context.Context,
	workspace windowmanager.WorkspaceID,
) (windowmanager.MigrationStatus, error) {
	entry, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspace),
		windowManagerStateDomain,
		windowManagerMigrationKey(r.profileID),
	)
	if errors.Is(err, clientstate.ErrNotFound) {
		return windowmanager.MigrationStatus{State: "none"}, nil
	}
	if err != nil {
		return windowmanager.MigrationStatus{}, mapWindowManagerStoreError("read migration", err)
	}
	var status windowmanager.MigrationStatus
	if err := json.Unmarshal(entry.Value, &status); err != nil {
		return status, fmt.Errorf("decode migration status: %w", err)
	}
	return status, nil
}

func (r *windowManagerRepository) migrateSnapshot(
	ctx context.Context,
	workspace windowmanager.WorkspaceID,
	entry clientstate.Entry,
) (windowmanager.Snapshot, error) {
	snapshot, summary, conversionErr := windowmanager.DecodeSnapshot(entry.Value)
	if conversionErr == nil && snapshot.WorkspaceID != workspace {
		conversionErr = errors.New("snapshot workspace mismatch")
	}
	status := windowmanager.MigrationStatus{
		State:       windowManagerMigrationMigrated,
		FromVersion: storedWindowManagerSnapshotVersion(entry.Value),
		At:          time.Now().UTC(),
		Summary:     &summary,
	}
	if conversionErr != nil {
		snapshot = windowmanager.DefaultSnapshot(workspace)
		status.State, status.Summary, status.Error = windowManagerMigrationFailed, nil, migrationDiagnostic(
			conversionErr,
		)
		status.FailureRevision = snapshot.Revision
	}
	backups, superseded, err := migrationBackupOps(
		ctx, r.service, clientstate.WorkspaceID(workspace), r.profileID, entry.Value,
		clientstate.Op{Kind: clientstate.OpCopy, SourceKey: entry.Key, SourceRev: entry.Rev},
	)
	if err != nil {
		return windowmanager.Snapshot{}, err
	}
	annotateSupersededBackups(&status, superseded)
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return windowmanager.Snapshot{}, err
	}
	record, err := json.Marshal(status)
	if err != nil {
		return windowmanager.Snapshot{}, err
	}
	ops := make([]clientstate.Op, len(backups), len(backups)+2)
	copy(ops, backups)
	ops = append(
		ops,
		clientstate.Op{
			Kind:  clientstate.OpPut,
			Key:   windowManagerSnapshotKey(r.profileID),
			Value: encoded,
			IfRev: entry.Rev,
		},
		clientstate.Op{Kind: clientstate.OpPut, Key: windowManagerMigrationKey(r.profileID), Value: record},
	)
	if _, err := r.service.Apply(
		ctx,
		clientstate.WorkspaceID(workspace),
		windowManagerStateDomain,
		ops,
		clientstate.ApplyOptions{Origin: "window-manager.snapshot.migrate"},
	); err != nil {
		return windowmanager.Snapshot{}, mapWindowManagerStoreError("migrate snapshot", err)
	}
	event := "window_manager.snapshot_migrated"
	if conversionErr != nil {
		event = "window_manager.snapshot_migration_failed"
	}
	r.logger.Info(
		event,
		"workspace_id",
		workspace,
		"profile_id",
		r.profileID,
		"from_version",
		status.FromVersion,
		"summary",
		status.Summary,
		"diagnostic",
		status.Error,
	)
	snapshot.Migration = &status
	return snapshot, nil
}

func migrationDiagnostic(err error) *windowmanager.Diagnostic {
	if topology, ok := errors.AsType[*windowmanager.TopologyError](err); ok && len(topology.Diagnostics) > 0 {
		return &topology.Diagnostics[0]
	}
	return &windowmanager.Diagnostic{Code: "migration.conversion_failed", Message: err.Error()}
}

func (r *windowManagerRepository) Migration(
	ctx context.Context,
	workspace windowmanager.WorkspaceID,
	action string,
	replace bool,
) (windowmanager.MigrationStatus, error) {
	lock := r.lockFor(workspace)
	lock.Lock()
	defer lock.Unlock()
	r.writeMu.RLock()
	defer r.writeMu.RUnlock()
	if r.sealed {
		return windowmanager.MigrationStatus{}, errWindowManagerProfileDeleted
	}
	status, err := r.readMigration(ctx, workspace)
	if err != nil || action == "" {
		return status, err
	}
	switch action {
	case "ack":
		status.Acknowledged = true
		return status, r.writeMigrationStatus(ctx, workspace, status, action)
	case "retry":
		return r.retrySnapshotMigration(ctx, workspace, status, replace)
	default:
		return status, windowmanager.ErrInvalidCommand
	}
}

func (r *windowManagerRepository) writeMigrationStatus(
	ctx context.Context,
	workspace windowmanager.WorkspaceID,
	status windowmanager.MigrationStatus,
	action string,
) error {
	record, err := json.Marshal(status)
	if err != nil {
		return err
	}
	_, err = r.service.Apply(ctx, clientstate.WorkspaceID(workspace), windowManagerStateDomain,
		[]clientstate.Op{{Kind: clientstate.OpPut, Key: windowManagerMigrationKey(r.profileID), Value: record}},
		clientstate.ApplyOptions{Origin: "window-manager.migration." + action})
	return mapWindowManagerStoreError("update migration", err)
}

func (r *windowManagerRepository) retrySnapshotMigration(
	ctx context.Context,
	workspace windowmanager.WorkspaceID,
	status windowmanager.MigrationStatus,
	replace bool,
) (windowmanager.MigrationStatus, error) {
	if status.State != windowManagerMigrationFailed {
		return status, fmt.Errorf("no failed migration: %w", windowmanager.ErrInvalidCommand)
	}
	current, revision, _, err := r.loadCurrent(ctx, workspace)
	if err != nil {
		return status, err
	}
	if !replace && current.Revision != status.FailureRevision {
		return status, &windowmanager.RevisionConflictError{Expected: status.FailureRevision, Current: current.Revision}
	}
	legacy, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspace),
		windowManagerStateDomain,
		windowManagerLegacyKey(r.profileID),
	)
	if err != nil {
		return status, err
	}
	snapshot, summary, conversionErr := windowmanager.DecodeSnapshot(legacy.Value)
	if conversionErr == nil && snapshot.WorkspaceID != workspace {
		conversionErr = errors.New("snapshot workspace mismatch")
	}
	status.Acknowledged, status.At = false, time.Now().UTC()
	if conversionErr != nil {
		status.Error = migrationDiagnostic(conversionErr)
		if err := r.annotateSupersededBackups(ctx, workspace, &status); err != nil {
			return status, err
		}
		return status, r.writeMigrationStatus(ctx, workspace, status, "retry")
	}
	next, err := windowmanager.NextTopologyRevision(current.Revision)
	if err != nil {
		return status, err
	}
	snapshot.Revision, snapshot.UpdatedAt, snapshot.Migration = next, status.At, nil
	status.State, status.Error, status.Summary = windowManagerMigrationMigrated, nil, &summary
	status.FailureRevision = 0
	if err := r.annotateSupersededBackups(ctx, workspace, &status); err != nil {
		return status, err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return status, err
	}
	record, err := json.Marshal(status)
	if err != nil {
		return status, err
	}
	_, err = r.service.Apply(ctx, clientstate.WorkspaceID(workspace), windowManagerStateDomain, []clientstate.Op{
		{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey(r.profileID), Value: encoded, IfRev: revision},
		{Kind: clientstate.OpPut, Key: windowManagerMigrationKey(r.profileID), Value: record},
	}, clientstate.ApplyOptions{Origin: "window-manager.migration.retry"})
	return status, mapWindowManagerStoreError("retry migration", err)
}

func (r *windowManagerRepository) purgePartition(ctx context.Context, workspace windowmanager.WorkspaceID) error {
	return purgeWindowManagerPartition(ctx, r.service, clientstate.WorkspaceID(workspace), r.profileID)
}

func purgeWindowManagerPartition(
	ctx context.Context,
	service clientstate.Service,
	workspace clientstate.WorkspaceID,
	profile string,
) error {
	entries, err := service.List(ctx, workspace, windowManagerStateDomain)
	if err != nil {
		return err
	}
	var ops []clientstate.Op
	for _, entry := range entries {
		owner, partitioned := statekeys.Profile(entry.Key)
		if !entry.Deleted && partitioned && owner == profile {
			ops = append(ops, clientstate.Op{Kind: clientstate.OpDelete, Key: entry.Key, IfRev: entry.Rev})
		}
	}
	if len(ops) == 0 {
		return nil
	}
	_, err = service.Apply(
		ctx,
		workspace,
		windowManagerStateDomain,
		ops,
		clientstate.ApplyOptions{Origin: "window-manager.partition.purge"},
	)
	return mapWindowManagerStoreError("purge partition", err)
}
