// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (s *Service) ReplayAfterRestore(ctx context.Context, after time.Time) error {
	if s.cfg.Stripe == nil {
		return ErrUnavailable
	}
	params := url.Values{
		"created[gte]":   {strconv.FormatInt(after.Unix(), 10)},
		"created[lte]":   {strconv.FormatInt(s.cfg.Now().Unix(), 10)},
		stripeLimitParam: {"100"},
		"types[]": {
			"customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted",
			"invoice.paid", "invoice.payment_failed", checkoutCompleted, checkoutExpired,
		},
	}
	for {
		var page stripeList[stripeEvent]
		if err := s.cfg.Stripe.request(ctx, http.MethodGet, "/v1/events", params, "", &page); err != nil {
			return err
		}
		for _, event := range page.Data {
			if !validStripeID(event.ID, "evt_") {
				return errors.New("invalid Stripe restore event")
			}
			// Replay fetches current subscription state under the same lock as a live webhook.
			if err := s.applyStripeEvent(ctx, event); err != nil {
				return err
			}
		}
		if !page.HasMore {
			return s.Reconcile(ctx)
		}
		if len(page.Data) == 0 || page.Data[len(page.Data)-1].ID == params.Get("starting_after") {
			return errors.New("Stripe event pagination did not advance")
		}
		params.Set("starting_after", page.Data[len(page.Data)-1].ID)
	}
}
