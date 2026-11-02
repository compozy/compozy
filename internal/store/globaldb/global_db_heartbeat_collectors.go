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
	scope.ProfileID = instance.ProfileID
	scope.WorkspaceID = instance.WorkspaceID
	scope.MainSessionID = instance.MainSessionID
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
	facts.Tasks = collectorTaskFacts(tasks)
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
	facts.Sessions = collectorSessionFacts(sessions)
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

func collectorTaskFacts(tasks []sqlcgen.ListInstanceCollectorTasksRow) []heartbeat.TaskFact {
	var facts []heartbeat.TaskFact
	for _, row := range tasks {
		facts = append(
			facts,
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
	return facts
}

func collectorSessionFacts(sessions []sqlcgen.ListInstanceCollectorSessionsRow) []heartbeat.SessionFact {
	var facts []heartbeat.SessionFact
	for _, row := range sessions {
		facts = append(
			facts,
			heartbeat.SessionFact{
				ID:          row.ID,
				ProfileID:   row.ProfileID,
				WorkspaceID: row.WorkspaceID,
				Health:      row.Health,
			},
		)
	}
	return facts
}
