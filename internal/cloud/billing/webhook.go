// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

const (
	stripeSignatureAge  = 5 * time.Minute
	checkoutCompleted   = "checkout.session.completed"
	checkoutExpired     = "checkout.session.expired"
	subscriptionUpdated = "customer.subscription.updated"
	subscriptionDeleted = "customer.subscription.deleted"
)

var (
	errStripeSignature      = &Failure{http.StatusBadRequest, "stripe_signature_invalid", "Invalid Stripe signature."}
	errUnrelatedStripeEvent = errors.New("Stripe event is outside Compozy billing")
)

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID           string `json:"id"`
			Customer     string `json:"customer"`
			Subscription string `json:"subscription"`
			Parent       struct {
				SubscriptionDetails struct {
					Subscription string `json:"subscription"`
				} `json:"subscription_details"`
			} `json:"parent"`
		} `json:"object"`
	} `json:"data"`
}

func (s *Service) Webhook(ctx context.Context, signature string, raw []byte) error {
	if s.cfg.Stripe == nil {
		return ErrUnavailable
	}
	now := s.cfg.Now()
	if !verifyStripeSignature(signature, raw, s.cfg.Stripe.config.WebhookSecret, now) {
		return errStripeSignature
	}
	var event stripeEvent
	if err := json.Unmarshal(raw, &event); err != nil || !validStripeID(event.ID, "evt_") {
		return ErrInvalidRequest
	}
	return s.applyStripeEvent(ctx, event)
}

func (s *Service) applyStripeEvent(ctx context.Context, event stripeEvent) error {
	object := event.Data.Object
	var subscription string
	switch event.Type {
	case "customer.subscription.created", subscriptionUpdated, subscriptionDeleted:
		subscription = object.ID
	case "invoice.paid", "invoice.payment_failed":
		subscription = object.Parent.SubscriptionDetails.Subscription
		if subscription == "" {
			subscription = object.Subscription
		}
	case checkoutCompleted:
		subscription = object.Subscription
	case checkoutExpired:
		return s.refreshSubscription(ctx, "", event, "")
	default:
		return nil
	}
	if subscription == "" {
		return nil
	}
	return s.refreshSubscription(ctx, subscription, event, "")
}

func verifyStripeSignature(header string, raw []byte, secret string, now time.Time) bool {
	var timestamp string
	var signatures []string
	for item := range strings.SplitSeq(header, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(item), "=")
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	delta := now.Sub(time.Unix(seconds, 0))
	if err != nil || delta > stripeSignatureAge || delta < -stripeSignatureAge {
		return false
	}
	digest := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(digest, "%s.%s", timestamp, raw) // hash.Hash.Write never returns an error.
	expected := digest.Sum(nil)
	for _, signature := range signatures {
		signature, err := hex.DecodeString(signature)
		if err == nil && hmac.Equal(signature, expected) {
			return true
		}
	}
	return false
}

func (s *Service) refreshSubscription(ctx context.Context, id string, event stripeEvent, expectedAccount string) error {
	return s.applySubscriptionRefresh(ctx, id, event, expectedAccount, time.Time{})
}

func (s *Service) applySubscriptionRefresh(
	ctx context.Context, id string, event stripeEvent, expectedAccount string, cutoff time.Time,
) error {
	if id != "" && !validStripeID(id, "sub_") {
		return ErrInvalidRequest
	}
	receivedAt := s.cfg.Now()
	var ended, interval string
	err := s.db.Transaction(ctx, func(q *store.Queries) error {
		if id != "" {
			if err := q.LockStripeSubscription(ctx, id); err != nil {
				return err
			}
		}
		if !cutoff.IsZero() {
			stale, err := q.BillingStripeSubscriptionStale(ctx, id, cutoff)
			if err != nil || !stale {
				return err
			}
		}
		if event.ID != "" {
			added, err := q.InsertStripeEvent(ctx, event.ID, event.Type, receivedAt)
			if err != nil || !added {
				return err
			}
		}
		if id != "" {
			if err := s.fetchAndSaveSubscription(ctx, q, id, expectedAccount, event, &ended); err != nil {
				return err
			}
		}
		var err error
		interval, err = s.finishStripeEvent(ctx, q, event)
		return err
	})
	// Ignored events roll back their tentative receipt along with the transaction.
	if errors.Is(err, errUnrelatedStripeEvent) {
		if expectedAccount != "" {
			return ErrInvalidRequest
		}
		return nil
	}
	if err == nil {
		if ended != "" {
			s.cfg.Metrics.Count("cloud_subscription_ends_total", observe.Labels{"reason": ended})
		}
		if interval != "" {
			s.countCheckout(interval, strings.TrimPrefix(event.Type, "checkout.session."))
		}
	}
	return err
}

