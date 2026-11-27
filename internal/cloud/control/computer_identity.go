// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"cmp"
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Service) LinkedAccountEmail(ctx context.Context, id string) (string, error) {
	account, err := s.db.LockAuthAccount(ctx, id)
	return account.Email, err
}

func (s *Service) RevalidateComputer(
	ctx context.Context, computer store.ControlComputer, proof ComputerRequestAuth, installationID string,
) error {
	if err := s.consumeComputerProof(ctx, computer, proof); err != nil {
		return err
	}
	if installationID != "" && len(installationID) > 128 {
		return ErrInvalidInput
	}
	err := s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		switch account.State {
		case "suspended":
			return errAccountSuspended
		case "deleting":
			return errRevalidationAccountDeleted
		}
		row, err := q.ControlComputer(ctx, computer.ID)
		if errors.Is(err, store.ErrNotFound) || !row.UnlinkedAt.IsZero() {
			return errRevalidationUnlinked
		}
		if err != nil {
			return err
		}
		now := s.config.Now()
		if err := q.AdvanceAccountActivity(ctx, account.ID, now); err != nil {
			return err
		}
		return q.RecordInstallationActivity(ctx, account.ID, cmp.Or(row.InstallationID, installationID), now)
	})
	if errors.Is(err, ErrUnauthorized) {
		return errRevalidationUnlinked
	}
	return err
}

var (
	errRevalidationAccountDeleted = &Error{http.StatusUnauthorized, "account_deleted", "Your account was deleted."}
	errRevalidationUnlinked       = &Error{http.StatusUnauthorized, "computer_unlinked",
		"This computer was unlinked from your account. Sign in to keep using it."}
)

func (s *Service) UnlinkComputerWithKey(
	ctx context.Context, identity store.ControlComputer, proof ComputerRequestAuth,
) error {
	if err := s.consumeComputerProof(ctx, identity, proof); err != nil {
		return err
	}
	var tunnel store.TunnelRef
	err := s.accountTransaction(ctx, identity.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		computer, err := q.ControlComputer(ctx, identity.ID)
		if err != nil || !computer.UnlinkedAt.IsZero() {
			return err
		}
		tunnel, err = s.unlinkComputer(ctx, q, computer, "")
		return err
	})
	if err == nil {
		s.finishHostedDelete(ctx, tunnel)
	}
	return err
}

func (s *Service) unlinkComputer(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, actor remoteproto.DeviceID,
) (store.TunnelRef, error) {
	now := s.config.Now()
	if err := q.UnlinkControlComputer(ctx, computer, now); err != nil {
		return store.TunnelRef{}, err
	}
	tunnel, err := s.markHostedDeleting(ctx, q, computer)
	if err != nil {
		return store.TunnelRef{}, err
	}
	if _, err := s.reconcilePlan(ctx, q, computer.AccountID, now); err != nil {
		return store.TunnelRef{}, err
	}
	if err := s.securityEvent(ctx, q, computer.AccountID, actor, "computer_unlinked",
		map[string]any{auditComputerID: computer.ID}); err != nil {
		return store.TunnelRef{}, err
	}
	_, err = q.AdvanceSecurity(ctx, store.ControlEvent{
		Type: remoteproto.ControlUnlink, AccountID: computer.AccountID, ComputerID: computer.ID,
	})
	return tunnel, err
}
