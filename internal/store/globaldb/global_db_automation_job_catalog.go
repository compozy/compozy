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

const automationJobCatalogWorkspaceIndex = "idx_automation_job_catalog_workspace_order"

const automationJobCatalogBaseSQL = ` FROM automation_job_catalog_entries AS c
	LEFT JOIN automation_job_overlays AS o ON o.job_id = c.job_id`

func listAutomationJobCatalog(
	ctx context.Context,
	executor automationCatalogExecutor,
	query automation.JobListQuery,
) (automation.JobListPage, error) {
	cursor, hasCursor, err := automation.JobListCursor(query)
	if err != nil {
		return automation.JobListPage{}, err
	}
	where, args := automationJobCatalogWhere(query)
	var total int
	// dynamic-sql: optional catalog filters change the count predicate set.
	if err := executor.QueryRowContext(
		ctx,
		`SELECT COUNT(*)`+automationJobCatalogBaseSQL+where,
		args...,
	).Scan(&total); err != nil {
		return automation.JobListPage{}, fmt.Errorf("store: count automation job catalog: %w", err)
	}
	candidateWhere, candidateArgs := where, append([]any(nil), args...)
	if hasCursor {
		candidateWhere = appendAutomationCatalogPredicate(
			candidateWhere,
			`(c.source_rank, c.name, c.job_id) > (?, ?, ?)`,
		)
		candidateArgs = append(
			candidateArgs,
			automation.ListSourceRank(cursor.Source),
			cursor.Name,
			cursor.ID,
		)
	}
	candidateArgs = append(candidateArgs, query.Limit+1)
	candidates, err := readAutomationJobCatalogCandidates(
		ctx,
		executor,
		candidateWhere,
		candidateArgs,
	)
	if err != nil {
		return automation.JobListPage{}, err
	}
	hasMore := len(candidates) > query.Limit
	if hasMore {
		candidates = candidates[:query.Limit]
	}
	jobs, err := hydrateAutomationJobCatalogPage(ctx, executor, candidates)
	if err != nil {
		return automation.JobListPage{}, err
	}
	page := automation.JobListPage{
		Jobs:    jobs,
		Total:   total,
		Limit:   query.Limit,
		HasMore: hasMore,
	}
	if hasMore {
		last := candidates[len(candidates)-1]
		page.NextCursor, err = automation.EncodeJobListCursor(query, automation.ListCursorPosition{
			Source: last.Source,
			Name:   last.Name,
			ID:     last.ID,
		})
		if err != nil {
			return automation.JobListPage{}, err
		}
	}
	return page, nil
}

func automationJobCatalogWhere(query automation.JobListQuery) (string, []any) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 16)
	if query.Scope != "" {
		clauses = append(clauses, "c.scope = ?")
		args = append(args, query.Scope)
	}
	if query.WorkspaceID != "" {
		clauses = append(clauses, "c.workspace_id = ?")
		args = append(args, query.WorkspaceID)
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
		clauses = append(clauses, automationJobEffectiveEnabledSQL+" = ?")
		args = append(args, *query.Enabled)
	}
	if query.Search != "" {
		clauses = append(clauses, `(
			instr(c.search_name, ?) > 0 OR
			instr(c.search_agent_name, ?) > 0 OR
			instr(c.search_prompt, ?) > 0 OR
			instr(c.search_scope, ?) > 0 OR
			instr(c.search_source, ?) > 0 OR
			instr(c.search_schedule_mode, ?) > 0 OR
			instr(c.search_schedule_expr, ?) > 0 OR
			instr(c.search_schedule_interval, ?) > 0 OR
			instr(c.search_schedule_time, ?) > 0
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

const automationJobEffectiveEnabledSQL = `(CASE
	WHEN c.source IN ('config', 'package') AND o.enabled_override IS NOT NULL
		THEN o.enabled_override
	ELSE c.enabled
END)`

func readAutomationJobCatalogCandidates(
	ctx context.Context,
	executor automationCatalogExecutor,
	where string,
	args []any,
) (candidates []automationCatalogCandidate, err error) {
	// dynamic-sql: optional filters and keyset cursor terms change the candidate query shape.
	rows, err := executor.QueryContext(
		ctx,
		`SELECT c.job_id, c.source, c.name`+automationJobCatalogBaseSQL+where+
			` ORDER BY c.source_rank, c.name, c.job_id LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query automation job catalog candidates: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close automation job catalog candidate rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		var candidate automationCatalogCandidate
		if scanErr := rows.Scan(&candidate.ID, &candidate.Source, &candidate.Name); scanErr != nil {
			return nil, fmt.Errorf("store: scan automation job catalog candidate: %w", scanErr)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate automation job catalog candidates: %w", err)
	}
	return candidates, nil
}

func hydrateAutomationJobCatalogPage(
	ctx context.Context,
	executor automationCatalogExecutor,
	candidates []automationCatalogCandidate,
) (jobs []automation.Job, err error) {
	if len(candidates) == 0 {
		return []automation.Job{}, nil
	}
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("store: encode automation job catalog ids: %w", err)
	}
	rows, err := sqlcgen.New(executor).HydrateAutomationJobCatalog(ctx, string(idsJSON))
	if err != nil {
		return nil, fmt.Errorf("store: hydrate automation job catalog page: %w", err)
	}
	for _, row := range rows {
		job, scanErr := automationJobFromHydrated(row)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	if len(jobs) != len(candidates) {
		return nil, fmt.Errorf(
			"store: automation job catalog hydrated %d of %d selected ids",
			len(jobs),
			len(candidates),
		)
	}
	return jobs, nil
}
