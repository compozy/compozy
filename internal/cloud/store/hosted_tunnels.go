package store

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

type TunnelRef struct {
	ComputerID remoteproto.ComputerID
	Generation int64
}

type HostedTunnel struct {
	TunnelRef
	Hostname     string
	State        string
	CloudflareID string
	Origin       string
	DeleteAfter  time.Time
}

func hostedTunnel(row sqlcgen.Tunnel, err error) (HostedTunnel, error) {
	return HostedTunnel{
		ComputerID: remoteproto.ComputerID(row.ComputerID), Generation: row.Generation,
		Hostname: row.Hostname, State: row.State, CloudflareID: row.CfTunnelID.String,
		Origin: row.HostedOrigin.String, DeleteAfter: row.DeleteAfter.Time,
	}, err
}

func (q *Queries) LockHostedTunnel(ctx context.Context, id remoteproto.ComputerID) (HostedTunnel, error) {
	return hostedTunnel(q.generated.LockHostedTunnel(ctx, string(id)))
}

func (q *Queries) InsertHostedTunnel(
	ctx context.Context, id remoteproto.ComputerID, hostname string, now time.Time,
) (HostedTunnel, error) {
	return hostedTunnel(q.generated.InsertHostedTunnel(ctx, sqlcgen.InsertHostedTunnelParams{
		ComputerID: string(id), Hostname: hostname, CreatedAt: timestamp(now),
	}))
}

func (q *Queries) ReserveHostedTunnel(ctx context.Context, ref TunnelRef, now time.Time) (HostedTunnel, error) {
	return hostedTunnel(q.generated.ReserveHostedTunnel(ctx, sqlcgen.ReserveHostedTunnelParams{
		ComputerID: string(ref.ComputerID), Generation: ref.Generation, CreatedAt: timestamp(now),
	}))
}

func (q *Queries) UpdateHostedTunnel(ctx context.Context, tunnel HostedTunnel) (HostedTunnel, error) {
	return hostedTunnel(q.generated.UpdateHostedTunnel(ctx, sqlcgen.UpdateHostedTunnelParams{
		ComputerID: string(tunnel.ComputerID), Generation: tunnel.Generation,
		State: tunnel.State, CfTunnelID: dbText(tunnel.CloudflareID), HostedOrigin: dbText(tunnel.Origin),
	}))
}

func (q *Queries) ActiveHostedTunnels(
	ctx context.Context, accountID string, now time.Time,
) (map[remoteproto.ComputerID]string, error) {
	rows, err := q.generated.ActiveHostedTunnels(ctx, sqlcgen.ActiveHostedTunnelsParams{
		AccountID: accountID, AsOf: timestamp(now),
	})
	if err != nil {
		return nil, err
	}
	result := make(map[remoteproto.ComputerID]string, len(rows))
	for _, row := range rows {
		result[remoteproto.ComputerID(row.ComputerID)] = row.Hostname
	}
	return result, nil
}

func (q *Queries) SweepableHostedTunnels(
	ctx context.Context, now, idleSince, unfinishedSince time.Time,
) ([]TunnelRef, error) {
	rows, err := q.generated.SweepableHostedTunnels(ctx, sqlcgen.SweepableHostedTunnelsParams{
		AsOf: timestamp(now), IdleSince: timestamp(idleSince), UnfinishedSince: timestamp(unfinishedSince),
	})
	if err != nil {
		return nil, err
	}
	refs := make([]TunnelRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, TunnelRef{ComputerID: remoteproto.ComputerID(row.ComputerID), Generation: row.Generation})
	}
	return refs, nil
}

func (q *Queries) HostedTunnelCanSweep(
	ctx context.Context, id remoteproto.ComputerID, now, idleSince, unfinishedSince time.Time,
) (bool, error) {
	return q.generated.HostedTunnelCanSweep(ctx, sqlcgen.HostedTunnelCanSweepParams{
		ComputerID: string(id), AsOf: timestamp(now), IdleSince: timestamp(idleSince),
		UnfinishedSince: timestamp(unfinishedSince),
	})
}

func (q *Queries) CountAllocatedHostedTunnels(ctx context.Context) (int64, error) {
	return q.generated.CountAllocatedHostedTunnels(ctx)
}
