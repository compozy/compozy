// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/cloud/email"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Service) PatchAccountEmail(
	ctx context.Context, principal DevicePrincipal, patch remoteproto.AccountEmailPatch,
) (remoteproto.AccountSummary, error) {
	if patch.MarketingOptIn == nil || (patch.Source != "settings" && patch.Source != "cli" && patch.Source != "sign_in") ||
		(patch.Source == "sign_in" && !*patch.MarketingOptIn) {
		return remoteproto.AccountSummary{}, ErrInvalidInput
	}
	var result remoteproto.AccountSummary
	err := s.deviceTransaction(ctx, principal, func(q *store.Queries, a store.ControlAccount, device store.ControlDevice) error {
		account, err := q.LockAuthAccount(ctx, a.ID)
		if err != nil {
			return err
		}
		if err := s.recordConsent(ctx, q, account, *patch.MarketingOptIn, patch.Source); err != nil {
			return err
		}
		account.MarketingOptIn = *patch.MarketingOptIn
		result, err = s.accountSummary(ctx, q, account, device, principal)
		return err
	})
	return result, err
}

func (s *Service) recordConsent(ctx context.Context, q *store.Queries, account store.AuthAccount, optIn bool, source string) error {
	if err := q.UpdateMarketingConsent(ctx, account, optIn, source, s.config.Now()); err != nil {
		return err
	}
	return q.NotifyControlEvent(ctx, store.ControlEvent{Type: remoteproto.ControlConsentChanged, AccountID: account.ID})
}

func (s *Service) Unsubscribe(ctx context.Context, id string) error {
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, id)
		if errors.Is(err, store.ErrAuthNotFound) {
			return nil
		}
		if err != nil || !account.MarketingOptIn || account.State == accountDeleting {
			return err
		}
		return s.recordConsent(ctx, q, account, false, "unsubscribe")
	})
}

func (s *Service) SweepAccountEmail(ctx context.Context) error {
	if s.config.Email == nil {
		return nil
	}
	securityErr := s.deliverSecurityEmails(ctx)
	if !s.config.Email.SyncsContacts() || s.config.Unsubscribe == nil {
		return securityErr
	}
	return errors.Join(securityErr, s.syncMarketingContacts(ctx))
}

func (s *Service) syncMarketingContacts(ctx context.Context) error {
	now := s.config.Now()
	ids, err := s.db.DirtyMarketingAccounts(ctx, now)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		work, cancel := context.WithTimeout(ctx, deletionTimeout)
		err := s.syncMarketingAccount(work, id, now)
		cancel()
		if err != nil {
			failures = append(failures, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *Service) syncMarketingAccount(ctx context.Context, id string, now time.Time) error {
	var syncErr error
	err := s.db.Transaction(ctx, func(q *store.Queries) error {
		a, err := q.ClaimMarketingAccount(ctx, id, now)
		if errors.Is(err, store.ErrAuthNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		syncErr = s.syncClaimedMarketingAccount(ctx, q, a)
		if syncErr != nil {
			// Keep the retry with the consent revision locked by the claim.
			return q.RetryMarketingAccount(ctx, id, s.config.Now().Add(lifecycleRetry))
		}
		return q.RecordMarketingSync(ctx, id, s.config.Now())
	})
	return errors.Join(syncErr, err)
}

func (s *Service) syncClaimedMarketingAccount(ctx context.Context, q *store.Queries, a store.AuthAccount) error {
	known, err := q.MarketingAddresses(ctx, a.ID)
	if err != nil {
		return err
	}
	if a.MarketingOptIn {
		known = append(known, a.Email)
	}
	slices.Sort(known)
	for _, address := range slices.Compact(known) {
		if err := q.LockMarketingAddress(ctx, address); err != nil {
			return err
		}
		if a.MarketingOptIn && address == a.Email {
			if err := s.config.Email.UpsertContact(ctx, a.Email, a.Name, s.config.Unsubscribe.URL(a.ID)); err != nil {
				return err
			}
			continue
		}
		if err := s.removeMarketingContact(ctx, q, a.ID, address); err != nil {
			return err
		}
		if err := q.ForgetMarketingAddress(ctx, a.ID, address); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) removeMarketingContact(ctx context.Context, q *store.Queries, accountID, address string) error {
	owned, err := q.OtherMarketingContactOwner(ctx, accountID, address)
	if err != nil || owned {
		return err
	}
	return s.config.Email.DeleteContact(ctx, address)
}

func (s *Service) deliverSecurityEmails(ctx context.Context) error {
	now := s.config.Now()
	entries, err := s.db.DueAccountEmails(ctx, now)
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		work, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := s.deliverSecurityEmail(work, entry, now)
		cancel()
		if err != nil {
			retryErr := s.db.RetryAccountEmail(ctx, entry.ID, now, s.config.Now().Add(lifecycleRetry))
			failures = append(failures, errors.Join(err, retryErr))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *Service) deliverSecurityEmail(ctx context.Context, entry store.AccountEmailEntry, now time.Time) error {
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, entry.AccountID)
		if errors.Is(err, store.ErrAuthNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		event, err := q.LockAccountEmail(ctx, entry.ID, now)
		if errors.Is(err, store.ErrAuthNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if account.State != accountDeleting {
			key := event.Type + "/" + strconv.FormatInt(entry.ID, 10)
			msg := s.securityEmail(account.Email, entry.ID, event)
			if err := s.config.Email.Send(ctx, key, msg); err != nil {
				return err
			}
		}
		return q.RemoveAccountEmail(ctx, entry.ID)
	})
}

func (s *Service) securityEmail(address string, id int64, event store.AccountEmail) email.Message {
	if event.Type == store.EventAccountSuspended {
		return email.Message{To: address, Subject: "Your Compozy account was suspended",
			Text: "This account is suspended. Contact support@compozy.com."}
	}
	name, location := event.Notice.Name, cmp.Or(event.Notice.Location, "Unknown location")
	link := s.config.Issuer + "/account/sign-ins/" + strconv.FormatInt(id, 10)
	return email.Message{
		To:      address,
		Subject: "New sign-in to Compozy on " + name + " (" + location + ")",
		Text:    "A device signed in to your Compozy account. If this wasn’t you, sign in again to revoke this device:\n" + link,
	}
}
