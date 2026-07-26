package globaldb

import (
	"context"
	"errors"
	"fmt"

	automation "github.com/compozy/agh/internal/automation/model"
)

const automationDefinitionOrderSQL = ` ORDER BY CASE source
	WHEN 'config' THEN 0
	WHEN 'package' THEN 1
	WHEN 'dynamic' THEN 2
	ELSE 3
END, name, id`

// ListJobDefinitions returns complete persisted definitions for internal reconciliation.
func (g *AutomationRepo) ListJobDefinitions(
	ctx context.Context,
	source automation.JobSource,
) (jobs []automation.Job, err error) {
	if err := g.checkReady(ctx, "list automation job definitions"); err != nil {
		return nil, err
	}
	if source != "" {
		if err := source.Validate("job_definition_query.source"); err != nil {
			return nil, err
		}
	}
	// dynamic-sql: optional definition filters and caller limit change the statement shape.
	statement := automationJobRichSelectSQL
	args := make([]any, 0, 1)
	if source != "" {
		statement += ` WHERE source = ?`
		args = append(args, source)
	}
	statement += automationDefinitionOrderSQL
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query automation job definitions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close automation job definition rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		job, scanErr := scanAutomationJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate automation job definitions: %w", err)
	}
	return jobs, nil
}

// ListTriggerDefinitions returns complete persisted definitions for internal reconciliation.
func (g *AutomationRepo) ListTriggerDefinitions(
	ctx context.Context,
	source automation.JobSource,
) (triggers []automation.Trigger, err error) {
	if err := g.checkReady(ctx, "list automation trigger definitions"); err != nil {
		return nil, err
	}
	if source != "" {
		if err := source.Validate("trigger_definition_query.source"); err != nil {
			return nil, err
		}
	}
	// dynamic-sql: optional definition filters and caller limit change the statement shape.
	statement := automationTriggerRichSelectSQL
	args := make([]any, 0, 1)
	if source != "" {
		statement += ` WHERE source = ?`
		args = append(args, source)
	}
	statement += automationDefinitionOrderSQL
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query automation trigger definitions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close automation trigger definition rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		trigger, scanErr := scanAutomationTrigger(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		triggers = append(triggers, trigger)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate automation trigger definitions: %w", err)
	}
	return triggers, nil
}
