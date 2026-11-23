package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *GatewayRepo) GetIngressBinding(
	ctx context.Context,
	ref ingress.SubjectRef,
) (ingress.Binding, error) {
	if err := g.checkReady(ctx, "load gateway ingress binding"); err != nil {
		return ingress.Binding{}, err
	}
	row, err := g.readQueries.GetGatewayIngressBinding(ctx, ingressBindingKey(ref))
	if errors.Is(err, sql.ErrNoRows) {
		return ingress.Binding{}, ingress.ErrSubjectNotFound
	}
	if err != nil {
		return ingress.Binding{}, fmt.Errorf("store: load gateway ingress binding: %w", err)
	}
	return gatewayIngressBindingFromGenerated(row)
}

func (g *GatewayRepo) PutIngressBinding(
	ctx context.Context,
	expected ingress.Subject,
	endpointGeneration uint64,
	confirmedAt time.Time,
) (_ ingress.Binding, changed bool, err error) {
	if err := g.checkReady(ctx, "put gateway ingress binding"); err != nil {
		return ingress.Binding{}, false, err
	}
	generation, err := validateIngressBindingInput(endpointGeneration, confirmedAt)
	if err != nil {
		return ingress.Binding{}, false, err
	}
	var binding ingress.Binding
	if err := store.ExecuteWrite(ctx, g.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		actual, writeErr := resolveExpectedIngressSubject(writeCtx, queries, expected)
		if writeErr != nil {
			return writeErr
		}
		key := ingressBindingKey(actual.SubjectRef)
		current, currentErr := queries.GetGatewayIngressBinding(writeCtx, key)
		if currentErr == nil && current.ScopeKind == string(actual.Scope) &&
			current.WorkspaceID.String == actual.WorkspaceID && current.EndpointGeneration == generation {
			binding, writeErr = gatewayIngressBindingFromGenerated(current)
			changed = false
			return writeErr
		}
		if currentErr != nil && !errors.Is(currentErr, sql.ErrNoRows) {
			return fmt.Errorf("store: read prior ingress binding: %w", currentErr)
		}
		row, writeErr := queries.UpsertGatewayIngressBinding(
			writeCtx,
			sqlcgen.UpsertGatewayIngressBindingParams{
				SubjectKind: string(actual.Kind), SubjectID: actual.ID, ScopeKind: string(actual.Scope),
				WorkspaceID: store.SQLNullString(actual.WorkspaceID), EndpointGeneration: generation,
				ConfirmedAt: store.FormatTimestamp(confirmedAt),
			},
		)
		if writeErr != nil {
			return fmt.Errorf("store: upsert gateway ingress binding: %w", writeErr)
		}
		binding, writeErr = gatewayIngressBindingFromGenerated(row)
		changed = writeErr == nil
		return writeErr
	}); err != nil {
		return ingress.Binding{}, false, fmt.Errorf("store: put gateway ingress binding: %w", err)
	}
	return binding, changed, nil
}

func (g *GatewayRepo) DeleteIngressBinding(
	ctx context.Context,
	expected ingress.Subject,
) (bool, error) {
	if err := g.checkReady(ctx, "delete gateway ingress binding"); err != nil {
		return false, err
	}
	var rows int64
	if err := store.ExecuteWrite(ctx, g.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		actual, writeErr := resolveExpectedIngressSubject(writeCtx, queries, expected)
		if writeErr != nil {
			return writeErr
		}
		rows, writeErr = queries.DeleteGatewayIngressBinding(
			writeCtx,
			sqlcgen.DeleteGatewayIngressBindingParams{
				SubjectKind: string(actual.Kind), SubjectID: actual.ID,
			},
		)
		if writeErr != nil {
			return fmt.Errorf("store: delete gateway ingress binding: %w", writeErr)
		}
		return nil
	}); err != nil {
		return false, fmt.Errorf("store: remove gateway ingress binding: %w", err)
	}
	return rows > 0, nil
}

func validateIngressBindingInput(endpointGeneration uint64, confirmedAt time.Time) (int64, error) {
	generation, err := gatewayGeneration(endpointGeneration)
	if err != nil {
		return 0, fmt.Errorf("store: invalid ingress endpoint generation: %w", err)
	}
	if generation <= 0 {
		return 0, errors.New("store: invalid ingress endpoint generation")
	}
	if confirmedAt.IsZero() {
		return 0, errors.New("store: ingress confirmation timestamp is required")
	}
	return generation, nil
}

func resolveExpectedIngressSubject(
	ctx context.Context,
	queries *sqlcgen.Queries,
	expected ingress.Subject,
) (ingress.Subject, error) {
	actual, err := resolveGatewayIngressSubject(ctx, queries, expected.SubjectRef)
	if err != nil {
		return ingress.Subject{}, err
	}
	if actual.Scope != expected.Scope || actual.WorkspaceID != expected.WorkspaceID ||
		actual.ProfileID != expected.ProfileID {
		return ingress.Subject{}, ingress.ErrForbidden
	}
	return actual, nil
}

func (g *GatewayRepo) ListIngressBindings(ctx context.Context) ([]ingress.Binding, error) {
	if err := g.checkReady(ctx, "list gateway ingress bindings"); err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ListGatewayIngressBindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list gateway ingress bindings: %w", err)
	}
	bindings := make([]ingress.Binding, 0, len(rows))
	for _, row := range rows {
		binding, mapErr := gatewayIngressBindingFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func (g *GatewayRepo) SweepOrphanedIngressBindings(ctx context.Context) (int64, error) {
	if err := g.checkReady(ctx, "sweep orphaned gateway ingress bindings"); err != nil {
		return 0, err
	}
	rows, err := executeWriteQueryResult(
		ctx,
		g.repoBase,
		"globaldb.SweepOrphanedGatewayIngressBindings",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.SweepOrphanedGatewayIngressBindings(ctx)
		},
	)
	if err != nil {
		return 0, fmt.Errorf("store: sweep orphaned gateway ingress bindings: %w", err)
	}
	return rows, nil
}

func gatewayIngressBindingFromGenerated(
	row sqlcgen.GatewayIngressBinding,
) (ingress.Binding, error) {
	generation, err := positiveGatewayUnsignedValue("ingress binding generation", row.EndpointGeneration)
	if err != nil {
		return ingress.Binding{}, err
	}
	confirmedAt, err := store.ParseTimestamp(row.ConfirmedAt)
	if err != nil {
		return ingress.Binding{}, fmt.Errorf(
			"store: parse ingress binding confirmation: %w",
			err,
		)
	}
	return ingress.Binding{
		Subject: ingress.SubjectRef{
			Kind: ingress.SubjectKind(row.SubjectKind), ID: strings.TrimSpace(row.SubjectID),
		},
		Scope: ingress.ScopeKind(
			row.ScopeKind,
		), WorkspaceID: strings.TrimSpace(row.WorkspaceID.String),
		EndpointGeneration: generation, ConfirmedAt: confirmedAt,
	}, nil
}

func ingressBindingKey(ref ingress.SubjectRef) sqlcgen.GetGatewayIngressBindingParams {
	return sqlcgen.GetGatewayIngressBindingParams{
		SubjectKind: string(ref.Kind), SubjectID: strings.TrimSpace(ref.ID),
	}
}
