// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

func (s *Service) Checkout(ctx context.Context, accountID string, target Target, authorize Authorize) (string, error) {
	if !s.validTarget(target) {
		return "", ErrInvalidRequest
	}
	price := s.cfg.AnnualPrice
	if target.Interval == "month" {
		price = s.cfg.MonthlyPrice
	}
	if s.cfg.Stripe == nil || price == "" {
		return "", ErrUnavailable
	}
	for {
		var checkout stripeCheckout
		var unrecordedSubscription string
		err := s.accountTransaction(ctx, accountID, authorize, func(q *store.Queries, account store.AuthAccount) error {
			if _, err := s.ownedHostedComputer(ctx, q, accountID, target.Hostname); err != nil {
				return err
			}
			subscriptions, err := q.Subscriptions(ctx, accountID)
			if err != nil {
				return err
			}
			if s.status(subscriptions, s.cfg.Now()).Plan == plan.Pro {
				return ErrAlreadyPro
			}
			unrecordedSubscription, err = s.closePreviousCheckouts(ctx, q, accountID)
			if err != nil || unrecordedSubscription != "" {
				return err
			}
			checkout, err = s.cfg.Stripe.createCheckout(ctx, stripeCheckoutRequest{
				AccountID: accountID, Email: account.Email, CustomerID: billingCustomer(subscriptions), PriceID: price,
				SuccessURL: s.cfg.Issuer + SuccessPath + "?session_id={CHECKOUT_SESSION_ID}",
				CancelURL:  s.cfg.Issuer + target.URL() + "&checkout=" + CheckoutCancelled,
				Key:        "checkout_" + rand.Text(),
			})
			if err != nil {
				return err
			}
			return q.InsertBillingCheckout(ctx, store.BillingCheckout{
				ID: checkout.ID, AccountID: accountID, Interval: target.Interval, Hostname: target.Hostname,
			})
		})
		if err != nil {
			return "", err
		}
		if unrecordedSubscription == "" {
			s.countCheckout(target.Interval, "created")
			return checkout.URL, nil
		}
		// Subscription refresh takes the subscription lock before the account lock.
		if err := s.refreshSubscription(ctx, unrecordedSubscription, stripeEvent{}, accountID); err != nil {
			return "", err
		}
	}
}

func (s *Service) closePreviousCheckouts(ctx context.Context, q *store.Queries, accountID string) (string, error) {
	checkouts, err := q.BillingCheckouts(ctx, accountID, false)
	if err != nil {
		return "", err
	}
	var unrecordedSubscription string
	for _, checkout := range checkouts {
		current, err := s.cfg.Stripe.closeCheckout(ctx, checkout.ID)
		if err != nil {
			return "", err
		}
		switch current.Status {
		case stripeCheckoutExpired:
			if err := q.MarkBillingCheckoutExpired(ctx, checkout.ID, s.cfg.Now()); err != nil {
				return "", err
			}
		case stripeCheckoutComplete:
			if current.ClientReference != accountID || current.Subscription == "" {
				return "", ErrUnavailable
			}
			sub, err := q.BillingSubscription(ctx, current.Subscription)
			if errors.Is(err, store.ErrNotFound) {
				unrecordedSubscription = cmp.Or(unrecordedSubscription, current.Subscription)
				continue
			}
			if err != nil {
				return "", err
			}
			if sub.AccountID != accountID ||
				(sub.Status != stripeStatusCanceled && sub.Status != "incomplete_expired") {
				return "", ErrUnavailable
			}
		default:
			return "", ErrUnavailable
		}
	}
	return unrecordedSubscription, nil
}