func (s *Service) finishStripeEvent(ctx context.Context, q *store.Queries, event stripeEvent) (string, error) {
	if event.ID == "" {
		return "", nil
	}
	var interval string
	switch event.Type {
	case checkoutCompleted, checkoutExpired:
		var err error
		interval, err = q.BillingCheckoutInterval(ctx, event.Data.Object.ID)
		if err != nil {
			return "", err
		}
		if interval == "" && event.Type == checkoutExpired {
			return "", errUnrelatedStripeEvent
		}
	}
	return interval, q.MarkStripeEventProcessed(ctx, event.ID, s.cfg.Now())
}

func (s *Service) countCheckout(interval, result string) {
	s.cfg.Metrics.Count("cloud_checkout_sessions_total", observe.Labels{"interval": interval, "result": result})
}

func (s *Service) fetchAndSaveSubscription(
	ctx context.Context,
	q *store.Queries,
	id, expectedAccount string,
	event stripeEvent,
	ended *string,
) error {
	old, err := q.BillingSubscription(ctx, id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if old.Source == "complimentary" {
		return errUnrelatedStripeEvent
	}
	fetch, cancel := context.WithTimeout(ctx, stripeTimeout)
	defer cancel()
	allowRetirement := expectedAccount == "" &&
		(event.ID == "" || event.Type == subscriptionUpdated || event.Type == subscriptionDeleted)
	current, hasInvoices, err := s.fetchOwnedSubscription(fetch, id, old, event, allowRetirement)
	if err != nil {
		return err
	}
	if err := bindSubscriptionAccount(&current, old.AccountID, expectedAccount); err != nil {
		return err
	}
	if current.AccountID == "" {
		return errUnrelatedStripeEvent
	}
	if hasInvoices && current.PriceID != "" {
		if err := s.cfg.Stripe.readLastPayment(fetch, &current, s.cfg.AnnualPrice); err != nil {
			return err
		}
	}
	if err := q.LockPlanAccount(ctx, current.AccountID); err != nil {
		if errors.Is(err, store.ErrNotFound) && expectedAccount == "" {
			return errUnrelatedStripeEvent
		}
		return err
	}
	return s.saveSubscriptionPlan(ctx, q, old, current, ended)
}

func (s *Service) fetchOwnedSubscription(
	ctx context.Context, id string, old plan.Subscription, event stripeEvent, allowRetirement bool,
) (plan.Subscription, bool, error) {
	current, hasInvoices, err := s.cfg.Stripe.subscription(ctx, id, s.cfg.MonthlyPrice, s.cfg.AnnualPrice)
	if err != nil {
		if f, ok := errors.AsType[*stripeFailure](err); ok && f.Status == http.StatusNotFound &&
			old.ID == "" && event.ID != "" {
			err = errUnrelatedStripeEvent
		}
		return plan.Subscription{}, false, err
	}
	// A lifecycle update can remove an entitlement already bound to this subscription and customer.
	if current.PriceID == "" && (!allowRetirement || old.PriceID == "" || old.CustomerID == "") {
		return plan.Subscription{}, false, errUnrelatedStripeEvent
	}
	if current.CustomerID == "" || (old.CustomerID != "" && current.CustomerID != old.CustomerID) ||
		(event.Data.Object.Customer != "" && current.CustomerID != event.Data.Object.Customer) {
		return plan.Subscription{}, false, errUnrelatedStripeEvent
	}
	return current, hasInvoices, nil
}

func (s *Service) saveSubscriptionPlan(
	ctx context.Context, q *store.Queries, old, current plan.Subscription, ended *string,
) error {
	now := s.cfg.Now()
	previous, err := q.Subscriptions(ctx, current.AccountID)
	if err != nil {
		return err
	}
	if err := q.SaveStripeSubscription(ctx, current, now); err != nil {
		return err
	}
	subscriptions, err := q.Subscriptions(ctx, current.AccountID)
	if err != nil {
		return err
	}
	before, after := s.status(previous, now), s.status(subscriptions, now)
	if current.Status == stripeStatusCanceled && old.Status != stripeStatusCanceled && current.PriceID != "" {
		*ended = current.CancellationReason
		if *ended == "" {
			*ended = "unknown"
		}
	}
	if before.Plan == plan.Pro && after.Plan == plan.Free {
		if err := q.StartProLapse(ctx, current.AccountID, now); err != nil {
			return err
		}
	}
	return q.ApplyAccountPlan(ctx, current.AccountID, after.Plan, now, before != after)
}

func bindSubscriptionAccount(current *plan.Subscription, previous, expected string) error {
	if previous != "" {
		if current.AccountID != "" && current.AccountID != previous {
			return errors.New("Stripe subscription account changed")
		}
		current.AccountID = previous
	}
	if expected != "" && current.AccountID != expected {
		return ErrInvalidRequest
	}
	return nil
}
