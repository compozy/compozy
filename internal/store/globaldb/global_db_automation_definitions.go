package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// CreateJob stores a new automation job definition.
func (g *AutomationRepo) CreateJob(
	ctx context.Context,
	job automation.Job,
) (created automation.Job, err error) {
	if err := g.checkReady(ctx, "create automation job"); err != nil {
		return automation.Job{}, err
	}
	normalized, err := g.normalizeJobForCreate(job)
	if err != nil {
		return automation.Job{}, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return automation.Job{}, fmt.Errorf(
			"store: begin create automation job %q: %w",
			normalized.ID,
			err,
		)
	}
	defer rollbackAutomationDefinitionTx(&err, tx, "create automation job")
	if err := g.insertJob(ctx, tx, normalized); err != nil {
		return automation.Job{}, fmt.Errorf(
			"store: create automation job %q: %w",
			normalized.ID,
			err,
		)
	}
	if err := upsertAutomationJobCatalog(ctx, tx, normalized); err != nil {
		return automation.Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return automation.Job{}, fmt.Errorf(
			"store: commit create automation job %q: %w",
			normalized.ID,
			err,
		)
	}
	return normalized, nil
}

// UpdateJob replaces the mutable fields of a persisted automation job definition.
func (g *AutomationRepo) UpdateJob(
	ctx context.Context,
	job automation.Job,
) (updated automation.Job, err error) {
	if err := g.checkReady(ctx, "update automation job"); err != nil {
		return automation.Job{}, err
	}
	normalized, err := g.normalizeJobForUpdate(job)
	if err != nil {
		return automation.Job{}, err
	}
	params, err := automationJobUpdateParams(normalized)
	if err != nil {
		return automation.Job{}, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return automation.Job{}, fmt.Errorf(
			"store: begin update automation job %q: %w",
			normalized.ID,
			err,
		)
	}
	defer rollbackAutomationDefinitionTx(&err, tx, "update automation job")
	affected, err := sqlcgen.New(tx).UpdateAutomationJob(ctx, params)
	if err != nil {
		return automation.Job{}, fmt.Errorf(
			"store: update automation job %q: %w",
			normalized.ID,
			mapAutomationJobConstraintError(err),
		)
	}
	if err := requireAutomationAffected(
		affected,
		automation.ErrJobNotFound,
		normalized.ID,
		"automation job",
	); err != nil {
		return automation.Job{}, err
	}
	if err := upsertAutomationJobCatalog(ctx, tx, normalized); err != nil {
		return automation.Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return automation.Job{}, fmt.Errorf(
			"store: commit update automation job %q: %w",
			normalized.ID,
			err,
		)
	}
	return g.GetJob(ctx, normalized.ID)
}

// DeleteJob removes an automation job definition.
func (g *AutomationRepo) DeleteJob(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "delete automation job"); err != nil {
		return err
	}
	trimmedID, err := requireAutomationID(id, "automation job id")
	if err != nil {
		return err
	}
	affected, err := g.queries.DeleteAutomationJob(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf(
			"store: delete automation job %q: %w",
			trimmedID,
			mapAutomationJobConstraintError(err),
		)
	}
	return requireAutomationAffected(
		affected,
		automation.ErrJobNotFound,
		trimmedID,
		"automation job",
	)
}

// GetJob loads one persisted automation job definition by primary key.
func (g *AutomationRepo) GetJob(ctx context.Context, id string) (automation.Job, error) {
	if err := g.checkReady(ctx, "get automation job"); err != nil {
		return automation.Job{}, err
	}
	trimmedID, err := requireAutomationID(id, "automation job id")
	if err != nil {
		return automation.Job{}, err
	}
	row, err := g.queries.GetAutomationJob(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return automation.Job{}, automation.ErrJobNotFound
		}
		return automation.Job{}, err
	}
	return automationJobFromGenerated(row)
}

// CreateTrigger stores a new automation trigger definition.
func (g *AutomationRepo) CreateTrigger(
	ctx context.Context,
	trigger automation.Trigger,
) (created automation.Trigger, err error) {
	if err := g.checkReady(ctx, "create automation trigger"); err != nil {
		return automation.Trigger{}, err
	}
	normalized, err := g.normalizeTriggerForCreate(trigger)
	if err != nil {
		return automation.Trigger{}, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: begin create automation trigger %q: %w",
			normalized.ID,
			err,
		)
	}
	defer rollbackAutomationDefinitionTx(&err, tx, "create automation trigger")
	if err := g.insertTrigger(ctx, tx, normalized); err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: create automation trigger %q: %w",
			normalized.ID,
			err,
		)
	}
	if err := upsertAutomationTriggerCatalog(ctx, tx, normalized); err != nil {
		return automation.Trigger{}, err
	}
	if err := tx.Commit(); err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: commit create automation trigger %q: %w",
			normalized.ID,
			err,
		)
	}
	return normalized, nil
}

