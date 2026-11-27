// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

// MutateRemoteDevice applies a trusted device's request through the linked computer.
func (s *Service) MutateRemoteDevice(
	ctx context.Context, computer store.ControlComputer, computerProof ComputerRequestAuth,
	mutation remoteproto.RemoteDeviceMutation,
) error {
	if mutation.DeviceID == "" || mutation.CallerJKT == "" || mutation.DPoP == "" {
		return ErrInvalidInput
	}
	if err := s.consumeComputerProof(ctx, computer, computerProof); err != nil {
		return err
	}
	return s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		caller, err := s.authorizeRemoteDeviceMutation(ctx, q, account, computer, mutation)
		if err != nil {
			return err
		}
		if mutation.Scopes != nil {
			_, err = s.patchOwnedDevice(ctx, q, account.ID, caller, mutation.DeviceID, remoteproto.AccountDevicePatch{
				Scopes: map[remoteproto.ComputerID][]remoteproto.Scope{computer.ID: *mutation.Scopes},
			})
			return err
		}
		if _, err := findDevice(ctx, q, account.ID, mutation.DeviceID); err != nil {
			return err
		}
		_, err = s.revokeDevice(ctx, q, account.ID, mutation.DeviceID, auditActor(caller))
		return err
	})
}

func (s *Service) authorizeRemoteDeviceMutation(
	ctx context.Context, q *store.Queries, account store.ControlAccount, computer store.ControlComputer,
	mutation remoteproto.RemoteDeviceMutation,
) (remoteproto.DeviceID, error) {
	if err := activeAccount(account); err != nil {
		return "", err
	}
	current, err := ownedComputer(ctx, q, account.ID, computer.ID)
	if err != nil {
		return "", err
	}
	if computer.Key != current.Key {
		return "", ErrProofInvalid
	}
	if !current.RemoteEnabled || current.PlanLimited {
		return "", errComputerDisabled
	}
	owner, err := q.FindDeviceKey(ctx, mutation.CallerJKT)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrUnauthorized
	}
	if err != nil {
		return "", err
	}
	if owner.AccountID != account.ID || owner.DeviceRevoked || owner.State != store.KeyActive || !owner.Admin {
		return "", errAdminRequired
	}
	caller := remoteproto.DeviceID(owner.DeviceID)
	scopes, err := q.EffectiveControlScopes(ctx, store.ControlDevice{ID: caller, Admin: owner.Admin}, computer.ID)
	if err != nil {
		return "", err
	}
	if !slices.Contains(scopes, remoteproto.ScopeAdmin) {
		return "", errAdminRequired
	}
	if err := s.verifyRemoteDeviceMutation(ctx, q, current, owner, mutation); err != nil {
		return "", err
	}
	return caller, nil
}

func (s *Service) verifyRemoteDeviceMutation(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, owner store.DeviceKeyOwner,
	mutation remoteproto.RemoteDeviceMutation,
) error {
	method := http.MethodDelete
	if mutation.Scopes != nil {
		method = http.MethodPatch
	}
	path := remoteproto.RemoteDevicesPath + "/" + url.PathEscape(string(mutation.DeviceID))
	binding := remoteproto.DPoPBinding{
		Method: method, URL: "https://" + remoteproto.TunnelHost + path,
		JKT: mutation.CallerJKT, CredentialHash: mutation.CredentialHash, Audience: computer.ID,
	}
	if owner.Kind == store.BrowserDevice {
		tier, err := s.reconcilePlan(ctx, q, computer.AccountID, s.config.Now())
		if err != nil {
			return err
		}
		if err := admitGrant(computer, tier, remoteproto.RouteCloud); err != nil {
			return err
		}
		tunnel, err := q.LockHostedTunnel(ctx, computer.ID)
		if errors.Is(err, store.ErrNotFound) {
			return ErrUnauthorized
		}
		if err != nil {
			return err
		}
		if mutation.CredentialHash != "" || tunnel.State != remoteproto.HostedTunnelActive ||
			tunnel.TakenDown || tunnel.Hostname != owner.Origin {
			return ErrUnauthorized
		}
		binding.URL = "https://" + tunnel.Hostname + path
		binding.Audience = ""
	} else if mutation.CredentialHash == "" {
		return ErrUnauthorized
	}
	now := s.config.Now()
	proof, err := remoteproto.CheckDPoP(mutation.DPoP, binding, now)
	if err != nil {
		return ErrUnauthorized
	}
	fresh, err := q.UseComputerProof(ctx, mutation.CallerJKT, proof.JTI, now.Add(remoteproto.DPoPReplayWindow))
	if err == nil && !fresh {
		return errProofReplayed
	}
	return err
}
