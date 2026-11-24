package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *IngressRepo) GetIngressEndpoint(ctx context.Context) (ingress.EndpointState, error) {
	if err := g.checkReady(ctx, "load ingress endpoint"); err != nil {
		return ingress.EndpointState{}, err
	}
	row, err := g.readQueries.GetIngressEndpointState(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ingress.EndpointState{}, ingress.ErrEndpointUnverified
	}
	if err != nil {
		return ingress.EndpointState{}, fmt.Errorf("store: load ingress endpoint: %w", err)
	}
	generation, err := ingressGeneration(row.EndpointGeneration)
	if err != nil {
		return ingress.EndpointState{}, err
	}
	at, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return ingress.EndpointState{}, err
	}
	return ingress.EndpointState{
		URL: row.EndpointUrl, Generation: generation, Reachable: row.Reachable, UpdatedAt: at,
	}, nil
}

func (g *IngressRepo) SyncRemoteIngressEndpoint(ctx context.Context, next ingress.RemoteEndpoint, now time.Time) error {
	if err := g.checkReady(ctx, "sync ingress endpoint"); err != nil {
		return err
	}
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		current, err := q.GetIngressEndpointState(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store: read prior ingress endpoint: %w", err)
		}
		base := next.Base
		if !next.Managed || (next.Pending && current.EndpointUrl != "") {
			base = current.EndpointUrl
		}
		reachable := next.Managed && next.Reachable
		baseChanged := current.EndpointUrl != base
		if !baseChanged && current.Reachable == reachable {
			return nil
		}
		if err := q.UpsertIngressEndpointState(ctx, sqlcgen.UpsertIngressEndpointStateParams{
			EndpointUrl: base, Reachable: reachable, UpdatedAt: store.FormatTimestamp(now),
		}); err != nil {
			return fmt.Errorf("store: update ingress endpoint: %w", err)
		}
		if baseChanged {
			return q.RefreshGatewayRemovalBindingCount(ctx)
		}
		return nil
	})
}

func ingressGeneration(value int64) (uint64, error) {
	if value <= 0 {
		return 0, errors.New("store: invalid ingress endpoint generation")
	}
	return uint64(value), nil
}
