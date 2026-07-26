package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	automation "github.com/compozy/agh/internal/automation/model"
)

type automationCatalogExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// ListJobs returns one stable page of effective persisted automation jobs.
func (g *AutomationRepo) ListJobs(
	ctx context.Context,
	query automation.JobListQuery,
) (page automation.JobListPage, err error) {
	if err := g.checkReady(ctx, "list automation jobs"); err != nil {
		return automation.JobListPage{}, err
	}
	normalized := automation.JobListQueryWithDefaultLimit(query)
	if err := automation.ValidateJobListQuery(normalized); err != nil {
		return automation.JobListPage{}, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return automation.JobListPage{}, fmt.Errorf("store: begin automation job catalog read: %w", err)
	}
	defer rollbackAutomationCatalogTx(&err, tx, "automation job catalog read")
	page, err = listAutomationJobCatalog(ctx, tx, normalized)
	if err != nil {
		return automation.JobListPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return automation.JobListPage{}, fmt.Errorf("store: commit automation job catalog read: %w", err)
	}
	return page, nil
}

// ListTriggers returns one stable page of effective persisted automation triggers.
func (g *AutomationRepo) ListTriggers(
	ctx context.Context,
	query automation.TriggerListQuery,
) (page automation.TriggerListPage, err error) {
	if err := g.checkReady(ctx, "list automation triggers"); err != nil {
		return automation.TriggerListPage{}, err
	}
	normalized := automation.TriggerListQueryWithDefaultLimit(query)
	if err := automation.ValidateTriggerListQuery(normalized); err != nil {
		return automation.TriggerListPage{}, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return automation.TriggerListPage{}, fmt.Errorf("store: begin automation trigger catalog read: %w", err)
	}
	defer rollbackAutomationCatalogTx(&err, tx, "automation trigger catalog read")
	page, err = listAutomationTriggerCatalog(ctx, tx, normalized)
	if err != nil {
		return automation.TriggerListPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return automation.TriggerListPage{}, fmt.Errorf("store: commit automation trigger catalog read: %w", err)
	}
	return page, nil
}

func rollbackAutomationCatalogTx(target *error, tx *sql.Tx, action string) {
	if target == nil || tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		*target = errors.Join(*target, fmt.Errorf("store: rollback %s: %w", action, err))
	}
}
