package billing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

// CancelForDeletion runs after the account enters deleting and before its tombstone.
func (s *Service) CancelForDeletion(ctx context.Context, accountID string) error {
	if err := s.ExpireCheckouts(ctx, accountID); err != nil {
		return err
	}
	subscriptions, err := s.db.Subscriptions(ctx, accountID)
	if err != nil {
		return err
	}
	for _, sub := range subscriptions {
		if sub.Source == stripeSource && sub.Entitled(s.cfg.MonthlyPrice, s.cfg.AnnualPrice, s.cfg.Now()) {
			if err := s.cancelForDeletion(ctx, sub); err != nil {
				return err
			}
		}
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		if err := q.LockPlanAccount(ctx, accountID); err != nil {
			return err
		}
		now := s.cfg.Now()
		return q.EndComplimentarySubscriptions(ctx, accountID, now)
	})
}

func (s *Service) ExpireCheckouts(ctx context.Context, accountID string) error {
	checkouts, err := s.db.BillingCheckouts(ctx, accountID)
	if err != nil {
		return err
	}
	for _, checkout := range checkouts {
		if err := s.finishDeletingCheckout(ctx, checkout); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) finishDeletingCheckout(ctx context.Context, checkout store.BillingCheckout) error {
	if s.cfg.Stripe == nil {
		return ErrUnavailable
	}
	current, err := s.cfg.Stripe.checkout(ctx, checkout.ID)
	if err != nil {
		return err
	}
	if current.Status == "open" {
		err := s.cfg.Stripe.request(ctx, http.MethodPost,
			"/v1/checkout/sessions/"+url.PathEscape(checkout.ID)+"/expire", nil, "delete_"+checkout.ID, nil)
		if err == nil {
			return nil
		}
		// A payment can complete between reading an open Checkout and attempting expiry.
		current, err = s.cfg.Stripe.checkout(ctx, checkout.ID)
		if err != nil {
			return err
		}
		if current.Status == "open" {
			return ErrUnavailable
		}
	}
	if current.Status == "complete" && current.Subscription != "" {
		return s.refreshSubscription(ctx, current.Subscription, stripeEvent{}, checkout.AccountID)
	}
	return nil
}

func (s *Service) cancelForDeletion(ctx context.Context, sub plan.Subscription) error {
	if s.cfg.Stripe == nil {
		return ErrUnavailable
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		if err := q.LockStripeSubscription(ctx, sub.ID); err != nil {
			return err
		}
		if err := q.LockPlanAccount(ctx, sub.AccountID); err != nil {
			return err
		}
		current, err := q.BillingSubscription(ctx, sub.ID)
		now := s.cfg.Now()
		if err != nil || !current.Entitled(s.cfg.MonthlyPrice, s.cfg.AnnualPrice, now) {
			return err
		}
		if until := current.RefundEligibleUntil(); now.Before(until) {
			if err := s.cfg.Stripe.refundInvoice(ctx, current.LastInvoiceID, current.ID); err != nil {
				return err
			}
		}
		if err := s.cfg.Stripe.cancelSubscription(ctx, current.ID); err != nil {
			return err
		}
		current.Status, current.EndsAt = "canceled", now
		current.CancelAtPeriodEnd = false
		return q.SaveStripeSubscription(ctx, current, now)
	})
}

type stripeInvoicePayment struct {
	ID         string `json:"id"`
	AmountPaid int64  `json:"amount_paid"`
	Payment    struct {
		Type          string `json:"type"`
		PaymentIntent string `json:"payment_intent"`
		Charge        string `json:"charge"`
	} `json:"payment"`
}

func (s *Stripe) refundInvoice(ctx context.Context, invoice, subscription string) error {
	if !validStripeID(invoice, "in_") {
		return errors.New("stripe invoice for the refundable payment is missing")
	}
	params := url.Values{"invoice": {invoice}, "status": {stripeStatusPaid}, "limit": {"100"}}
	var page stripeList[stripeInvoicePayment]
	if err := s.request(ctx, http.MethodGet, "/v1/invoice_payments", params, "", &page); err != nil {
		return err
	}
	if len(page.Data) == 0 || page.HasMore {
		return errors.New("cannot identify all payments for the refundable Stripe invoice")
	}
	for _, payment := range page.Data {
		if err := s.refundPayment(ctx, subscription, payment); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stripe) refundPayment(ctx context.Context, subscription string, payment stripeInvoicePayment) error {
	params := url.Values{}
	switch payment.Payment.Type {
	case "payment_intent":
		params.Set("payment_intent", payment.Payment.PaymentIntent)
	case "charge":
		params.Set("charge", payment.Payment.Charge)
	default:
		return errors.New("Stripe invoice payment cannot be refunded through its payment method")
	}
	if !validStripeID(payment.ID, "inpay_") || payment.AmountPaid <= 0 {
		return errors.New("Stripe invoice payment has no refundable amount or identity")
	}
	params.Set("amount", strconv.FormatInt(payment.AmountPaid, 10))
	key := "delete_refund_" + subscription + "_" + payment.ID
	var refund struct {
		Status string `json:"status"`
	}
	err := s.request(ctx, http.MethodPost, "/v1/refunds", params, key, &refund)
	if stripeCode(err, "charge_already_refunded") {
		return nil
	}
	if err != nil {
		return err
	}
	if refund.Status != "succeeded" && refund.Status != "pending" {
		return errors.New("Stripe has not accepted the full refund")
	}
	return nil
}

func (s *Stripe) cancelSubscription(ctx context.Context, id string) error {
	path := "/v1/subscriptions/" + url.PathEscape(id)
	var current stripeSubscription
	err := s.request(ctx, http.MethodDelete, path, nil, "delete_cancel_"+id, &current)
	if err != nil {
		// A retry after an unacknowledged cancellation must observe Stripe's terminal state.
		if readErr := s.request(ctx, http.MethodGet, path, nil, "", &current); readErr != nil {
			return errors.Join(err, readErr)
		}
	}
	if current.ID != id || current.Status != "canceled" {
		return errors.Join(err, errors.New("Stripe has not confirmed immediate cancellation"))
	}
	return nil
}
