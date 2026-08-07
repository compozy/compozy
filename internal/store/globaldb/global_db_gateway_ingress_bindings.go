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
	generation, err := gatewayGeneration(endpointGeneration)
	if err != nil || generation <= 0 {
		return gateway.IngressBinding{}, false, errors.Join(errors.New("store: invalid ingress endpoint generation"), err)
	}
	if confirmedAt.IsZero() {
		return gateway.IngressBinding{}, false, errors.New("store: ingress confirmation timestamp is required")
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return gateway.IngressBinding{}, false, fmt.Errorf("store: begin ingress binding transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "gateway ingress binding"))
		}
	}()
	queries := sqlcgen.New(tx)
	actual, err := resolveGatewayIngressSubject(ctx, queries, expected.IngressSubjectRef)
	if err != nil {
		return gateway.IngressBinding{}, false, err
	}
	if actual.Scope != expected.Scope || actual.WorkspaceID != expected.WorkspaceID {
		return gateway.IngressBinding{}, false, gateway.ErrIngressForbidden
	}
	key := ingressBindingKey(actual.IngressSubjectRef)
	current, currentErr := queries.GetGatewayIngressBinding(ctx, key)
	if currentErr == nil && current.ScopeKind == string(actual.Scope) &&
		current.WorkspaceID.String == actual.WorkspaceID && current.EndpointGeneration == generation {
		binding, mapErr := gatewayIngressBindingFromGenerated(current)
		if mapErr != nil {
			return gateway.IngressBinding{}, false, mapErr
		}
		if err := tx.Commit(); err != nil {
			return gateway.IngressBinding{}, false, fmt.Errorf("store: commit unchanged ingress binding: %w", err)
		}
		committed = true
		return binding, false, nil
	}
	if currentErr != nil && !errors.Is(currentErr, sql.ErrNoRows) {
		return gateway.IngressBinding{}, false, fmt.Errorf("store: read prior ingress binding: %w", currentErr)
	}
	row, err := queries.UpsertGatewayIngressBinding(ctx, sqlcgen.UpsertGatewayIngressBindingParams{
		SubjectKind: string(actual.Kind), SubjectID: actual.ID, ScopeKind: string(actual.Scope),
		WorkspaceID: nullableString(actual.WorkspaceID), EndpointGeneration: generation,
		ConfirmedAt: store.FormatTimestamp(confirmedAt),
	})
	if err != nil {
		return gateway.IngressBinding{}, false, fmt.Errorf("store: upsert gateway ingress binding: %w", err)
	}
	binding, err := gatewayIngressBindingFromGenerated(row)
	if err != nil {
		return gateway.IngressBinding{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return gateway.IngressBinding{}, false, fmt.Errorf("store: commit ingress binding: %w", err)
	}
	committed = true
	return binding, true, nil
}

func (g *GatewayRepo) DeleteIngressBinding(
	ctx context.Context,
	expected gateway.IngressSubject,
) (_ bool, err error) {
	if err := g.checkReady(ctx, "delete gateway ingress binding"); err != nil {
		return false, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: begin ingress unbind transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "gateway ingress unbind"))
		}
	}()
	queries := sqlcgen.New(tx)
	actual, err := resolveGatewayIngressSubject(ctx, queries, expected.IngressSubjectRef)
	if err != nil {
		return false, err
	}
	if actual.Scope != expected.Scope || actual.WorkspaceID != expected.WorkspaceID {
		return false, gateway.ErrIngressForbidden
	}
	rows, err := queries.DeleteGatewayIngressBinding(ctx, sqlcgen.DeleteGatewayIngressBindingParams{
		SubjectKind: string(actual.Kind), SubjectID: strings.TrimSpace(actual.ID),
	})
	if err != nil {
		return false, fmt.Errorf("store: delete gateway ingress binding: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit ingress unbind: %w", err)
	}
	committed = true
	return rows > 0, nil
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

func gatewayIngressBindingFromGenerated(row sqlcgen.GatewayIngressBinding) (gateway.IngressBinding, error) {
	if row.EndpointGeneration <= 0 {
		return gateway.IngressBinding{}, errors.New("store: invalid ingress binding generation")
	}
	confirmedAt, err := store.ParseTimestamp(row.ConfirmedAt)
	if err != nil {
		return gateway.IngressBinding{}, fmt.Errorf("store: parse ingress binding confirmation: %w", err)
	}
	return gateway.IngressBinding{
		Subject: gateway.IngressSubjectRef{
			Kind: gateway.IngressSubjectKind(row.SubjectKind), ID: strings.TrimSpace(row.SubjectID),
		},
		Scope: gateway.IngressScopeKind(row.ScopeKind), WorkspaceID: strings.TrimSpace(row.WorkspaceID.String),
		EndpointGeneration: uint64(row.EndpointGeneration), ConfirmedAt: confirmedAt,
	}, nil
}

func ingressBindingKey(ref gateway.IngressSubjectRef) sqlcgen.GetGatewayIngressBindingParams {
	return sqlcgen.GetGatewayIngressBindingParams{
		SubjectKind: string(ref.Kind), SubjectID: strings.TrimSpace(ref.ID),
	}
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}
