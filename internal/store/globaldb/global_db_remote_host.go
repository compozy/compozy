package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ remotehost.Store = (*RemoteHostRepo)(nil)

func (r *RemoteHostRepo) RemoteIdentity(ctx context.Context) (remotehost.Identity, error) {
	row, err := r.readQueries.GetRemoteIdentity(ctx)
	if err != nil {
		return remotehost.Identity{}, remoteRecordError(err)
	}
	linked, err := time.Parse(time.RFC3339Nano, row.LinkedAt)
	if err != nil {
		return remotehost.Identity{}, err
	}
	leaf, err := time.Parse(time.RFC3339Nano, row.TlsLeafNotAfter)
	if err != nil {
		return remotehost.Identity{}, err
	}
	return remotehost.Identity{
		ComputerID: remoteproto.ComputerID(row.ComputerID), AccountID: row.AccountID,
		ComputerKeyRef: row.ComputerKeyRef, TLSRootRef: row.TlsRootRef, TLSRootSHA256: row.TlsRootSha256,
		TLSLeafNotAfter: leaf, LinkedAt: linked,
	}, nil
}

func (r *RemoteHostRepo) SaveRemoteIdentity(ctx context.Context, identity remotehost.Identity) error {
	return r.executeWriteQuery(ctx, "globaldb.SaveRemoteIdentity", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.PutRemoteIdentity(ctx, sqlcgen.PutRemoteIdentityParams{
			ComputerID: string(identity.ComputerID), AccountID: identity.AccountID,
			ComputerKeyRef: identity.ComputerKeyRef, TlsRootRef: identity.TLSRootRef,
			TlsRootSha256: identity.TLSRootSHA256, LinkedAt: store.FormatTimestamp(identity.LinkedAt),
			TlsLeafNotAfter: store.FormatTimestamp(identity.TLSLeafNotAfter),
		})
	})
}

func (r *RemoteHostRepo) RemoteLeaseSequence(ctx context.Context) (uint64, error) {
	row, err := r.readQueries.GetRemoteIdentity(ctx)
	if err != nil {
		return 0, remoteRecordError(err)
	}
	if row.LeaseSequence < 0 {
		return 0, remoteproto.ErrLeaseInvalid
	}
	return uint64(row.LeaseSequence), nil
}

func (r *RemoteHostRepo) RemoteLastMint(ctx context.Context) (remoteproto.SessionMinted, error) {
	row, err := r.readQueries.GetRemoteIdentity(ctx)
	if err != nil {
		return remoteproto.SessionMinted{}, remoteRecordError(err)
	}
	return remoteproto.SessionMinted{
		At: row.LastMintAt, DeviceID: remoteproto.DeviceID(row.LastMintDeviceID), JKT: row.LastMintJkt,
	}, nil
}

func (r *RemoteHostRepo) UpdateRemoteLeaf(ctx context.Context, id remoteproto.ComputerID, expires time.Time) error {
	return r.executeWriteQuery(ctx, "globaldb.UpdateRemoteLeaf", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpdateRemoteLeaf(ctx, sqlcgen.UpdateRemoteLeafParams{
			ComputerID: string(id), TlsLeafNotAfter: store.FormatTimestamp(expires),
		})
	})
}

func (r *RemoteHostRepo) RemoteSnapshot(
	ctx context.Context,
	identity remotehost.Identity,
) (remoteproto.SecuritySnapshot, error) {
	row, err := r.readQueries.GetRemoteSecurityState(ctx)
	if err != nil {
		return remoteproto.SecuritySnapshot{}, remoteRecordError(err)
	}
	if row.AppliedEpoch <= 0 {
		return remoteproto.SecuritySnapshot{}, remoteproto.ErrSnapshotInvalid
	}
	snapshot := remoteproto.SecuritySnapshot{
		AccountID:     identity.AccountID,
		Linked:        true,
		SecurityEpoch: uint64(row.AppliedEpoch),
	}
	if err := json.Unmarshal([]byte(row.DevicesJson), &snapshot.Devices); err != nil {
		return remoteproto.SecuritySnapshot{}, err
	}
	return snapshot, nil
}

func (r *RemoteHostRepo) RemoteSession(ctx context.Context, hash [32]byte) (remotehost.Session, error) {
	row, err := r.readQueries.GetRemoteSession(ctx, hash[:])
	if err != nil {
		return remotehost.Session{}, remoteRecordError(err)
	}
	return remoteSessionFromRow(row)
}

func (r *RemoteHostRepo) RemoteGrantUsed(ctx context.Context, jti string) (bool, error) {
	return r.readQueries.HasRemoteGrant(ctx, jti)
}

