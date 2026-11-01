package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"fmt"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// CollectorFacts resolves identity from durable state and intersects caller reach with the instance ceiling.
func (r *InstanceRepo) CollectorFacts(
	ctx context.Context,
	id string,
	scope heartbeat.CollectorScope,
) (heartbeat.CollectorScope, heartbeat.CollectorFacts, error) {
	facts := heartbeat.CollectorFacts{}
	instance, err := r.Get(ctx, id)
	if err != nil {
		return scope, facts, err
	}
	scope.ProfileID, scope.WorkspaceID, scope.MainSessionID = instance.ProfileID, instance.WorkspaceID, instance.MainSessionID
	scope.Unrestricted = scope.Unrestricted && instance.GrantCeiling == "unrestricted"
	workspaces := []string{instance.WorkspaceID}
	if instance.WorkspaceID == "" && scope.Unrestricted {
		workspaces = append(workspaces, scope.ReachableWorkspaces...)
	}
	encoded, err := json.Marshal(workspaces)
	if err != nil {
		return scope, facts, fmt.Errorf("encode collector reach: %w", err)
	}
	key := sql.NullString{String: id, Valid: true}
	scope.ChainSessionIDs, err = r.queries.ListInstanceCollectorChain(ctx, key)
	if err != nil {
		return scope, facts, fmt.Errorf("read collector chain: %w", err)
	}
	tasks, err := r.queries.ListInstanceCollectorTasks(
		ctx,
		sqlcgen.ListInstanceCollectorTasksParams{
			InstanceID: key,
			ProfileID:  instance.ProfileID,
			Workspaces: string(encoded),
		},
	)
	if err != nil {
		return scope, facts, fmt.Errorf("collect tasks: %w", err)
	}
	for _, row := range tasks {
		facts.Tasks = append(
			facts.Tasks,
			heartbeat.TaskFact{
				ID:             row.ID,
				RunID:          row.RunID,
				ProfileID:      row.ProfileID,
				WorkspaceID:    row.WorkspaceID,
				OwnerKind:      "agent_session",
				OwnerSessionID: row.OwnerRef.String,
				Status:         row.Status,
			},
		)
	}
	sessions, err := r.queries.ListInstanceCollectorSessions(
		ctx,
		sqlcgen.ListInstanceCollectorSessionsParams{
			InstanceID: key,
			ProfileID:  instance.ProfileID,
			Workspaces: string(encoded),
		},
	)
	if err != nil {
		return scope, facts, fmt.Errorf("collect sessions: %w", err)
	}
	for _, row := range sessions {
		facts.Sessions = append(
			facts.Sessions,
			heartbeat.SessionFact{
				ID:          row.ID,
				ProfileID:   row.ProfileID,
				WorkspaceID: row.WorkspaceID,
				Health:      row.Health,
			},
		)
	}
	runs, err := r.queries.ListInstanceCollectorAutomations(
		ctx,
		sqlcgen.ListInstanceCollectorAutomationsParams{
			ProfileID:  sql.NullString{String: instance.ProfileID, Valid: true},
			Workspaces: string(encoded),
		},
	)
	if err != nil {
		return scope, facts, fmt.Errorf("collect automations: %w", err)
	}
	for _, row := range runs {
		endedAt, err := store.ParseTimestamp(row.EndedAt.String)
		if err != nil {
			return scope, facts, fmt.Errorf("collect automation %s: %w", row.ID, err)
		}
		facts.Automations = append(
			facts.Automations,
			heartbeat.AutomationFact{
				ID:          row.ID,
				ProfileID:   row.ProfileID.String,
				WorkspaceID: row.WorkspaceID.String,
				Status:      row.Status,
				Cursor:      store.FormatTimestamp(endedAt) + "|" + row.ID,
			},
		)
	}
	return scope, facts, nil
}
