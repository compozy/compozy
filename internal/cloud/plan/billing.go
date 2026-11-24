package plan

import (
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

type Billing struct {
	Plan         Plan
	Subscription Subscription
	PaymentIssue bool
}

// Subscriptions arrive in display order, with Stripe before complimentary grants.
func BillingStatus(subscriptions []Subscription, monthlyPrice, annualPrice string, now time.Time) Billing {
	result := Billing{Plan: Free}
	for _, s := range subscriptions {
		if !s.Entitled(monthlyPrice, annualPrice, now) {
			continue
		}
		if result.Plan == Free {
			result.Plan, result.Subscription = Pro, s
		}
		result.PaymentIssue = result.PaymentIssue || (s.Source == "stripe" && s.Status == "past_due")
	}
	return result
}

func (s Subscription) RefundEligibleUntil() time.Time {
	if s.Source == "stripe" && s.RefundablePayment && !s.LastPaymentAt.IsZero() {
		return s.LastPaymentAt.Add(14 * 24 * time.Hour)
	}
	return time.Time{}
}

func (b Billing) AccountSubscription() *remoteproto.AccountSubscription {
	if b.Plan != Pro {
		return nil
	}
	s := b.Subscription
	end := s.CurrentPeriodEnd
	if s.Source == "complimentary" {
		end = s.EndsAt
	}
	result := &remoteproto.AccountSubscription{Source: s.Source, CancelAtPeriodEnd: s.CancelAtPeriodEnd}
	if s.Interval != "" {
		result.Interval = new(s.Interval)
	}
	if !end.IsZero() {
		result.CurrentPeriodEnd = new(end)
	}
	if until := s.RefundEligibleUntil(); !until.IsZero() {
		result.RefundEligibleUntil = new(until)
	}
	return result
}
