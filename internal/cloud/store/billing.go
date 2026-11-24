package store

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

func planSubscription(s sqlcgen.Subscription) plan.Subscription {
	return plan.Subscription{
		ID: s.ID, AccountID: s.AccountID, Source: s.Source, Status: s.Status, PriceID: s.PriceID.String,
		Interval: s.Interval.String, CustomerID: s.StripeCustomerID.String,
		CurrentPeriodEnd: s.CurrentPeriodEnd.Time, CancelAtPeriodEnd: s.CancelAtPeriodEnd,
		LastPaymentAt: s.LastPaymentAt.Time, EndsAt: s.EndsAt.Time,
		LastInvoiceID: s.LastInvoiceID.String, RefundablePayment: s.RefundablePayment,
	}
}

func (q *Queries) LockStripeSubscription(ctx context.Context, id string) error {
	if !q.inTx {
		return errors.New("locking a Stripe subscription requires a transaction")
	}
	return q.generated.LockStripeSubscription(ctx, id)
}

func (q *Queries) InsertStripeEvent(ctx context.Context, id, kind string, now time.Time) (bool, error) {
	count, err := q.generated.InsertStripeEvent(ctx, sqlcgen.InsertStripeEventParams{
		EventID: id, Type: kind, ReceivedAt: timestamp(now),
	})
	return count == 1, err
}

func (q *Queries) MarkStripeEventProcessed(ctx context.Context, id string, now time.Time) error {
	return q.generated.MarkStripeEventProcessed(ctx, sqlcgen.MarkStripeEventProcessedParams{
		EventID: id, ProcessedAt: timestamp(now),
	})
}

func (q *Queries) BillingSubscription(ctx context.Context, id string) (plan.Subscription, error) {
	row, err := q.generated.BillingSubscription(ctx, id)
	return planSubscription(row), err
}

func (q *Queries) SaveStripeSubscription(ctx context.Context, s plan.Subscription, now time.Time) error {
	count, err := q.generated.SaveStripeSubscription(ctx, sqlcgen.SaveStripeSubscriptionParams{
		ID: s.ID, AccountID: s.AccountID, StripeCustomerID: dbText(s.CustomerID),
		Status: s.Status, PriceID: dbText(s.PriceID), Interval: dbText(s.Interval),
		CurrentPeriodEnd: timestamp(s.CurrentPeriodEnd), CancelAtPeriodEnd: s.CancelAtPeriodEnd,
		LastPaymentAt: timestamp(s.LastPaymentAt), EndsAt: timestamp(s.EndsAt),
		LastInvoiceID: dbText(s.LastInvoiceID), RefundablePayment: s.RefundablePayment, AsOf: timestamp(now),
	})
	if err == nil && count != 1 {
		return errors.New("stripe subscription already belongs to another entitlement")
	}
	return err
}

func (q *Queries) BillingStripeSubscriptionIDs(ctx context.Context) ([]string, error) {
	return q.generated.BillingStripeSubscriptionIDs(ctx)
}

func (q *Queries) ExpiredComplimentaryAccounts(ctx context.Context, now time.Time) ([]string, error) {
	return q.generated.ExpiredComplimentaryAccounts(ctx, timestamp(now))
}

func (q *Queries) MarkComplimentaryReconciled(ctx context.Context, account string, now time.Time) (bool, error) {
	count, err := q.generated.MarkComplimentaryReconciled(ctx, sqlcgen.MarkComplimentaryReconciledParams{
		AccountID: account, AsOf: timestamp(now),
	})
	return count == 1, err
}

func (q *Queries) EndComplimentarySubscriptions(ctx context.Context, account string, now time.Time) error {
	return q.generated.EndComplimentarySubscriptions(ctx, sqlcgen.EndComplimentarySubscriptionsParams{
		AccountID: account, EndsAt: timestamp(now),
	})
}

type BillingCheckout struct {
	ID, AccountID, Interval, Hostname string
}

func (q *Queries) InsertBillingCheckout(ctx context.Context, c BillingCheckout) error {
	return q.generated.InsertBillingCheckout(ctx, sqlcgen.InsertBillingCheckoutParams{
		ID: c.ID, AccountID: c.AccountID, Interval: c.Interval,
		Hostname: dbText(c.Hostname),
	})
}

func billingCheckout(c sqlcgen.BillingCheckout) BillingCheckout {
	return BillingCheckout{ID: c.ID, AccountID: c.AccountID, Interval: c.Interval, Hostname: c.Hostname.String}
}

func (q *Queries) AccountBillingCheckout(ctx context.Context, account, id string) (BillingCheckout, error) {
	row, err := q.generated.AccountBillingCheckout(
		ctx,
		sqlcgen.AccountBillingCheckoutParams{ID: id, AccountID: account},
	)
	return billingCheckout(row), err
}

func (q *Queries) BillingCheckouts(ctx context.Context, account string) ([]BillingCheckout, error) {
	rows, err := q.generated.BillingCheckouts(ctx, account)
	if err != nil {
		return nil, err
	}
	results := make([]BillingCheckout, 0, len(rows))
	for _, c := range rows {
		results = append(results, billingCheckout(c))
	}
	return results, nil
}
