// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

type ArchivedEvent struct {
	ID        int64          `json:"id"`
	AccountID string         `json:"account_id"`
	Type      string         `json:"type"`
	Actor     string         `json:"actor"`
	At        time.Time      `json:"at"`
	Details   map[string]any `json:"details"`
}

func (s *Store) WithBackupRecoveryLock(ctx context.Context, ship func() error) (retErr error) {
	conn, err := s.ConnectDirect(ctx)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		retErr = errors.Join(retErr, conn.Close(closeCtx))
	}()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1, hashtext(current_schema()))", 1129270363); err != nil {
		return err
	}
	return ship()
}

func (s *Store) ReadSecurityArchive(ctx context.Context, afterID int64) (time.Time, []ArchivedEvent, error) {
	var captured time.Time
	var events []ArchivedEvent
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Sequence assignment and commit order can differ, so fence concurrent appends before moving the cursor.
		if _, err := tx.Exec(ctx, "LOCK TABLE deleted_accounts, security_events IN SHARE MODE"); err != nil {
			return err
		}
		q := sqlcgen.New(tx)
		clock, err := q.ArchiveClock(ctx)
		if err != nil {
			return err
		}
		captured = clock.Time.UTC()
		rows, err := q.ArchiveSecurityEvents(ctx, afterID)
		if err != nil {
			return err
		}
		events = make([]ArchivedEvent, len(rows))
		for i, row := range rows {
			event := ArchivedEvent{
				ID:        row.ID,
				AccountID: row.AccountID.String,
				Type:      row.Type,
				Actor:     row.Actor,
				At:        row.At.Time.UTC(),
			}
			if err := json.Unmarshal(row.DetailsJson, &event.Details); err != nil {
				return errors.New("invalid archived security event")
			}
			events[i] = event
		}
		return nil
	})
	return captured, events, err
}

func (s *Store) CheckRestoreReady(ctx context.Context, expected time.Time) error {
	pending, err := s.generated.HasPendingCloudRestore(ctx)
	if err != nil {
		return connectionError("check cloud restore state", err)
	}
	if pending {
		return errors.New("cloud restore finalization is pending")
	}
	if !expected.IsZero() {
		row, err := s.generated.GetCloudRestore(ctx, timestamp(expected))
		if err != nil || !row.CompletedAt.Valid {
			return errors.New("expected cloud restore has not been finalized")
		}
	}
	return nil
}

type RestoreState struct {
	Applied, Completed bool
}

func (s *Store) BeginRestore(ctx context.Context, point, through time.Time) (RestoreState, error) {
	if err := s.generated.BeginCloudRestore(ctx, sqlcgen.BeginCloudRestoreParams{
		RestorePoint: timestamp(point), ArchiveThrough: timestamp(through),
	}); err != nil {
		return RestoreState{}, errors.New("start cloud restore: another restore may be pending")
	}
	row, err := s.generated.GetCloudRestore(ctx, timestamp(point))
	if err != nil {
		return RestoreState{}, err
	}
	if !row.ArchiveThrough.Time.Equal(through) {
		return RestoreState{}, errors.New("retry with the original cloud restore archive cutoff")
	}
	return RestoreState{row.AppliedAt.Valid, row.CompletedAt.Valid}, nil
}

func (s *Store) ApplyRestore(
	ctx context.Context, point, at time.Time, archiveID int64, events []ArchivedEvent,
) error {
	return s.Transaction(ctx, func(q *Queries) error {
		row, err := q.generated.LockCloudRestore(ctx, timestamp(point))
		if err != nil || row.AppliedAt.Valid {
			return err
		}
		if err := q.invalidateRestoreCredentials(ctx, point, at); err != nil {
			return err
		}
		if err := q.generated.AdvanceSecurityArchiveSequence(ctx, archiveID); err != nil {
			return err
		}
		for _, event := range events {
			if err := q.replayRestoreEvent(ctx, event); err != nil {
				return fmt.Errorf("replay security event %d: %w", event.ID, err)
			}
		}
		return q.generated.MarkRestoreApplied(ctx, sqlcgen.MarkRestoreAppliedParams{
			RestorePoint: timestamp(point), AppliedAt: timestamp(at),
		})
	})
}

func (s *Store) FinishRestore(ctx context.Context, point, at time.Time) error {
	return s.Transaction(ctx, func(q *Queries) error {
		changed, err := q.generated.FinishCloudRestore(ctx, sqlcgen.FinishCloudRestoreParams{
			RestorePoint: timestamp(point), CompletedAt: timestamp(at),
		})
		if err != nil || changed == 0 {
			return err
		}
		return q.AppendSecurityEvent(ctx, SecurityEvent{
			Type: "cloud_restore_finalized", Actor: "system", At: at,
			Details: map[string]any{"restore_point": point.Format(time.RFC3339Nano)},
		})
	})
}

