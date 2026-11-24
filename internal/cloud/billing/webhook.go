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

	"github.com/compozy/compozy/internal/cloud/store"
)

const stripeSignatureAge = 5 * time.Minute

var errStripeSignature = &Failure{http.StatusBadRequest, "stripe_signature_invalid", "Invalid Stripe signature."}

type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID           string `json:"id"`
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
	object := event.Data.Object
	var subscription string
	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		subscription = object.ID
	case "invoice.paid", "invoice.payment_failed":
		subscription = object.Parent.SubscriptionDetails.Subscription
		if subscription == "" {
			subscription = object.Subscription
		}
	case "checkout.session.completed":
		subscription = object.Subscription
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
	if !validStripeID(id, "sub_") {
		return ErrInvalidRequest
	}
	receivedAt := s.cfg.Now()
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		if err := q.LockStripeSubscription(ctx, id); err != nil {
			return err
		}
		if event.ID != "" {
			added, err := q.InsertStripeEvent(ctx, event.ID, event.Type, receivedAt)
			if err != nil || !added {
				return err
			}
		}
		if err := s.fetchAndSaveSubscription(ctx, q, id, expectedAccount); err != nil {
			return err
		}
		if event.ID != "" {
			return q.MarkStripeEventProcessed(ctx, event.ID, s.cfg.Now())
		}
		return nil
	})
}

func (s *Service) fetchAndSaveSubscription(ctx context.Context, q *store.Queries, id, expectedAccount string) error {
	old, err := q.BillingSubscription(ctx, id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if old.Source == "complimentary" {
		return nil
	}
	fetch, cancel := context.WithTimeout(ctx, stripeTimeout)
	defer cancel()
	current, err := s.cfg.Stripe.subscription(fetch, id, s.cfg.MonthlyPrice, s.cfg.AnnualPrice)
	if err != nil {
		return err
	}
	if old.AccountID != "" {
		if current.AccountID != "" && current.AccountID != old.AccountID {
			return errors.New("Stripe subscription account changed")
		}
		current.AccountID = old.AccountID
	}
	if expectedAccount != "" && current.AccountID != expectedAccount {
		return ErrInvalidRequest
	}
	if current.AccountID == "" {
		return nil
	}
	if err := q.LockPlanAccount(ctx, current.AccountID); err != nil {
		if errors.Is(err, store.ErrNotFound) && expectedAccount == "" {
			return nil
		}
		return err
	}
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
	return q.ApplyAccountPlan(ctx, current.AccountID, after.Plan, now, before != after)
}
