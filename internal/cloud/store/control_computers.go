package store

import (
	"context"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

type ControlComputer struct {
	ID            remoteproto.ComputerID
	AccountID     string
	Name          string
	Platform      string
	Version       string
	Key           remoteproto.JWK
	TLSRootSHA256 string
	RemoteEnabled bool
	PlanLimited   bool
	LastSessionAt time.Time
	LastControlAt time.Time
	LinkID        string
	LinkedAt      time.Time
	UnlinkedAt    time.Time
	Endpoints     []string
}

func controlComputer(row sqlcgen.Computer, err error) (ControlComputer, error) {
	if err != nil {
		return ControlComputer{}, err
	}
	var key remoteproto.JWK
	if err := json.Unmarshal(row.ComputerJwk, &key); err != nil {
		return ControlComputer{}, errors.New("invalid stored computer key")
	}
	return ControlComputer{
		ID: remoteproto.ComputerID(row.ID), AccountID: row.AccountID, Name: row.Name, Platform: row.Platform,
		Version: row.Version, Key: key, TLSRootSHA256: row.TlsRootSha256, RemoteEnabled: row.RemoteEnabled,
		PlanLimited: row.PlanLimited, LastSessionAt: row.LastSessionAt.Time, LastControlAt: row.LastControlAt.Time,
		LinkID: row.ControlLinkID.String, LinkedAt: row.LinkedAt.Time,
		UnlinkedAt: row.UnlinkedAt.Time, Endpoints: []string{},
	}, nil
}

func (q *Queries) ControlComputer(ctx context.Context, id remoteproto.ComputerID) (ControlComputer, error) {
	return controlComputer(q.generated.ControlComputer(ctx, string(id)))
}

func (q *Queries) ListControlComputers(ctx context.Context, account string) ([]ControlComputer, error) {
	rows, err := q.generated.ControlComputers(ctx, account)
	if err != nil {
		return nil, err
	}
	endpoints, err := q.generated.ControlEndpoints(ctx, account)
	if err != nil {
		return nil, err
	}
	byComputer := make(map[string][]string)
	for _, endpoint := range endpoints {
		byComputer[endpoint.ComputerID] = append(byComputer[endpoint.ComputerID], endpoint.Address)
	}
	computers := make([]ControlComputer, 0, len(rows))
	for _, row := range rows {
		computer, err := controlComputer(row, nil)
		if err != nil {
			return nil, err
		}
		computer.Endpoints = append(computer.Endpoints, byComputer[row.ID]...)
		computers = append(computers, computer)
	}
	return computers, nil
}

func (q *Queries) CreateControlComputer(ctx context.Context, computer ControlComputer) (ControlComputer, error) {
	jwk, err := json.Marshal(computer.Key)
	if err != nil {
		return ControlComputer{}, err
	}
	return controlComputer(q.generated.CreateControlComputer(ctx, sqlcgen.CreateControlComputerParams{
		ID: string(computer.ID), AccountID: computer.AccountID, Name: computer.Name, Platform: computer.Platform,
		Version: computer.Version, ComputerJwk: string(jwk), TlsRootSha256: computer.TLSRootSHA256,
		LinkedAt: timestamp(computer.LinkedAt),
	}))
}

func (q *Queries) UpdateControlComputer(ctx context.Context, computer ControlComputer) (ControlComputer, error) {
	return controlComputer(q.generated.UpdateControlComputer(ctx, sqlcgen.UpdateControlComputerParams{
		ID: string(
			computer.ID,
		),
		AccountID:     computer.AccountID,
		Name:          computer.Name,
		RemoteEnabled: computer.RemoteEnabled,
	}))
}

func (q *Queries) SetControlEndpoints(
	ctx context.Context, id remoteproto.ComputerID, endpoints []string, at time.Time,
) error {
	if err := q.generated.ClearControlEndpoints(ctx, string(id)); err != nil {
		return err
	}
	return q.generated.SetControlEndpoints(ctx, sqlcgen.SetControlEndpointsParams{
		ComputerID: string(id), Addresses: endpoints, ReportedAt: timestamp(at),
	})
}

func (q *Queries) UnlinkControlComputer(ctx context.Context, computer ControlComputer, at time.Time) error {
	return q.generated.UnlinkControlComputer(ctx, sqlcgen.UnlinkControlComputerParams{
		ID: string(computer.ID), AccountID: computer.AccountID, UnlinkedAt: timestamp(at),
	})
}

func (q *Queries) OpenComputerControlLink(
	ctx context.Context,
	computer ControlComputer,
	linkID string,
	at time.Time,
) error {
	return q.generated.OpenComputerControlLink(ctx, sqlcgen.OpenComputerControlLinkParams{
		ID: string(
			computer.ID,
		),
		AccountID:     computer.AccountID,
		ControlLinkID: dbText(linkID),
		LastControlAt: timestamp(at),
	})
}

func (q *Queries) CloseComputerControlLink(
	ctx context.Context,
	id remoteproto.ComputerID,
	linkID string,
) (bool, error) {
	count, err := q.generated.CloseComputerControlLink(ctx, sqlcgen.CloseComputerControlLinkParams{
		ID: string(id), ControlLinkID: dbText(linkID),
	})
	return count == 1, err
}

func (q *Queries) ApplyControlPlan(ctx context.Context, account string, tier plan.Plan) (bool, error) {
	if tier == plan.Pro {
		changed, err := q.generated.ApplyControlProPlan(ctx, account)
		return changed > 0, err
	}
	changed, err := q.generated.ApplyControlFreePlan(ctx, account)
	return changed > 0, err
}

func (q *Queries) AdvanceControlLease(
	ctx context.Context, computer ControlComputer, linkID string, at time.Time,
) (uint64, error) {
	sequence, err := q.generated.AdvanceControlLease(ctx, sqlcgen.AdvanceControlLeaseParams{
		ComputerID: string(computer.ID), AccountID: computer.AccountID,
		LinkID: linkID, SequenceFloor: at.UnixMicro(), ControlAt: timestamp(at),
	})
	if err != nil {
		return 0, err
	}
	if sequence <= 0 {
		return 0, errors.New("stored lease sequence must be positive")
	}
	return uint64(sequence), nil
}

func (q *Queries) RecordControlMint(
	ctx context.Context,
	computer ControlComputer,
	mint remoteproto.SessionMinted,
) error {
	return q.generated.RecordControlMint(ctx, sqlcgen.RecordControlMintParams{
		ComputerID: string(computer.ID), AccountID: computer.AccountID, DeviceID: string(mint.DeviceID),
		Jkt: mint.JKT, MintedAt: timestamp(time.Unix(mint.At, 0)),
	})
}

func (q *Queries) ListControlPeers(
	ctx context.Context, computer ControlComputer, now time.Time,
) ([]remoteproto.HostedPeer, error) {
	rows, err := q.generated.ListControlPeers(ctx, sqlcgen.ListControlPeersParams{
		AccountID: computer.AccountID, ID: string(computer.ID), AsOf: timestamp(now),
	})
	if err != nil {
		return nil, err
	}
	peers := make([]remoteproto.HostedPeer, 0, len(rows))
	for _, row := range rows {
		peers = append(peers, remoteproto.HostedPeer{
			ComputerID: remoteproto.ComputerID(row.ID), Name: row.Name, HostedURL: "https://" + row.Hostname,
		})
	}
	return peers, nil
}

type SlotReservation struct {
	ID         string                 `json:"reservation_id"`
	ExpiresAt  time.Time              `json:"expires_at"`
	ComputerID remoteproto.ComputerID `json:"computer_id,omitempty"`
}

func (q *Queries) LinkedControlReservation(
	ctx context.Context, id, account string, device remoteproto.DeviceID,
) (remoteproto.ComputerID, error) {
	result, err := q.generated.LinkedControlReservation(ctx, sqlcgen.LinkedControlReservationParams{
		ReservationID: dbText(id), AccountID: account, DeviceID: string(device),
	})
	return remoteproto.ComputerID(result), err
}

func (q *Queries) ActiveControlReservation(
	ctx context.Context, id, account string, device remoteproto.DeviceID, at time.Time,
) (SlotReservation, error) {
	row, err := q.generated.ActiveControlReservation(ctx, sqlcgen.ActiveControlReservationParams{
		ID: id, AccountID: account, DeviceID: string(device), ExpiresAt: timestamp(at),
	})
	return SlotReservation{ID: row.ID, ExpiresAt: row.ExpiresAt.Time}, err
}

func (q *Queries) CreateControlReservation(
	ctx context.Context, account string, device remoteproto.DeviceID, reservation SlotReservation,
) (SlotReservation, error) {
	row, err := q.generated.CreateControlReservation(ctx, sqlcgen.CreateControlReservationParams{
		ID: reservation.ID, AccountID: account, DeviceID: string(device), ExpiresAt: timestamp(reservation.ExpiresAt),
	})
	return SlotReservation{ID: row.ID, ExpiresAt: row.ExpiresAt.Time}, err
}

func (q *Queries) ReleaseControlReservation(
	ctx context.Context, id, account string, device remoteproto.DeviceID, at time.Time,
) error {
	return q.generated.ReleaseControlReservation(ctx, sqlcgen.ReleaseControlReservationParams{
		ID: id, AccountID: account, DeviceID: string(device), ReleasedAt: timestamp(at),
	})
}

func (q *Queries) ConsumeControlReservation(
	ctx context.Context, id, account string, device remoteproto.DeviceID, at time.Time,
) (bool, error) {
	count, err := q.generated.ConsumeControlReservation(ctx, sqlcgen.ConsumeControlReservationParams{
		ID: id, AccountID: account, DeviceID: string(device), AsOf: timestamp(at),
	})
	return count == 1, err
}

type ControlLinkToken struct {
	AccountID     string
	DeviceID      remoteproto.DeviceID
	ReservationID string
	ExpiresAt     time.Time
	UsedAt        time.Time
	UsedByJKT     string
	ComputerID    remoteproto.ComputerID
}

func (q *Queries) ControlLinkToken(ctx context.Context, hash []byte) (ControlLinkToken, error) {
	row, err := q.generated.ControlLinkToken(ctx, hash)
	return ControlLinkToken{
		AccountID: row.AccountID, DeviceID: remoteproto.DeviceID(row.DeviceID),
		ReservationID: row.ReservationID.String, ExpiresAt: row.ExpiresAt.Time, UsedAt: row.UsedAt.Time,
		UsedByJKT: row.UsedByJkt.String, ComputerID: remoteproto.ComputerID(row.ResultComputerID.String),
	}, err
}

func (q *Queries) CreateControlLinkToken(ctx context.Context, hash []byte, token ControlLinkToken) error {
	return q.generated.CreateControlLinkToken(ctx, sqlcgen.CreateControlLinkTokenParams{
		TokenSha256: hash, AccountID: token.AccountID, DeviceID: string(token.DeviceID),
		ReservationID: dbText(token.ReservationID), ExpiresAt: timestamp(token.ExpiresAt),
	})
}

func (q *Queries) UseControlLinkToken(
	ctx context.Context, hash []byte, id remoteproto.ComputerID, jkt string, at time.Time,
) error {
	return q.generated.UseControlLinkToken(ctx, sqlcgen.UseControlLinkTokenParams{
		TokenSha256: hash, ResultComputerID: dbText(string(id)), UsedByJkt: dbText(jkt), UsedAt: timestamp(at),
	})
}

func (q *Queries) RecordControlGrant(ctx context.Context, claims remoteproto.GrantClaims) error {
	return q.generated.RecordControlGrant(ctx, sqlcgen.RecordControlGrantParams{
		Jti: claims.JTI, ComputerID: string(claims.Audience), DeviceID: string(claims.Subject),
		Route: string(claims.Route), IssuedAt: timestamp(time.Unix(claims.IssuedAt, 0)),
	})
}

func (q *Queries) UseComputerProof(ctx context.Context, jkt, jti string, expiresAt time.Time) (bool, error) {
	count, err := q.generated.UseComputerProof(ctx, sqlcgen.UseComputerProofParams{
		Jkt: jkt, Jti: jti, ExpiresAt: timestamp(expiresAt),
	})
	return count == 1, err
}

func (s *Store) CompactControlRecords(ctx context.Context, now time.Time) error {
	return errors.Join(
		s.generated.PurgeComputerProofs(ctx, timestamp(now)),
		s.generated.PurgeControlGrants(ctx, timestamp(now.AddDate(0, 0, -30))),
	)
}