func (q *Queries) replayRestoreEvent(ctx context.Context, event ArchivedEvent) error {
	exists, err := q.generated.HasArchivedSecurityEvent(ctx, sqlcgen.HasArchivedSecurityEventParams{
		EventID: event.ID, EventAt: timestamp(event.At), EventType: event.Type,
	})
	if err != nil || exists {
		return err
	}
	replay, err := q.applyRestoreRevocation(ctx, event)
	if err != nil || !replay || event.Type == "account_deleted" {
		return err
	}
	return q.AppendSecurityEvent(ctx, SecurityEvent{
		AccountID: event.AccountID, Type: event.Type, Actor: event.Actor, At: event.At, Details: event.Details,
	})
}

func (q *Queries) applyRestoreRevocation(ctx context.Context, event ArchivedEvent) (bool, error) {
	device, _ := event.Details["device_id"].(string)
	computer, _ := event.Details["computer_id"].(string)
	switch event.Type {
	case "device_revoked":
		return true, q.revokeRestoredDevice(ctx, event, device)
	case "device_session_revoked":
		session, _ := event.Details["session_id"].(string)
		if session == "" || event.AccountID == "" {
			return false, errors.New("archived device-session revocation lacks its target")
		}
		_, err := q.RevokeAdminDeviceSession(ctx, event.AccountID, session, event.At)
		return true, err
	case "identity_detached":
		provider, _ := event.Details["provider"].(string)
		subject, _ := event.Details["subject"].(string)
		if provider == "" || subject == "" || event.AccountID == "" {
			return false, errors.New("archived identity detachment lacks its target")
		}
		_, err := q.DetachAdminIdentity(ctx, event.AccountID, provider, subject)
		return true, err
	case "device_key_revoked", "browser_key_revoked":
		jkt, _ := event.Details["jkt"].(string)
		if jkt == "" {
			return true, q.revokeRestoredDevice(ctx, event, device)
		}
		return true, q.generated.RevokeRestoredBrowserKey(ctx, sqlcgen.RevokeRestoredBrowserKeyParams{
			AccountID: event.AccountID, Jkt: jkt, RevokedAt: timestamp(event.At),
		})
	case "device_admin_changed":
		admin, _ := event.Details["admin"].(bool)
		if admin {
			return false, nil
		}
		return true, q.generated.DisableRestoredAdmin(ctx, sqlcgen.DisableRestoredAdminParams{
			ID: device, AccountID: event.AccountID,
		})
	case "device_scopes_changed":
		return true, q.restoreDeviceScopes(ctx, event, remoteproto.DeviceID(device))
	case "computer_unlinked":
		return true, q.UnlinkControlComputer(ctx, ControlComputer{
			ID: remoteproto.ComputerID(computer), AccountID: event.AccountID,
		}, event.At)
	case "computer_remote_access_changed":
		enabled, _ := event.Details["remote_enabled"].(bool)
		if enabled {
			return false, nil
		}
		return true, q.generated.RevokeRestoredComputerAccess(ctx, sqlcgen.RevokeRestoredComputerAccessParams{
			ID: computer, AccountID: event.AccountID,
		})
	case "hosted_tunnel_deleted":
		generation, _ := event.Details["generation"].(float64)
		return true, q.generated.MarkRestoredTunnelDeleting(ctx, sqlcgen.MarkRestoredTunnelDeletingParams{
			ComputerID: computer, AccountID: event.AccountID, Generation: int64(generation),
		})
	case "hosted_tunnel_takedown":
		return true, q.restoreAdminTunnelTakedown(ctx, event, computer)
	case "cloud_signing_key_retired":
		kid, _ := event.Details["kid"].(string)
		if kid == "" {
			return false, errors.New("archived signing-key retirement lacks its target")
		}
		_, err := q.generated.RetireSigningKey(ctx, sqlcgen.RetireSigningKeyParams{
			Kid: kid, RetiredAt: timestamp(event.At),
		})
		return true, err
	case "account_token_revoked":
		token, _ := event.Details["token_id"].(string)
		if token == "" || event.AccountID == "" {
			return false, errors.New("archived account-token revocation lacks its target")
		}
		return true, q.generated.RevokeRestoredAccountToken(ctx, sqlcgen.RevokeRestoredAccountTokenParams{
			TokenID: token, AccountID: event.AccountID, RevokedAt: timestamp(event.At),
		})
	case "account_suspended", "account_unsuspended", "account_deletion_started", "account_deleted":
		return true, q.restoreAccountState(ctx, event)
	default:
		return false, nil
	}
}

