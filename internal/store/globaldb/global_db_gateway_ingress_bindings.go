package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *GatewayRepo) GetIngressBinding(
	ctx context.Context,
	ref gateway.IngressSubjectRef,
) (gateway.IngressBinding, error) {
	if err := g.checkReady(ctx, "load gateway ingress binding"); err != nil {
		return gateway.IngressBinding{}, err
	}
	row, err := g.queries.GetGatewayIngressBinding(ctx, ingressBindingKey(ref))
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.IngressBinding{}, gateway.ErrIngressSubjectNotFound
	}
	if err != nil {
		return gateway.IngressBinding{}, fmt.Errorf("store: load gateway ingress binding: %w", err)
	}
	return gatewayIngressBindingFromGenerated(row)
}

func (g *GatewayRepo) PutIngressBinding(
	ctx context.Context,
	expected gateway.IngressSubject,
	endpointGeneration uint64,
	confirmedAt time.Time,
) (_ gateway.IngressBinding, changed bool, err error) {
	if err := g.checkReady(ctx, "put gateway ingress binding"); err != nil {
		return gateway.IngressBinding{}, false, err
	}
	generation, err := validateIngressBindingInput(endpointGeneration, confirmedAt)
	if err != nil {
		return gateway.IngressBinding{}, false, err
	}
	var binding gateway.IngressBinding
	if err := store.ExecuteWrite(ctx, g.db, func(writeCtx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		actual, writeErr := resolveExpectedIngressSubject(writeCtx, queries, expected)
		if writeErr != nil {
			return writeErr
		}
		key := ingressBindingKey(actual.IngressSubjectRef)
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
		return gateway.IngressBinding{}, false, fmt.Errorf("store: put gateway ingress binding: %w", err)
	}
	return binding, changed, nil
}

func (g *GatewayRepo) DeleteIngressBinding(
	ctx context.Context,
	expected gateway.IngressSubject,
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
	expected gateway.IngressSubject,
) (gateway.IngressSubject, error) {
	actual, err := resolveGatewayIngressSubject(ctx, queries, expected.IngressSubjectRef)
	if err != nil {
		return gateway.IngressSubject{}, err
	}
	if actual.Scope != expected.Scope || actual.WorkspaceID != expected.WorkspaceID {
		return gateway.IngressSubject{}, gateway.ErrIngressForbidden
	}
	return actual, nil
}

func (g *GatewayRepo) ListIngressBindings(ctx context.Context) ([]gateway.IngressBinding, error) {
	if err := g.checkReady(ctx, "list gateway ingress bindings"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListGatewayIngressBindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list gateway ingress bindings: %w", err)
	}
	bindings := make([]gateway.IngressBinding, 0, len(rows))
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
	rows, err := g.queries.SweepOrphanedGatewayIngressBindings(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: sweep orphaned gateway ingress bindings: %w", err)
	}
	return rows, nil
}

func gatewayIngressBindingFromGenerated(
	row sqlcgen.GatewayIngressBinding,
) (gateway.IngressBinding, error) {
	generation, err := positiveGatewayUnsignedValue("ingress binding generation", row.EndpointGeneration)
	if err != nil {
		return gateway.IngressBinding{}, err
	}
	confirmedAt, err := store.ParseTimestamp(row.ConfirmedAt)
	if err != nil {
		return gateway.IngressBinding{}, fmt.Errorf(
			"store: parse ingress binding confirmation: %w",
			err,
		)
	}
	return gateway.IngressBinding{
		Subject: gateway.IngressSubjectRef{
			Kind: gateway.IngressSubjectKind(row.SubjectKind), ID: strings.TrimSpace(row.SubjectID),
		},
		Scope: gateway.IngressScopeKind(
			row.ScopeKind,
		), WorkspaceID: strings.TrimSpace(row.WorkspaceID.String),
		EndpointGeneration: generation, ConfirmedAt: confirmedAt,
	}, nil
}

func ingressBindingKey(ref gateway.IngressSubjectRef) sqlcgen.GetGatewayIngressBindingParams {
	return sqlcgen.GetGatewayIngressBindingParams{
		SubjectKind: string(ref.Kind), SubjectID: strings.TrimSpace(ref.ID),
	}
}