// UpdateTrigger replaces the mutable fields of a persisted automation trigger definition.
func (g *AutomationRepo) UpdateTrigger(
	ctx context.Context,
	trigger automation.Trigger,
) (updated automation.Trigger, err error) {
	if err := g.checkReady(ctx, "update automation trigger"); err != nil {
		return automation.Trigger{}, err
	}
	normalized, err := g.normalizeTriggerForUpdate(trigger)
	if err != nil {
		return automation.Trigger{}, err
	}
	params, err := automationTriggerUpdateParams(normalized)
	if err != nil {
		return automation.Trigger{}, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: begin update automation trigger %q: %w",
			normalized.ID,
			err,
		)
	}
	defer rollbackAutomationDefinitionTx(&err, tx, "update automation trigger")
	queries := sqlcgen.New(tx)
	previous, err := queries.GetAutomationTrigger(ctx, normalized.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return automation.Trigger{}, fmt.Errorf(
			"store: automation trigger %q: %w",
			normalized.ID,
			automation.ErrTriggerNotFound,
		)
	}
	if err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: load prior automation trigger %q: %w",
			normalized.ID,
			err,
		)
	}
	affected, err := queries.UpdateAutomationTrigger(ctx, params)
	if err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: update automation trigger %q: %w",
			normalized.ID,
			mapAutomationTriggerConstraintError(ctx, tx, normalized, err),
		)
	}
	if err := requireAutomationAffected(
		affected,
		automation.ErrTriggerNotFound,
		normalized.ID,
		"automation trigger",
	); err != nil {
		return automation.Trigger{}, err
	}
	if err := upsertAutomationTriggerCatalog(ctx, tx, normalized); err != nil {
		return automation.Trigger{}, err
	}
	identityChanged := automationNullStringValue(previous.WebhookID) != normalized.WebhookID ||
		automationNullStringValue(previous.EndpointSlug) != normalized.EndpointSlug
	if err := reconcileAutomationIngressBinding(ctx, queries, normalized, identityChanged); err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: invalidate changed webhook ingress binding %q: %w",
			normalized.ID,
			err,
		)
	}
	if err := tx.Commit(); err != nil {
		return automation.Trigger{}, fmt.Errorf(
			"store: commit update automation trigger %q: %w",
			normalized.ID,
			err,
		)
	}
	return g.GetTrigger(ctx, normalized.ID)
}

func reconcileAutomationIngressBinding(
	ctx context.Context,
	queries *sqlcgen.Queries,
	trigger automation.Trigger,
	identityChanged bool,
) error {
	if identityChanged {
		_, err := queries.DeleteGatewayIngressBinding(ctx, sqlcgen.DeleteGatewayIngressBindingParams{
			SubjectKind: string(gateway.IngressSubjectWebhookTrigger), SubjectID: trigger.ID,
		})
		return err
	}
	_, err := queries.DeleteMismatchedGatewayIngressBinding(
		ctx,
		sqlcgen.DeleteMismatchedGatewayIngressBindingParams{
			SubjectKind: string(gateway.IngressSubjectWebhookTrigger),
			SubjectID:   trigger.ID,
			ScopeKind:   string(trigger.Scope),
			WorkspaceID: nullableAutomationString(trigger.WorkspaceID),
		},
	)
	return err
}

// DeleteTrigger removes an automation trigger definition.
func (g *AutomationRepo) DeleteTrigger(ctx context.Context, id string) (err error) {
	if err := g.checkReady(ctx, "delete automation trigger"); err != nil {
		return err
	}
	trimmedID, err := requireAutomationID(id, "automation trigger id")
	if err != nil {
		return err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin automation trigger deletion: %w", err)
	}
	defer rollbackAutomationDefinitionTx(&err, tx, "delete automation trigger")
	queries := sqlcgen.New(tx)
	if _, err := queries.DeleteGatewayIngressBinding(ctx, sqlcgen.DeleteGatewayIngressBindingParams{
		SubjectKind: string(gateway.IngressSubjectWebhookTrigger), SubjectID: trimmedID,
	}); err != nil {
		return fmt.Errorf("store: delete webhook ingress binding %q: %w", trimmedID, err)
	}
	affected, err := queries.DeleteAutomationTrigger(ctx, trimmedID)
	if err != nil {
		return fmt.Errorf("store: delete automation trigger %q: %w", trimmedID, err)
	}
	if err := requireAutomationAffected(
		affected,
		automation.ErrTriggerNotFound,
		trimmedID,
		"automation trigger",
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit automation trigger deletion %q: %w", trimmedID, err)
	}
	return nil
}

// GetTrigger loads one persisted automation trigger definition by primary key.
func (g *AutomationRepo) GetTrigger(ctx context.Context, id string) (automation.Trigger, error) {
	if err := g.checkReady(ctx, "get automation trigger"); err != nil {
		return automation.Trigger{}, err
	}
	trimmedID, err := requireAutomationID(id, "automation trigger id")
	if err != nil {
		return automation.Trigger{}, err
	}
	row, err := g.queries.GetAutomationTrigger(ctx, trimmedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return automation.Trigger{}, automation.ErrTriggerNotFound
		}
		return automation.Trigger{}, err
	}
	return automationTriggerFromGenerated(row)
}

func rollbackAutomationDefinitionTx(target *error, tx *sql.Tx, action string) {
	if target == nil || tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		rollbackErr := fmt.Errorf("store: rollback %s: %w", action, err)
		*target = errors.Join(*target, rollbackErr)
	}
}
