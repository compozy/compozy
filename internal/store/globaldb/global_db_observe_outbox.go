package globaldb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func insertEventSummary(ctx context.Context, queries *sqlcgen.Queries, summary store.EventSummary) error {
	if err := queries.InsertEventSummaryOutbox(ctx, sqlcgen.InsertEventSummaryOutboxParams{
		ProfileID: summary.ProfileID,
		ID:        summary.ID, SessionID: summary.SessionID, WorkspaceID: summary.WorkspaceID,
		WorktreeID: summary.WorktreeID,
		Type:       summary.Type, AgentName: summary.AgentName, ContentJson: string(summary.ContentValue()),
		TaskID: summary.TaskID, RunID: summary.RunID, WorkflowID: summary.WorkflowID,
		ClaimTokenHash: summary.ClaimTokenHash, LeaseUntil: formatEventSummaryLeaseUntil(summary.LeaseUntil),
		CoordinatorSessionID: summary.CoordinatorSessionID, SchedulerReason: summary.SchedulerReason,
		HookEvent: summary.HookEvent, HookName: summary.HookName, ActorKind: summary.ActorKind,
		ActorID: summary.ActorID, ReleaseReason: summary.ReleaseReason,
		ParentSessionID: summary.ParentSessionID, RootSessionID: summary.RootSessionID,
		SpawnDepth: int64(summary.SpawnDepth), Provider: summary.Provider, Outcome: summary.Outcome,
		Summary: nullableObserveString(summary.Summary), Timestamp: store.FormatTimestamp(summary.Timestamp),
	}); err != nil {
		return fmt.Errorf("store: insert event summary: %w", err)
	}
	return nil
}

// drainCommittedEventSummaries does not turn an acknowledged core commit into
// an apparent rollback. The durable outbox retains failed deliveries for retry.
func (g *repoBase) drainCommittedEventSummaries(ctx context.Context) {
	if err := g.flushEventSummaryOutbox(ctx); err != nil {
		slog.WarnContext(ctx, "store: telemetry delivery deferred", "error", err)
	}
}

func (g *repoBase) flushEventSummaryOutbox(ctx context.Context) error {
	const batchSize = 128
	for {
		rows, err := g.readQueries.ListEventSummaryOutbox(ctx, batchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		summaries := make([]store.EventSummary, 0, len(rows))
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			summary, err := eventSummaryFromOutbox(row)
			if err != nil {
				return err
			}
			summaries = append(summaries, summary)
			ids = append(ids, row.ID)
		}
		if err := g.telemetry.WriteOutbox(ctx, summaries); err != nil {
			return err
		}
		if err := g.writeBackground(
			ctx,
			"acknowledge event summary outbox",
			func(ctx context.Context, tx *store.WriteTx) error {
				return sqlcgen.New(tx).DeleteEventSummaryOutbox(ctx, ids)
			},
		); err != nil {
			return err
		}
	}
}

func eventSummaryFromOutbox(row sqlcgen.EventSummaryOutbox) (store.EventSummary, error) {
	timestamp, err := store.ParseTimestamp(row.Timestamp)
	if err != nil {
		return store.EventSummary{}, err
	}
	lease, err := store.ParseNullableTimestamp(row.LeaseUntil)
	if err != nil {
		return store.EventSummary{}, err
	}
	summary := store.EventSummary{
		ID: row.ID, ProfileID: row.ProfileID, SessionID: row.SessionID, WorkspaceID: row.WorkspaceID,
		Type: row.Type, AgentName: row.AgentName, Provider: row.Provider, Outcome: row.Outcome,
		ParentSessionID: row.ParentSessionID, RootSessionID: row.RootSessionID, SpawnDepth: int(row.SpawnDepth),
		Summary: row.Summary.String, Timestamp: timestamp,
		EventCorrelation: store.EventCorrelation{
			WorktreeID: row.WorktreeID, TaskID: row.TaskID, RunID: row.RunID, WorkflowID: row.WorkflowID,
			ClaimTokenHash: row.ClaimTokenHash, LeaseUntil: lease, CoordinatorSessionID: row.CoordinatorSessionID,
			SchedulerReason: row.SchedulerReason, HookEvent: row.HookEvent, HookName: row.HookName,
			ActorKind: row.ActorKind, ActorID: row.ActorID, ReleaseReason: row.ReleaseReason,
		},
	}
	summary.SetContent([]byte(row.ContentJson))
	return summary, nil
}
