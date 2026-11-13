package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

var _ core.WindowManagerMigrationNotifications = (*windowManagerRegistry)(nil)
var _ windowmanager.MigrationAcknowledgementRepository = (*windowManagerRepository)(nil)

func (r *windowManagerRegistry) MigrationNotifications(
	ctx context.Context, workspaces []windowmanager.WorkspaceID,
) ([]windowmanager.MigrationNotice, error) {
	reader, ok := r.engine.(interface {
		DomainEntriesByPrefix(context.Context, []clientstate.WorkspaceID, string, string) (map[clientstate.WorkspaceID][]clientstate.Entry, error)
	})
	if !ok {
		return nil, errors.New("daemon: migration notification source is unavailable")
	}
	partitions := make([]clientstate.WorkspaceID, 0, len(workspaces))
	for _, workspace := range workspaces {
		partitions = append(partitions, clientstate.WorkspaceID(workspace))
	}
	entries, err := reader.DomainEntriesByPrefix(ctx, partitions, windowManagerStateDomain, statekeys.Migration(""))
	if err != nil {
		return nil, err
	}
	var notices []windowmanager.MigrationNotice
	for workspace, records := range entries {
		for _, entry := range records {
			profile, ok := statekeys.Profile(entry.Key)
			if !ok {
				continue
			}
			var status windowmanager.MigrationStatus
			if err := json.Unmarshal(entry.Value, &status); err != nil {
				return nil, fmt.Errorf("daemon: decode migration notification: %w", err)
			}
			if status.State == windowManagerMigrationMigrated && !status.Acknowledged && !status.At.IsZero() {
				notices = append(notices, windowmanager.MigrationNotice{
					WorkspaceID: windowmanager.WorkspaceID(workspace), ProfileID: profile, Status: status,
				})
			}
		}
	}
	return notices, nil
}

func (r *windowManagerRegistry) AcknowledgeMigrationNotification(
	ctx context.Context, notice windowmanager.MigrationNotice,
) error {
	manager, err := r.For(notice.ProfileID)
	if err != nil {
		return err
	}
	_, err = manager.AcknowledgeMigration(ctx, notice.WorkspaceID, notice.Status.At)
	return err
}
