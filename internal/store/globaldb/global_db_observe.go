package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const eventSummaryContentPayloadKey = "payload"

// WriteEventSummary stores a lightweight cross-session summary entry.
func (g *ObserveRepo) WriteEventSummary(ctx context.Context, summary store.EventSummary) error {
	if err := g.checkReady(ctx, "write event summary"); err != nil {
		return err
	}
	if err := g.populateEventSummaryProjections(ctx, &summary); err != nil {
		return err
	}
	redactEventSummary(&summary)
	if err := summary.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(summary.ID) == "" {
		summary.ID = store.NewID("sum")
	}
	if summary.Timestamp.IsZero() {
		summary.Timestamp = g.now()
	}
	summary.EventCorrelation = summary.Normalize()

	if err := g.queries.InsertEventSummary(ctx, sqlcgen.InsertEventSummaryParams{
		ID: summary.ID, SessionID: summary.SessionID, WorkspaceID: summary.WorkspaceID,
		Type: summary.Type, AgentName: summary.AgentName, ContentJson: string(summary.Content),
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

func redactEventSummary(summary *store.EventSummary) {
	if summary == nil {
		return
	}
	summary.Summary = redact.String(summary.Summary)
	if len(summary.Content) == 0 {
		return
	}
	engine := redact.New(redact.Options{Disabled: !redact.Enabled()})
	summary.Content = engine.RedactJSON(summary.Content, []string{
		"body", "command", "content", "description", "detail", watchEventsContentErrorKey,
		"message", "output", eventSummaryContentPayloadKey, loopRunEventPayloadKeyReason, "result", "stderr", "stdout",
		loopRunEventPayloadKeySummary, loopRunEventPayloadKeyText, loopRunEventPayloadKeyTitle,
	})
}

// ListEventSummaries returns global event summaries filtered by the supplied options.
func (g *ObserveRepo) ListEventSummaries(
	ctx context.Context,
	query store.EventSummaryQuery,
) (summaries []store.EventSummary, err error) {
	if err := g.validateEventSummaryQuery(ctx, query); err != nil {
		return nil, err
	}

	// dynamic-sql: optional event/memory filters, registry-derived IN lists, UNION inclusion, and limit shape the query.
	eventQuery := `SELECT 0 AS source_rank, rowid AS source_rowid, id, session_id, workspace_id,` +
		` type, agent_name, provider, outcome, content_json, task_id, run_id, workflow_id, claim_token_hash,` +
		` lease_until, coordinator_session_id,
		scheduler_reason, hook_event, hook_name, actor_kind, actor_id, release_reason,
		parent_session_id, root_session_id, spawn_depth, summary, timestamp FROM event_summaries`
	eventWhere, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("session_id", query.SessionID),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("type", query.Type),
		store.StringClause("task_id", query.TaskID),
		store.StringClause("run_id", query.RunID),
		store.StringClause("actor_kind", query.ActorKind),
		store.StringClause("actor_id", query.ActorID),
		store.StringClause("provider", query.Provider),
		store.StringClause("outcome", query.Outcome),
		store.Int64Clause("rowid", ">", query.AfterSequence),
		store.TimeClause("timestamp", ">=", query.Since),
	)
	eventWhere, args = appendEventRegistryClauses(eventWhere, args, "type", query)
	eventQuery = store.AppendWhere(eventQuery, eventWhere)

	sqlQuery := eventSummaryListQuery(eventQuery, query.Limit)
	if query.Limit > 0 {
		args = append(args, query.Limit)
	}

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query event summaries: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close event summary rows: %w", closeErr))
		}
	}()

	summaries = make([]store.EventSummary, 0)
	for rows.Next() {
		summary, scanErr := scanEventSummary(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate event summaries: %w", err)
	}

	return summaries, nil
}

func appendEventRegistryClauses(
	where []string,
	args []any,
	column string,
	query store.EventSummaryQuery,
) ([]string, []any) {
	if shouldApplyErrorOnlyClause(query) {
		where = append(where, "outcome IN (?, ?)")
		args = append(args, string(eventspkg.OutcomeFailure), string(eventspkg.OutcomeWarning))
	} else if errorOnlyContradictsOutcome(query) {
		where = append(where, "1 = 0")
	}
	if names := eventRegistryNamesForQuery(query); len(names) > 0 {
		where, args = appendNamesClause(where, args, column, names)
	}
	return where, args
}

func eventRegistryNamesForQuery(query store.EventSummaryQuery) []string {
	component := strings.TrimSpace(query.Component)
	if component == "" {
		return nil
	}
	return eventspkg.NamesForComponent(component)
}

func shouldApplyErrorOnlyClause(query store.EventSummaryQuery) bool {
	return query.ErrorOnly && strings.TrimSpace(query.Outcome) == ""
}

func errorOnlyContradictsOutcome(query store.EventSummaryQuery) bool {
	if !query.ErrorOnly {
		return false
	}
	switch eventspkg.Outcome(strings.TrimSpace(query.Outcome)) {
	case "", eventspkg.OutcomeFailure, eventspkg.OutcomeWarning:
		return false
	default:
		return true
	}
}

func appendNamesClause(where []string, args []any, column string, names []string) ([]string, []any) {
	if len(names) == 0 {
		where = append(where, "1 = 0")
		return where, args
	}
	where = append(where, sqlInClause(column, len(names)))
	for _, name := range names {
		args = append(args, name)
	}
	return where, args
}

func sqlInClause(column string, count int) string {
	if count <= 0 {
		return "1 = 0"
	}
	placeholders := make([]string, count)
	for index := range placeholders {
		placeholders[index] = "?"
	}
	return column + " IN (" + strings.Join(placeholders, ", ") + ")"
}

func (g *ObserveRepo) populateEventSummaryProjections(ctx context.Context, summary *store.EventSummary) error {
	summary.WorkspaceID = strings.TrimSpace(summary.WorkspaceID)
	summary.Provider = strings.TrimSpace(summary.Provider)
	summary.Outcome = strings.TrimSpace(summary.Outcome)
	if strings.TrimSpace(summary.Outcome) == "" {
		summary.Outcome = string(eventspkg.OutcomeFor(summary.Type))
	}
	if strings.TrimSpace(summary.SessionID) == "" ||
		(summary.WorkspaceID != "" && summary.Provider != "") {
		return nil
	}

	row, err := g.queries.GetEventSummarySessionProjection(ctx, strings.TrimSpace(summary.SessionID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: resolve event summary projections: %w", err)
	}
	if summary.WorkspaceID == "" {
		summary.WorkspaceID = strings.TrimSpace(row.WorkspaceID)
	}
	if summary.Provider == "" {
		summary.Provider = strings.TrimSpace(row.Provider)
	}
	return nil
}

func eventSummaryListQuery(combinedQuery string, limit int) string {
	baseSelect := `SELECT source_rowid, id, session_id, workspace_id, type, agent_name, provider, outcome, content_json,` +
		` task_id, run_id, workflow_id, claim_token_hash, lease_until, coordinator_session_id,` +
		` scheduler_reason, hook_event,
		hook_name, actor_kind, actor_id, release_reason, parent_session_id, root_session_id,
		spawn_depth, summary, timestamp`
	if limit <= 0 {
		return baseSelect + ` FROM (` + combinedQuery + `) ORDER BY timestamp ASC, source_rank ASC, source_rowid ASC`
	}
	return baseSelect + ` FROM (` + combinedQuery + ` ORDER BY timestamp DESC, source_rank DESC, source_rowid DESC
		LIMIT ?) AS recent_summaries ORDER BY timestamp ASC, source_rank ASC, source_rowid ASC`
}

func (g *ObserveRepo) validateEventSummaryQuery(
	ctx context.Context,
	query store.EventSummaryQuery,
) error {
	if err := g.checkReady(ctx, "list event summaries"); err != nil {
		return err
	}
	return query.Validate()
}

// SweepObservability removes global observability rows older than cutoff.