func (r *RemoteHostRepo) RemoteDeviceConnections(ctx context.Context) (map[remoteproto.DeviceID]time.Time, error) {
	rows, err := r.readQueries.ListRemoteDeviceConnections(ctx)
	if err != nil {
		return nil, err
	}
	connections := make(map[remoteproto.DeviceID]time.Time, len(rows))
	for _, row := range rows {
		connections[remoteproto.DeviceID(row.DeviceID)] = remoteTime(row.LastSeenAt)
	}
	return connections, nil
}

func (r *RemoteHostRepo) RemoteApprovals(ctx context.Context) ([]remotehost.Approval, error) {
	rows, err := r.readQueries.ListRemoteApprovals(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]remotehost.Approval, 0, len(rows))
	for _, row := range rows {
		result = append(result, remoteApprovalFromRow(row))
	}
	return result, nil
}

func (r *RemoteHostRepo) RemoteApproval(
	ctx context.Context,
	id remoteproto.DeviceID,
	jkt string,
) (remotehost.Approval, error) {
	return remoteHostTx{q: r.readQueries}.Approval(ctx, id, jkt)
}

func (r *RemoteHostRepo) RemoteApprovalByID(ctx context.Context, id string) (remotehost.Approval, error) {
	row, err := r.readQueries.GetRemoteApprovalByID(ctx, id)
	if err != nil {
		return remotehost.Approval{}, remoteRecordError(err)
	}
	return remoteApprovalFromRow(row), nil
}

func (r *RemoteHostRepo) WriteRemote(ctx context.Context, run func(context.Context, remotehost.Tx) error) error {
	return r.executeWriteQuery(ctx, "globaldb.WriteRemote", func(ctx context.Context, q *sqlcgen.Queries) error {
		return run(ctx, remoteHostTx{q})
	})
}

func remoteRecordError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return remotehost.ErrMissing
	}
	return err
}

type remoteHostTx struct{ q *sqlcgen.Queries }

var _ remotehost.Tx = remoteHostTx{}

func (t remoteHostTx) UseGrant(ctx context.Context, jti string, expiry time.Time) (bool, error) {
	n, err := t.q.UseRemoteGrant(ctx, sqlcgen.UseRemoteGrantParams{Jti: jti, ExpiresAt: expiry.UnixNano()})
	return n > 0, err
}

func (t remoteHostTx) InsertSession(ctx context.Context, session remotehost.Session) error {
	scopes, err := json.Marshal(session.Scopes)
	if err != nil {
		return err
	}
	var browser int64
	if session.Browser {
		browser = 1
	}
	if err := t.q.InsertRemoteSession(ctx, sqlcgen.InsertRemoteSessionParams{
		ID:               session.ID,
		CredentialSha256: session.CredentialHash[:],
		DeviceID:         string(session.DeviceID),
		DeviceJkt:        session.JKT,
		Scopes:           string(scopes),
		Route:            string(session.Route),
		Browser:          browser,
		CreatedAt:        session.CreatedAt.UnixNano(),
		ExpiresAt:        session.ExpiresAt.UnixNano(),
		SigningKeyID:     session.SigningKeyID,
	}); err != nil {
		return err
	}
	if err := t.q.RecordRemoteDeviceConnection(ctx, sqlcgen.RecordRemoteDeviceConnectionParams{
		DeviceID: string(session.DeviceID), LastSeenAt: session.CreatedAt.UnixNano(),
	}); err != nil {
		return err
	}
	return t.q.RecordRemoteLastMint(ctx, sqlcgen.RecordRemoteLastMintParams{
		MintedAt: session.CreatedAt.Unix(), DeviceID: string(session.DeviceID), Jkt: session.JKT,
	})
}

func (t remoteHostTx) ActiveSessions(ctx context.Context, now time.Time) ([]remotehost.Session, error) {
	rows, err := t.q.ActiveRemoteSessions(ctx, now.UnixNano())
	if err != nil {
		return nil, err
	}
	result := make([]remotehost.Session, 0, len(rows))
	for _, row := range rows {
		session, err := remoteSessionFromRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, session)
	}
	return result, nil
}

func remoteSessionFromRow(row sqlcgen.RemoteSession) (remotehost.Session, error) {
	var scopes []remoteproto.Scope
	if err := json.Unmarshal([]byte(row.Scopes), &scopes); err != nil {
		return remotehost.Session{}, err
	}
	if len(row.CredentialSha256) != 32 {
		return remotehost.Session{}, errors.New("stored remote credential hash is invalid")
	}
	return remotehost.Session{
		ID:             row.ID,
		CredentialHash: [32]byte(row.CredentialSha256),
		DeviceID:       remoteproto.DeviceID(row.DeviceID),
		JKT:            row.DeviceJkt,
		SigningKeyID:   row.SigningKeyID,
		Scopes:         scopes,
		Route:          remoteproto.RouteKind(row.Route),
		Browser:        row.Browser == 1,
		CreatedAt:      remoteTime(row.CreatedAt),
		ExpiresAt:      remoteTime(row.ExpiresAt),
		Revoked:        row.RevokedAt.Valid,
	}, nil
}

