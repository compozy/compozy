package globaldb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const automationTriggerCatalogWorkspaceIndex = "idx_automation_trigger_catalog_workspace_order"

const automationTriggerCatalogBaseSQL = ` FROM automation_trigger_catalog_entries AS c
	LEFT JOIN automation_trigger_overlays AS o ON o.trigger_id = c.trigger_id`

func listAutomationTriggerCatalog(
	ctx context.Context,
	executor automationCatalogExecutor,
	query automation.TriggerListQuery,
) (automation.TriggerListPage, error) {
	cursor, hasCursor, err := automation.TriggerListCursor(query)
	if err != nil {
		return automation.TriggerListPage{}, err
	}
	where, args := automationTriggerCatalogWhere(query)
	var total int
	// dynamic-sql: optional catalog filters change the count predicate set.
	if err := executor.QueryRowContext(
		ctx,
		`SELECT COUNT(*)`+automationTriggerCatalogBaseSQL+where,
		args...,
	).Scan(&total); err != nil {
		return automation.TriggerListPage{}, fmt.Errorf("store: count automation trigger catalog: %w", err)
	}
	candidateWhere, candidateArgs := where, append([]any(nil), args...)
	if hasCursor {
		candidateWhere = appendAutomationCatalogPredicate(
			candidateWhere,
			`(c.source_rank, c.name, c.trigger_id) > (?, ?, ?)`,
		)
		candidateArgs = append(
			candidateArgs,
			automation.ListSourceRank(cursor.Source),
			cursor.Name,
			cursor.ID,
		)
	}
	candidateArgs = append(candidateArgs, query.Limit+1)
	candidates, err := readAutomationTriggerCatalogCandidates(
		ctx,
		executor,
		candidateWhere,
		candidateArgs,
	)
	if err != nil {
		return automation.TriggerListPage{}, err
	}
	hasMore := len(candidates) > query.Limit
	if hasMore {
		candidates = candidates[:query.Limit]
	}
	triggers, err := hydrateAutomationTriggerCatalogPage(ctx, executor, candidates)
	if err != nil {
		return automation.TriggerListPage{}, err
	}
	page := automation.TriggerListPage{
		Triggers: triggers,
		Total:    total,
		Limit:    query.Limit,
		HasMore:  hasMore,
	}
	if hasMore {
		last := candidates[len(candidates)-1]
		page.NextCursor, err = automation.EncodeTriggerListCursor(query, automation.ListCursorPosition{
			Source: last.Source,
			Name:   last.Name,
			ID:     last.ID,
		})
		if err != nil {
			return automation.TriggerListPage{}, err
		}
	}
	return page, nil
}

func automationTriggerCatalogWhere(query automation.TriggerListQuery) (string, []any) {
	clauses := make([]string, 0, 7)
	args := make([]any, 0, 18)
	if query.Scope != "" {
		clauses = append(clauses, "c.scope = ?")
		args = append(args, query.Scope)
	}
	if query.WorkspaceID != "" {
		clauses = append(clauses, "c.workspace_id = ?")
		args = append(args, query.WorkspaceID)
	}
	if query.Event != "" {
		clauses = append(clauses, "c.event = ?")
		args = append(args, query.Event)
	}
	if query.Source != "" {
		clauses = append(clauses, "c.source = ?")
		args = append(args, query.Source)
	}
	if query.LoopName != "" {
		clauses = append(clauses, "c.loop_name = ?")
		args = append(args, query.LoopName)
	}
	if query.Enabled != nil {
		clauses = append(clauses, automationTriggerEffectiveEnabledSQL+" = ?")
		args = append(args, *query.Enabled)
	}
	if query.Search != "" {
		clauses = append(clauses, `(
			instr(c.search_name, ?) > 0 OR
			instr(c.search_agent_name, ?) > 0 OR
			instr(c.search_prompt, ?) > 0 OR
			instr(c.search_scope, ?) > 0 OR
			instr(c.search_source, ?) > 0 OR
			instr(c.search_event, ?) > 0 OR
			instr(c.search_endpoint_slug, ?) > 0 OR
			instr(c.search_webhook_id, ?) > 0 OR
			EXISTS (
				SELECT 1 FROM automation_trigger_catalog_filter_terms AS filter_term
				WHERE filter_term.trigger_id = c.trigger_id AND instr(filter_term.value, ?) > 0
			)
		)`)
		for range 9 {
			args = append(args, query.Search)
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

const automationTriggerEffectiveEnabledSQL = `(CASE
	WHEN c.source IN ('config', 'package') AND o.enabled_override IS NOT NULL
		THEN o.enabled_override
	ELSE c.enabled
END)`

func readAutomationTriggerCatalogCandidates(
	ctx context.Context,
	executor automationCatalogExecutor,
	where string,
	args []any,
) (candidates []automationCatalogCandidate, err error) {
	// dynamic-sql: optional filters and keyset cursor terms change the candidate query shape.
	rows, err := executor.QueryContext(
		ctx,
		`SELECT c.trigger_id, c.source, c.name`+automationTriggerCatalogBaseSQL+where+
			` ORDER BY c.source_rank, c.name, c.trigger_id LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query automation trigger catalog candidates: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close automation trigger catalog candidate rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		var candidate automationCatalogCandidate
		if scanErr := rows.Scan(&candidate.ID, &candidate.Source, &candidate.Name); scanErr != nil {
			return nil, fmt.Errorf("store: scan automation trigger catalog candidate: %w", scanErr)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate automation trigger catalog candidates: %w", err)
	}
	return candidates, nil
}

func hydrateAutomationTriggerCatalogPage(
	ctx context.Context,
	executor automationCatalogExecutor,
	candidates []automationCatalogCandidate,
) (triggers []automation.Trigger, err error) {
	if len(candidates) == 0 {
		return []automation.Trigger{}, nil
	}
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("store: encode automation trigger catalog ids: %w", err)
	}
	rows, err := sqlcgen.New(executor).HydrateAutomationTriggerCatalog(ctx, string(idsJSON))
	if err != nil {
		return nil, fmt.Errorf("store: hydrate automation trigger catalog page: %w", err)
	}
	for _, row := range rows {
		trigger, scanErr := automationTriggerFromHydrated(row)
		if scanErr != nil {
			return nil, scanErr
		}
		triggers = append(triggers, trigger)
	}
	if len(triggers) != len(candidates) {
		return nil, fmt.Errorf(
			"store: automation trigger catalog hydrated %d of %d selected ids",
			len(triggers),
			len(candidates),
		)
	}
	return triggers, nil
}