func (q *Queries) restoreAdminTunnelTakedown(ctx context.Context, event ArchivedEvent, computer string) error {
	hostname, _ := event.Details["hostname"].(string)
	generation, _ := event.Details["generation"].(float64)
	if hostname == "" || computer == "" || generation < 1 || event.AccountID == "" {
		return errors.New("archived tunnel takedown lacks its target")
	}
	return q.generated.RestoreAdminTunnelTakedown(ctx, sqlcgen.RestoreAdminTunnelTakedownParams{
		AccountID: event.AccountID, ComputerID: computer, Hostname: hostname,
		Generation: int64(generation), At: timestamp(event.At),
	})
}

func (q *Queries) restoreDeviceScopes(ctx context.Context, event ArchivedEvent, device remoteproto.DeviceID) error {
	if _, err := q.ControlDevice(ctx, event.AccountID, device); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	updates, ok := event.Details["scopes_by_computer"].(map[string]any)
	if !ok {
		return q.generated.RestrictRestoredScopes(ctx, sqlcgen.RestrictRestoredScopesParams{
			DeviceID: string(device), AccountID: event.AccountID,
		})
	}
	for id, value := range updates {
		computer, err := q.ControlComputer(ctx, remoteproto.ComputerID(id))
		if errors.Is(err, ErrNotFound) || (err == nil && computer.AccountID != event.AccountID) {
			continue
		}
		if err != nil {
			return err
		}
		items, ok := value.([]any)
		if !ok {
			return errors.New("invalid archived computer scopes")
		}
		scopes := make([]remoteproto.Scope, 0, len(items))
		for _, item := range items {
			name, _ := item.(string)
			scope := remoteproto.Scope(name)
			if !scope.Valid() {
				return errors.New("invalid archived device scope")
			}
			scopes = append(scopes, scope)
		}
		if _, err := q.SetControlDeviceScopes(ctx, device, computer.ID, scopes); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queries) revokeRestoredDevice(ctx context.Context, event ArchivedEvent, id string) error {
	if id == "" || event.AccountID == "" {
		return errors.New("archived device revocation lacks its target")
	}
	_, err := q.ControlDevice(ctx, event.AccountID, remoteproto.DeviceID(id))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return q.RevokeControlDevice(ctx, event.AccountID, remoteproto.DeviceID(id), event.At)
}

func (q *Queries) restoreAccountState(ctx context.Context, event ArchivedEvent) error {
	state := "active"
	var suspendedAt time.Time
	switch event.Type {
	case "account_suspended":
		state, suspendedAt = "suspended", event.At
	case "account_deleted", "account_deletion_started":
		state = "deleting"
	}
	if err := q.generated.MarkRestoredAccountState(ctx, sqlcgen.MarkRestoredAccountStateParams{
		AccountID: event.AccountID, State: state, SuspendedAt: timestamp(suspendedAt),
	}); err != nil {
		return err
	}
	if state == "deleting" {
		_, err := q.LockAuthAccount(ctx, event.AccountID)
		if errors.Is(err, ErrAuthNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if event.Type == "account_deleted" {
			return q.FinishAccountDeletion(ctx, event.AccountID, event.At)
		}
		if err := q.BeginAccountDeletion(ctx, event.AccountID, event.At); err != nil {
			return err
		}
	}
	if state != "active" {
		return q.generated.RevokeRestoredAccountTokens(ctx, sqlcgen.RevokeRestoredAccountTokensParams{
			AccountID: event.AccountID, RevokedAt: timestamp(event.At),
		})
	}
	return nil
}

func (q *Queries) invalidateRestoreCredentials(ctx context.Context, point, at time.Time) error {
	if err := q.generated.BumpRestoreEpochs(ctx); err != nil {
		return err
	}
	revokedAt := timestamp(at)
	for _, apply := range []func(context.Context, pgtype.Timestamptz) error{
		q.generated.RevokeRestoreDeviceSessions, q.generated.RevokeRestoreWebSessions,
		q.generated.RetireRestoredEnrollmentCredentials, q.generated.RetireRestoredBrowserCredentials,
	} {
		if err := apply(ctx, revokedAt); err != nil {
			return err
		}
	}
	if err := q.generated.RetireRestoredGrantCredentials(ctx); err != nil {
		return err
	}
	return q.generated.DropRestoredTokenInstances(ctx, timestamp(point))
}

type RestoredTunnel struct {
	Hostname, CloudflareID string
	Generation             int64
}

func (s *Store) RestoreKnownTunnels(ctx context.Context) ([]RestoredTunnel, error) {
	rows, err := s.generated.RestoreKnownTunnels(ctx)
	if err != nil {
		return nil, err
	}
	tunnels := make([]RestoredTunnel, len(rows))
	for i, row := range rows {
		tunnels[i] = RestoredTunnel{
			Hostname:     row.Hostname,
			CloudflareID: row.CfTunnelID.String,
			Generation:   row.Generation,
		}
	}
	return tunnels, nil
}