func (t remoteHostTx) RevokeSession(ctx context.Context, id string, now time.Time) error {
	return t.q.RevokeRemoteSession(
		ctx,
		sqlcgen.RevokeRemoteSessionParams{ID: id, RevokedAt: remoteNullableTime(new(now))},
	)
}

func (t remoteHostTx) Approval(
	ctx context.Context,
	device remoteproto.DeviceID,
	jkt string,
) (remotehost.Approval, error) {
	row, err := t.q.GetRemoteApproval(ctx, sqlcgen.GetRemoteApprovalParams{DeviceID: string(device), Jkt: jkt})
	if err != nil {
		return remotehost.Approval{}, remoteRecordError(err)
	}
	return remoteApprovalFromRow(row), nil
}

func (t remoteHostTx) SetApproval(ctx context.Context, approval remotehost.Approval) error {
	return t.q.PutRemoteApproval(ctx, sqlcgen.PutRemoteApprovalParams{
		ID: approval.ID, DeviceID: string(approval.DeviceID), Jkt: approval.JKT, Name: approval.Name,
		State: approval.State, CreatedAt: approval.CreatedAt.UnixNano(), ExpiresAt: approval.ExpiresAt.UnixNano(),
		DecidedAt: remoteNullableTime(approval.DecidedAt), DecisionJti: approval.DecisionJTI,
	})
}

func remoteApprovalFromRow(row sqlcgen.RemoteDeviceApproval) remotehost.Approval {
	var decided *time.Time
	if row.DecidedAt.Valid {
		decided = new(remoteTime(row.DecidedAt.Int64))
	}
	return remotehost.Approval{
		ID:       row.ID,
		DeviceID: remoteproto.DeviceID(row.DeviceID),
		JKT:      row.Jkt,
		Name:     row.Name,
		State:    row.State,
		CreatedAt: remoteTime(
			row.CreatedAt,
		),
		ExpiresAt:   remoteTime(row.ExpiresAt),
		DecidedAt:   decided,
		DecisionJTI: row.DecisionJti,
	}
}

func (t remoteHostTx) UseDecision(ctx context.Context, jti string, expiry time.Time) (bool, error) {
	n, err := t.q.UseRemoteApprovalDecision(
		ctx,
		sqlcgen.UseRemoteApprovalDecisionParams{Jti: jti, ExpiresAt: expiry.UnixNano()},
	)
	return n > 0, err
}

func (t remoteHostTx) SaveSnapshot(ctx context.Context, snapshot remoteproto.SecuritySnapshot, now time.Time) error {
	if snapshot.SecurityEpoch == 0 || snapshot.SecurityEpoch > math.MaxInt64 {
		return remoteproto.ErrSnapshotInvalid
	}
	devices, err := json.Marshal(snapshot.Devices)
	if err != nil {
		return err
	}
	return t.q.PutRemoteSecurityState(ctx, sqlcgen.PutRemoteSecurityStateParams{
		AppliedEpoch: int64(snapshot.SecurityEpoch), DevicesJson: string(devices), AppliedAt: now.UnixNano(),
	})
}

func (t remoteHostTx) ClearLink(ctx context.Context) error {
	for _, write := range []func(context.Context) error{
		t.q.DeleteRemoteIdentity, t.q.DeleteRemoteSecurityState, t.q.PurgeRemoteSessions,
		t.q.PurgeRemoteDeviceConnections, t.q.PurgeRemoteApprovals, t.q.PurgeRemoteGrants, t.q.PurgeRemoteApprovalDecisions,
	} {
		if err := write(ctx); err != nil {
			return fmt.Errorf("clear linked computer state: %w", err)
		}
	}
	return nil
}

func (t remoteHostTx) AdvanceLeaseSequence(ctx context.Context, computer remoteproto.ComputerID, seq uint64) error {
	if seq == 0 || seq > math.MaxInt64 {
		return remoteproto.ErrLeaseInvalid
	}
	n, err := t.q.AdvanceRemoteLeaseSequence(ctx, sqlcgen.AdvanceRemoteLeaseSequenceParams{
		Sequence: int64(seq), ComputerID: string(computer),
	})
	if err == nil && n == 0 {
		return remoteproto.ErrLeaseInvalid
	}
	return err
}

func (t remoteHostTx) Sweep(ctx context.Context, now time.Time) error {
	if err := t.q.SweepRemoteSessions(ctx, now.Add(-remotehost.SessionTTL).UnixNano()); err != nil {
		return err
	}
	if err := t.q.SweepRemoteGrants(ctx, now.UnixNano()); err != nil {
		return err
	}
	return t.q.SweepRemoteApprovalDecisions(ctx, now.UnixNano())
}

func remoteNullableTime(value *time.Time) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value.UnixNano(), Valid: true}
}

func remoteTime(nanos int64) time.Time { return time.Unix(0, nanos).UTC() }
