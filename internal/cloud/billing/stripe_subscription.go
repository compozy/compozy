// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
)

type stripePrice struct {
	ID        string `json:"id"`
	Recurring struct {
		Interval string `json:"interval"`
	} `json:"recurring"`
}

type stripeSubscription struct {
	ID                  string            `json:"id"`
	Status              string            `json:"status"`
	Customer            string            `json:"customer"`
	Metadata            map[string]string `json:"metadata"`
	LatestInvoice       string            `json:"latest_invoice"`
	CurrentPeriodEnd    int64             `json:"current_period_end"`
	CancelAtPeriodEnd   bool              `json:"cancel_at_period_end"`
	CancellationDetails struct {
		Reason string `json:"reason"`
	} `json:"cancellation_details"`
	EndedAt int64 `json:"ended_at"`
	Items   struct {
		Data []struct {
			Price            stripePrice `json:"price"`
			CurrentPeriodEnd int64       `json:"current_period_end"`
		} `json:"data"`
	} `json:"items"`
}

type stripeInvoice struct {
	ID            string `json:"id"`
	AmountPaid    int64  `json:"amount_paid"`
	BillingReason string `json:"billing_reason"`
	Transitions   struct {
		PaidAt int64 `json:"paid_at"`
	} `json:"status_transitions"`
	Lines struct {
		Data []struct {
			Pricing struct {
				Details struct {
					Price string `json:"price"`
				} `json:"price_details"`
			} `json:"pricing"`
		} `json:"data"`
	} `json:"lines"`
}

type stripeList[T any] struct {
	Data    []T  `json:"data"`
	HasMore bool `json:"has_more"`
}

func (s *Stripe) subscription(ctx context.Context, id, monthly, annual string) (plan.Subscription, error) {
	var result stripeSubscription
	err := s.request(ctx, http.MethodGet, "/v1/subscriptions/"+url.PathEscape(id), nil, "", &result)
	if err != nil {
		return plan.Subscription{}, err
	}
	if id != result.ID || result.Status == "" {
		return plan.Subscription{}, errors.New("invalid Stripe subscription")
	}
	sub := plan.Subscription{
		ID: id, AccountID: result.Metadata[accountMetadata], Source: stripeSource, Status: result.Status,
		CustomerID: result.Customer, CancelAtPeriodEnd: result.CancelAtPeriodEnd, EndsAt: stripeTime(result.EndedAt),
	}
	for _, item := range result.Items.Data {
		if item.Price.ID != "" && (item.Price.ID == annual || item.Price.ID == monthly) {
			sub.PriceID, sub.Interval = item.Price.ID, item.Price.Recurring.Interval
			sub.CurrentPeriodEnd = stripeTime(item.CurrentPeriodEnd)
			if sub.CurrentPeriodEnd.IsZero() {
				sub.CurrentPeriodEnd = stripeTime(result.CurrentPeriodEnd)
			}
			break
		}
	}
	switch result.CancellationDetails.Reason {
	case "cancellation_requested", "payment_disputed", "payment_failed":
		sub.CancellationReason = result.CancellationDetails.Reason
	}
	if sub.PriceID == "" || result.LatestInvoice == "" {
		return sub, nil
	}
	invoice, first, err := s.lastPaidInvoice(ctx, id)
	if err != nil {
		return plan.Subscription{}, err
	}
	sub.LastInvoiceID, sub.LastPaymentAt = invoice.ID, stripeTime(invoice.Transitions.PaidAt)
	sub.RefundablePayment = first || invoice.annualRenewal(annual)
	return sub, nil
}

func (i stripeInvoice) annualRenewal(annual string) bool {
	if i.BillingReason != "subscription_cycle" || annual == "" {
		return false
	}
	for _, line := range i.Lines.Data {
		if line.Pricing.Details.Price == annual {
			return true
		}
	}
	return false
}

func (s *Stripe) lastPaidInvoice(ctx context.Context, subscription string) (stripeInvoice, bool, error) {
	params := url.Values{"status": {stripeStatusPaid}, "subscription": {subscription}, stripeLimitParam: {"100"}}
	var latest stripeInvoice
	paid := 0
	for {
		var page stripeList[stripeInvoice]
		if err := s.request(ctx, http.MethodGet, "/v1/invoices", params, "", &page); err != nil {
			return stripeInvoice{}, false, err
		}
		for _, invoice := range page.Data {
			if invoice.AmountPaid <= 0 || invoice.Transitions.PaidAt <= 0 {
				continue
			}
			paid++
			if invoice.Transitions.PaidAt > latest.Transitions.PaidAt {
				latest = invoice
			}
		}
		if !page.HasMore {
			return latest, paid == 1, nil
		}
		if len(page.Data) == 0 || page.Data[len(page.Data)-1].ID == params.Get("starting_after") {
			return stripeInvoice{}, false, errors.New("Stripe invoice pagination did not advance")
		}
		params.Set("starting_after", page.Data[len(page.Data)-1].ID)
	}
}

func stripeTime(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}
