package auth

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Service) Enroll(
	ctx context.Context, credentials DeviceProof, input remoteproto.DeviceEnrollRequest,
) (remoteproto.DeviceEnrollResponse, error) {
	var result remoteproto.DeviceEnrollResponse
	if !validCredential(credentials.Token, "enr_") {
		return result, errEnrollment
	}
	if input.PublicJWK.Kty != "EC" || (input.Kind != "desktop" && input.Kind != "cli") ||
		!validLabel(input.Name, 160) || !validLabel(input.InstallationID, 128) {
		return result, ErrInvalidRequest
	}
	jkt, err := input.PublicJWK.Thumbprint()
	if err != nil {
		return result, ErrInvalidRequest
	}
	now := s.now()
	hash := hashSecret(credentials.Token)
	token, err := s.db.FindEnrollment(ctx, hash, now)
	if errors.Is(err, store.ErrAuthNotFound) || (err == nil && token.InstallationID != input.InstallationID) {
		return result, errEnrollment
	}
	if err != nil {
		return result, err
	}
	if err := s.verifyDeviceProof(ctx, credentials, jkt, "", now); err != nil {
		return result, err
	}
	err = s.db.Transaction(ctx, func(q *store.Queries) error {
		var err error
		result, err = s.enrollDevice(ctx, q, token, input, hash, jkt, s.now())
		return err
	})
	return result, err
}

func (s *Service) enrollDevice(
	ctx context.Context, q *store.Queries, token store.EnrollmentToken, input remoteproto.DeviceEnrollRequest,
	hash []byte, jkt string, now time.Time,
) (remoteproto.DeviceEnrollResponse, error) {
	var empty remoteproto.DeviceEnrollResponse
	account, err := q.LockAuthAccount(ctx, token.AccountID)
	if err != nil {
		return empty, err
	}
	if err := requireActive(account); err != nil {
		return empty, err
	}
	if err := s.writeLimit(account.ID); err != nil {
		return empty, err
	}
	if err := q.ConsumeEnrollment(ctx, hash, now); err != nil {
		if errors.Is(err, store.ErrAuthNotFound) {
			return empty, errEnrollment
		}
		return empty, err
	}
	device, err := ensureEnrolledDevice(ctx, q, token, input, jkt, now)
	if err != nil {
		return empty, err
	}
	session := store.AuthDeviceSession{
		ID: "ds_" + Secret(), AccountID: account.ID, DeviceID: device.ID, JKT: jkt, Provider: token.Provider,
	}
	tokens, err := s.issueTokens(session, now)
	if err != nil {
		return empty, err
	}
	session.RefreshHash = hashSecret(tokens.RefreshToken)
	if err := q.CreateAuthDeviceSession(ctx, session, now); err != nil {
		return empty, err
	}
	if err := touchDeviceActivity(ctx, q, session, now); err != nil {
		return empty, err
	}
	return remoteproto.DeviceEnrollResponse{
		DeviceID: remoteproto.DeviceID(device.ID), Admin: device.Admin, DeviceSessionTokens: tokens,
	}, nil
}

func ensureEnrolledDevice(
	ctx context.Context, q *store.Queries, token store.EnrollmentToken, input remoteproto.DeviceEnrollRequest,
	jkt string, now time.Time,
) (store.AuthDevice, error) {
	population, admins, err := q.DevicePopulation(ctx, token.AccountID)
	if err != nil {
		return store.AuthDevice{}, err
	}
	recoverAdmin := token.FreshSignIn && admins == 0
	key, err := q.FindDeviceKey(ctx, jkt)
	switch {
	case errors.Is(err, store.ErrAuthNotFound):
		return registerEnrolledDevice(ctx, q, token, input, jkt, population == 0 || recoverAdmin, now)
	case err != nil:
		return store.AuthDevice{}, err
	case key.AccountID != token.AccountID || key.State != store.KeyActive || key.DeviceRevoked ||
		key.Kind == store.BrowserDevice:
		return store.AuthDevice{}, errKeyRevoked
	}
	device := store.AuthDevice{ID: key.DeviceID, AccountID: key.AccountID, Admin: key.Admin}
	if recoverAdmin && !device.Admin {
		err := recoverDeviceAdmin(ctx, q, device, now)
		device.Admin = err == nil
		return device, err
	}
	return device, nil
}

func registerEnrolledDevice(
	ctx context.Context, q *store.Queries, token store.EnrollmentToken, input remoteproto.DeviceEnrollRequest,
	jkt string, admin bool, now time.Time,
) (store.AuthDevice, error) {
	d := store.AuthDevice{
		ID: "dev_" + Secret(), AccountID: token.AccountID, Name: input.Name, Kind: input.Kind,
		InstallationID: input.InstallationID, Admin: admin,
	}
	if err := q.CreateAuthDevice(ctx, d, now); err != nil {
		return d, err
	}
	err := q.CreateDeviceKey(ctx, store.AuthDeviceKey{
		JKT: jkt, DeviceID: d.ID, JWK: input.PublicJWK, State: store.KeyActive,
	}, now)
	if errors.Is(err, store.ErrAuthNotFound) {
		return d, errKeyRevoked
	}
	if err != nil {
		return d, err
	}
	if err := q.RecordAuthDeviceChange(ctx, d.AccountID); err != nil {
		return d, err
	}
	details := map[string]any{deviceIDDetail: d.ID, "kind": d.Kind, "admin": d.Admin, "location": token.Location}
	if _, err := netip.ParseAddr(strings.TrimSpace(d.Name)); err != nil {
		details["device_name"] = d.Name
	}
	return d, q.AppendSecurityEvent(ctx, store.SecurityEvent{
		Type: "device_enrolled", AccountID: d.AccountID, Actor: accountActorPrefix + d.ID, At: now, Details: details,
	})
}

func recoverDeviceAdmin(ctx context.Context, q *store.Queries, d store.AuthDevice, now time.Time) error {
	if err := q.PromoteRecoveredAdmin(ctx, d.ID); err != nil {
		return err
	}
	if err := q.RecordAuthDeviceChange(ctx, d.AccountID); err != nil {
		return err
	}
	return q.AppendSecurityEvent(ctx, store.SecurityEvent{
		Type: "device_admin_changed", AccountID: d.AccountID, Actor: accountActorPrefix + d.ID, At: now,
		Details: map[string]any{deviceIDDetail: d.ID, "admin": true, "reason": "recovery"},
	})
}
