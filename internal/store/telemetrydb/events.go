package telemetrydb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb/sqlcgen"
)

func insertEventSummary(ctx context.Context, queries *sqlcgen.Queries, summary store.EventSummary) error {
	if err := queries.InsertEventSummary(ctx, eventSummaryParams(summary)); err != nil {
		return fmt.Errorf("store: insert event summary: %w", err)
	}
	return nil
}

func eventSummaryParams(summary store.EventSummary) sqlcgen.InsertEventSummaryParams {
	return sqlcgen.InsertEventSummaryParams{
		ProfileID: summary.ProfileID,
		ID:        summary.ID, SessionID: summary.SessionID, WorkspaceID: summary.WorkspaceID,
		WorktreeID: summary.WorktreeID,
		Type:       summary.Type, AgentName: summary.AgentName, ContentJson: string(summary.ContentValue()),
		TaskID: summary.TaskID, RunID: summary.RunID, WorkflowID: summary.WorkflowID,
		ClaimTokenHash: summary.ClaimTokenHash, LeaseUntil: eventSummaryLeaseUntil(summary.LeaseUntil),
		CoordinatorSessionID: summary.CoordinatorSessionID, SchedulerReason: summary.SchedulerReason,
		HookEvent: summary.HookEvent, HookName: summary.HookName, ActorKind: summary.ActorKind,
		ActorID: summary.ActorID, ReleaseReason: summary.ReleaseReason,
		ParentSessionID: summary.ParentSessionID, RootSessionID: summary.RootSessionID,
		SpawnDepth: int64(summary.SpawnDepth), Provider: summary.Provider, Outcome: summary.Outcome,
		Summary: store.SQLNullString(summary.Summary), Timestamp: store.FormatTimestamp(summary.Timestamp),
	}
}

func (s *Store) WriteOutbox(ctx context.Context, summaries []store.EventSummary) []error {
	operations := make([]store.BackgroundWriteOperation, 0, len(summaries))
	for _, summary := range summaries {
		operations = append(operations, store.BackgroundWriteOperation{
			Name: "deliver durable lifecycle event",
			Run: func(ctx context.Context, tx *store.WriteTx) error {
				queries := sqlcgen.New(tx)
				params := eventSummaryParams(summary)
				if err := queries.InsertEventSummaryIfAbsent(
					ctx,
					sqlcgen.InsertEventSummaryIfAbsentParams(params),
				); err != nil {
					return err
				}
				identical, err := queries.CountIdenticalEventSummary(
					ctx,
					sqlcgen.CountIdenticalEventSummaryParams(params),
				)
				if err != nil {
					return err
				}
				if identical != 1 {
					return errors.New("store: lifecycle event identity collides with different telemetry")
				}
				return nil
			},
		})
	}
	return s.background.SubmitBatch(ctx, operations)
}

func eventSummaryLeaseUntil(value *time.Time) string {
	if value == nil {
		return ""
	}
	return store.FormatTimestamp(*value)
}

func (s *Store) WriteSuggestion(
	ctx context.Context,
	summary store.EventSummary,
	validate func(context.Context) error,
) error {
	return s.background.Submit(
		ctx,
		"record automation suggestion transition",
		func(ctx context.Context, tx *store.WriteTx) error {
			if err := validate(ctx); err != nil {
				return err
			}
			return sqlcgen.New(tx).
				InsertAutomationSuggestionEventSummary(ctx, sqlcgen.InsertAutomationSuggestionEventSummaryParams{
					ProfileID:   summary.ProfileID,
					ID:          summary.ID,
					WorkspaceID: summary.WorkspaceID,
					Type:        summary.Type,
					ContentJson: string(summary.ContentValue()),
					ActorID:     summary.ActorID,
					Outcome:     summary.Outcome,
					Summary:     store.SQLNullString(summary.Summary),
					Timestamp:   store.FormatTimestamp(summary.Timestamp),
				})
		},
	)
}
