package control

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	accountSuspended = "suspended"
	accountDeleting  = "deleting"
)

var errConfirmEmail = &Error{http.StatusBadRequest, "confirm_email_mismatch", "That email doesn't match this account."}

func (s *Service) DeleteAccount(
	ctx context.Context, principal DevicePrincipal, confirmEmail string,
) (remoteproto.AccountDeleteResponse, error) {
	var result remoteproto.AccountDeleteResponse
	err := s.deviceTransaction(ctx, principal, func(q *store.Queries, a store.ControlAccount, device store.ControlDevice) error {
		if !device.Admin {
			return errAdminRequired
		}
		account, err := q.LockAuthAccount(ctx, a.ID)
		if err != nil {
			return err
		}
		if account.Email != strings.ToLower(strings.TrimSpace(confirmEmail)) {
			return errConfirmEmail
		}
		result, err = s.startAccountDeletion(ctx, q, account.ID, auditActor(device.ID))
		return err
	})
	return result, err
}

func (s *Service) StartAccountDeletion(ctx context.Context, accountID, operatorEmail string) error {
	actor, err := adminActor(operatorEmail)
	if err != nil {
		return err
	}
	return s.accountTransaction(ctx, accountID, func(q *store.Queries, account store.ControlAccount) error {
		if account.State == accountDeleting {
			return nil
		}
		_, err := s.startAccountDeletion(ctx, q, account.ID, actor)
		return err
	})
}

func (s *Service) startAccountDeletion(
	ctx context.Context, q *store.Queries, accountID, actor string,
) (remoteproto.AccountDeleteResponse, error) {
	now := s.config.Now()
	subs, err := q.Subscriptions(ctx, accountID)
	if err != nil {
		return remoteproto.AccountDeleteResponse{}, err
	}
	result := remoteproto.AccountDeleteResponse{State: accountDeleting, Refund: "none"}
	for _, sub := range subs {
		if sub.Entitled(s.config.MonthlyPrice, s.config.AnnualPrice, now) {
			result.SubscriptionCancelled = true
			if now.Before(sub.RefundEligibleUntil()) {
				result.Refund = "full"
			}
		}
	}
	if err := q.SetLifecycleState(ctx, accountID, accountDeleting, time.Time{}); err != nil {
		return result, err
	}
	if err := q.RevokeAccountCredentials(ctx, accountID, now); err != nil {
		return result, err
	}
	if err := q.BeginAccountDeletion(ctx, accountID, now); err != nil {
		return result, err
	}
	if err := q.AppendSecurityEvent(ctx, store.SecurityEvent{
		AccountID: accountID, Type: "account_deletion_started", Actor: actor, At: now,
	}); err != nil {
		return result, err
	}
	return result, advanceLifecycleEpoch(ctx, q, accountID)
}

func (s *Service) SuspendAccount(ctx context.Context, accountID, operatorEmail string) error {
	return s.setAccountSuspension(ctx, accountID, operatorEmail, true)
}

func (s *Service) UnsuspendAccount(ctx context.Context, accountID, operatorEmail string) error {
	return s.setAccountSuspension(ctx, accountID, operatorEmail, false)
}

func (s *Service) setAccountSuspension(ctx context.Context, accountID, operatorEmail string, suspend bool) error {
	actor, err := adminActor(operatorEmail)
	if err != nil {
		return err
	}
	return s.accountTransaction(ctx, accountID, func(q *store.Queries, account store.ControlAccount) error {
		if account.State == accountDeleting {
			return errAccountDeleting
		}
		state := accountActive
		var suspendedAt time.Time
		now := s.config.Now()
		if suspend {
			state, suspendedAt = accountSuspended, now
		}
		if account.State == state {
			return nil
		}
		if err := q.SetLifecycleState(ctx, account.ID, state, suspendedAt); err != nil {
			return err
		}
		event := store.SecurityEvent{AccountID: account.ID, Actor: actor, At: now, Type: "account_unsuspended"}
		if err := applySuspension(ctx, q, event, suspend); err != nil {
			return err
		}
		return advanceLifecycleEpoch(ctx, q, account.ID)
	})
}

func applySuspension(ctx context.Context, q *store.Queries, event store.SecurityEvent, suspend bool) error {
	if !suspend {
		return q.AppendSecurityEvent(ctx, event)
	}
	if err := q.RevokeAccountCredentials(ctx, event.AccountID, event.At); err != nil {
		return err
	}
	if err := q.MarkAccountTunnelsDeleting(ctx, event.AccountID); err != nil {
		return err
	}
	event.Type = store.EventAccountSuspended
	return q.QueueSecurityEmail(ctx, event)
}

func adminActor(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.ContainsAny(email, "\r\n") {
		return "", ErrInvalidInput
	}
	return "admin:" + email, nil
}

func advanceLifecycleEpoch(ctx context.Context, q *store.Queries, id string) error {
	_, err := q.AdvanceSecurity(ctx, store.ControlEvent{Type: remoteproto.ControlSnapshot, AccountID: id})
	return err
}

func (s *Service) unlinkDeletingComputers(ctx context.Context, q *store.Queries, id string) error {
	if err := q.LockPlanAccount(ctx, id); err != nil {
		return err
	}
	computers, err := q.ListControlComputers(ctx, id)
	if err != nil {
		return err
	}
	now := s.config.Now()
	if err := q.ApplyAccountPlan(ctx, id, plan.Free, now, true); err != nil {
		return err
	}
	for _, computer := range computers {
		if err := q.UnlinkControlComputer(ctx, computer, now); err != nil {
			return err
		}
	}
	if err := q.MarkAccountTunnelsDeleting(ctx, id); err != nil {
		return err
	}
	if len(computers) == 0 {
		return nil
	}
	_, err = q.AdvanceSecurity(ctx, store.ControlEvent{Type: remoteproto.ControlUnlink, AccountID: id})
	return err
}

func (s *Service) deleteAccountTunnels(ctx context.Context, q *store.Queries, id string) error {
	refs, err := q.DeletingAccountTunnels(ctx, id)
	if err != nil || len(refs) == 0 {
		return err
	}
	if s.config.HostedTunnels == nil {
		return errors.New("Cloudflare is required to remove the deleting account's tunnels")
	}
	for _, ref := range refs {
		if err := s.DeleteHostedGeneration(ctx, ref); err != nil {
			return err
		}
	}
	return nil
}
