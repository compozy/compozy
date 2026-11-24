package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const (
	legacyIngressSource = "gateway"
	remoteIngressSource = "remote"
)

func (g *GatewayRepo) SyncRemoteIngressEndpoint(ctx context.Context, next ingress.RemoteEndpoint, now time.Time) error {
	if err := g.checkReady(ctx, "sync remote ingress endpoint"); err != nil {
		return err
	}
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		q := sqlcgen.New(tx)
		current, err := q.GetIngressEndpointState(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !next.Managed {
			if current.Source != remoteIngressSource {
				return nil
			}
			legacy, err := q.GetGatewayEndpointState(ctx, ingressEndpointTier)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			return putIngressEndpoint(ctx, q, current, legacy.EndpointUrl, legacyIngressSource, legacy.Reachable, now)
		}
		if next.Pending && current.Source == remoteIngressSource && current.EndpointUrl != "" {
			next.Base = current.EndpointUrl
		}
		return putIngressEndpoint(ctx, q, current, next.Base, remoteIngressSource, next.Reachable, now)
	})
}

func syncLegacyIngressEndpoint(ctx context.Context, q *sqlcgen.Queries, base string, ready bool, now time.Time) error {
	current, err := q.GetIngressEndpointState(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if current.Source == remoteIngressSource {
		return nil
	}
	if !ready {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		base = current.EndpointUrl
	}
	return putIngressEndpoint(ctx, q, current, base, legacyIngressSource, ready, now)
}

func putIngressEndpoint(
	ctx context.Context, q *sqlcgen.Queries, current sqlcgen.IngressEndpointState,
	base, source string, reachable bool, now time.Time,
) error {
	if current.EndpointUrl == base && current.Source == source && current.Reachable == reachable {
		return nil
	}
	_, err := q.UpsertIngressEndpointState(ctx, sqlcgen.UpsertIngressEndpointStateParams{
		EndpointUrl: base, Source: source, Reachable: reachable, UpdatedAt: store.FormatTimestamp(now),
	})
	return err
}

func ingressEndpointFromGenerated(row sqlcgen.IngressEndpointState) (ingress.EndpointState, error) {
	generation, err := positiveGatewayUnsignedValue("endpoint generation", row.EndpointGeneration)
	if err != nil {
		return ingress.EndpointState{}, err
	}
	at, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return ingress.EndpointState{}, err
	}
	return ingress.EndpointState{
		URL: row.EndpointUrl, Generation: generation, Reachable: row.Reachable,
		Remote: row.Source == remoteIngressSource, UpdatedAt: at,
	}, nil
}
