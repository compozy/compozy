package control

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

var (
	ErrBrowserCodeInvalid = &Error{http.StatusForbidden, "browser_code_invalid", "Sign-in didn't complete. Try again."}
	errBrowserCodeUsed    = &Error{
		http.StatusForbidden,
		"browser_code_used",
		"This sign-in code has already been used.",
	}
	errBrowserKeyMismatch = &Error{http.StatusForbidden, "browser_key_mismatch", "Sign-in didn't complete. Try again."}
)

func (s *Service) BrowserCodeComputer(
	ctx context.Context,
	input remoteproto.BrowserTokenRequest,
) (store.ControlComputer, error) {
	if !auth.ValidSecret(input.Code) || !validBrowserVerifier(input.CodeVerifier) || !auth.ValidSecret(input.JKT) {
		return store.ControlComputer{}, ErrInvalidInput
	}
	digest := sha256.Sum256([]byte(input.Code))
	code, err := s.db.BrowserCode(ctx, digest[:])
	if errors.Is(err, store.ErrAuthNotFound) {
		return store.ControlComputer{}, ErrBrowserCodeInvalid
	}
	if err != nil {
		return store.ControlComputer{}, err
	}
	return s.ComputerIdentity(ctx, remoteproto.ComputerID(code.ComputerID))
}

func (s *Service) RedeemBrowserCode(
	ctx context.Context,
	computer store.ControlComputer,
	proof ComputerRequestAuth,
	input remoteproto.BrowserTokenRequest,
) (Grant, error) {
	if err := s.consumeComputerProof(ctx, computer, proof); err != nil {
		return Grant{}, err
	}
	var result Grant
	err := s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		now := s.config.Now()
		code, err := readBrowserCode(ctx, q, computer, input, now)
		if err != nil {
			return err
		}
		tunnel, err := s.admitBrowserGrant(ctx, q, computer, now)
		if err != nil {
			return err
		}
		if err := admitBrowserKey(ctx, q, &account, code, tunnel.Hostname, now); err != nil {
			return err
		}
		device, err := findDevice(ctx, q, account.ID, remoteproto.DeviceID(code.DeviceID))
		if err != nil {
			return err
		}
		if err := q.UseBrowserCode(ctx, code.Hash, now); err != nil {
			return browserCodeError(err)
		}
		if err := q.AdvanceDeviceActivity(ctx, code.DeviceID, now); err != nil {
			return err
		}
		result, err = s.issueGrant(ctx, q, account, device, code.JKT, computer.ID, remoteproto.RouteCloud, true, now)
		return err
	})
	return result, err
}

func readBrowserCode(
	ctx context.Context,
	q *store.Queries,
	computer store.ControlComputer,
	input remoteproto.BrowserTokenRequest,
	now time.Time,
) (store.BrowserCode, error) {
	digest := sha256.Sum256([]byte(input.Code))
	code, err := q.BrowserCode(ctx, digest[:])
	if err != nil {
		return store.BrowserCode{}, browserCodeError(err)
	}
	verifier := sha256.Sum256([]byte(input.CodeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifier[:])
	if code.AccountID != computer.AccountID || code.ComputerID != string(computer.ID) ||
		subtle.ConstantTimeCompare([]byte(challenge), []byte(code.Challenge)) != 1 {
		return store.BrowserCode{}, ErrBrowserCodeInvalid
	}
	if code.JKT != input.JKT {
		return store.BrowserCode{}, errBrowserKeyMismatch
	}
	if !code.UsedAt.IsZero() {
		return store.BrowserCode{}, errBrowserCodeUsed
	}
	if !now.Before(code.ExpiresAt) {
		return store.BrowserCode{}, ErrBrowserCodeInvalid
	}
	return code, nil
}

func (s *Service) admitBrowserGrant(
	ctx context.Context, q *store.Queries, authenticated store.ControlComputer, now time.Time,
) (store.HostedTunnel, error) {
	tier, err := s.reconcilePlan(ctx, q, authenticated.AccountID, now)
	if err != nil {
		return store.HostedTunnel{}, err
	}
	computer, err := ownedComputer(ctx, q, authenticated.AccountID, authenticated.ID)
	if err != nil {
		return store.HostedTunnel{}, err
	}
	if computer.Key != authenticated.Key {
		return store.HostedTunnel{}, ErrProofInvalid
	}
	if err := admitGrant(computer, tier, remoteproto.RouteCloud); err != nil {
		return store.HostedTunnel{}, err
	}
	tunnel, err := q.LockHostedTunnel(ctx, computer.ID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && tunnel.State != remoteproto.HostedTunnelActive) {
		return store.HostedTunnel{}, ErrBrowserCodeInvalid
	}
	return tunnel, err
}

func admitBrowserKey(
	ctx context.Context,
	q *store.Queries,
	account *store.ControlAccount,
	code store.BrowserCode,
	origin string,
	now time.Time,
) error {
	key, err := q.FindDeviceKey(ctx, code.JKT)
	if err != nil {
		return browserCodeError(err)
	}
	if key.AccountID != account.ID || key.DeviceID != code.DeviceID || key.Kind != store.BrowserDevice ||
		key.Origin != origin {
		return ErrBrowserCodeInvalid
	}
	if key.DeviceRevoked || key.State == store.KeyRevoked {
		return errDeviceRevoked
	}
	if key.State == store.KeyActive {
		return nil
	}
	if err := q.ActivateBrowserKey(ctx, code.DeviceID, code.JKT, origin, now); err != nil {
		return browserCodeError(err)
	}
	account.SecurityEpoch, err = q.AdvanceSecurity(ctx, store.ControlEvent{
		Type: remoteproto.ControlSnapshot, AccountID: account.ID,
	})
	return err
}

func browserCodeError(err error) error {
	if errors.Is(err, store.ErrAuthNotFound) {
		return ErrBrowserCodeInvalid
	}
	return err
}

func validBrowserVerifier(verifier string) bool {
	return len(verifier) >= 43 && len(verifier) <= 128 && !strings.ContainsFunc(verifier, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("-._~", r))
	})
}
