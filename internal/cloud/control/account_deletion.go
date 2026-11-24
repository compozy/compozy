// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/cloud/email"
	"github.com/compozy/compozy/internal/cloud/store"
)

const (
	deletionBilling int16 = iota
	deletionUnlink
	deletionTunnels
	deletionIdentities
	deletionContact
	deletionStripeCustomer
	deletionNotify
	deletionFinalize

	lifecycleRetry  = time.Minute
	deletionTimeout = 45 * time.Second
)

func (s *Service) SweepAccountDeletions(ctx context.Context) error {
	ids, err := s.db.DueAccountDeletions(ctx, s.config.Now())
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err := s.runAccountDeletion(ctx, id); err != nil {
			failures = append(failures, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *Service) runAccountDeletion(ctx context.Context, id string) error {
	for {
		now := s.config.Now()
		advanced, err := s.advanceAccountDeletion(ctx, id, now)
		if err != nil || !advanced {
			return err
		}
	}
}

func (s *Service) advanceAccountDeletion(ctx context.Context, id string, now time.Time) (bool, error) {
	lease := now.Add(2 * lifecycleRetry)
	step, claimed, err := s.db.ClaimAccountDeletion(ctx, id, now, lease)
	if err != nil || !claimed {
		return false, err
	}
	work, cancel := context.WithTimeout(ctx, deletionTimeout)
	defer cancel()
	if err = s.runExternalDeletionStep(work, id, step); err == nil {
		err = s.commitDeletionStep(work, id, step, lease)
	}
	if err != nil {
		retryErr := s.db.RetryAccountDeletion(ctx, id, lease, s.config.Now().Add(lifecycleRetry))
		return false, errors.Join(fmt.Errorf("account deletion step %d: %w", step, err), retryErr)
	}
	return step != deletionFinalize, nil
}

func (s *Service) runExternalDeletionStep(ctx context.Context, id string, step int16) error {
	switch step {
	case deletionBilling:
		return s.config.Billing.CancelForDeletion(ctx, id)
	case deletionTunnels:
		return s.deleteAccountTunnels(ctx, s.db.Queries, id)
	case deletionContact:
		return s.removeAccountContacts(ctx, id)
	case deletionStripeCustomer:
		return s.config.Billing.RedactDeletingCustomer(ctx, id)
	case deletionNotify:
		return s.sendDeletionNotice(ctx, s.db.Queries, id)
	default:
		return nil
	}
}

func (s *Service) commitDeletionStep(ctx context.Context, id string, step int16, lease time.Time) error {
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		if err := q.LockClaimedAccountDeletion(ctx, id, lease); err != nil {
			return err
		}
		switch step {
		case deletionUnlink:
			if err := s.unlinkDeletingComputers(ctx, q, id); err != nil {
				return err
			}
		case deletionIdentities:
			if err := q.RemoveDeletingAccountIdentities(ctx, id); err != nil {
				return err
			}
		case deletionFinalize:
			if _, err := q.LockAuthAccount(ctx, id); err != nil {
				return err
			}
			return q.FinishAccountDeletion(ctx, id, s.config.Now())
		}
		return q.AdvanceAccountDeletion(ctx, id, s.config.Now())
	})
}

func (s *Service) removeAccountContacts(ctx context.Context, id string) error {
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, id)
		if err != nil {
			return err
		}
		known, err := q.MarketingAddresses(ctx, id)
		if err != nil {
			return err
		}
		if !s.config.Email.SyncsContacts() {
			if len(known) != 0 {
				return errors.New("removing the deleting account's addresses requires Resend contacts")
			}
			return nil
		}
		known = append(known, account.Email)
		slices.Sort(known)
		for _, address := range slices.Compact(known) {
			if err := q.LockMarketingAddress(ctx, address); err != nil {
				return err
			}
			if err := s.removeMarketingContact(ctx, q, id, address); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) sendDeletionNotice(ctx context.Context, q *store.Queries, id string) error {
	if s.config.Email == nil {
		return nil
	}
	account, err := q.LockAuthAccount(ctx, id)
	if err != nil {
		return err
	}
	return s.config.Email.Send(ctx, "account-deleted/"+id, email.Message{
		To: account.Email, Subject: "Your Compozy account was deleted",
		Text: "Your Compozy Cloud account was deleted. Your local sessions and projects are still on your computers.",
	})
}
