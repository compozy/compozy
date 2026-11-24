package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

const hostedPlanRetention = 7 * 24 * time.Hour

func (q *Queries) LockPlanAccount(ctx context.Context, accountID string) error {
	if !q.inTx {
		return errors.New("locking a plan account requires a transaction")
	}
	if _, err := q.generated.LockPlanAccount(ctx, accountID); err != nil {
		return fmt.Errorf("lock plan account: %w", err)
	}
	return nil
}

func (q *Queries) PlanUsage(ctx context.Context, accountID string, now time.Time) (plan.Usage, error) {
	usage, err := q.generated.PlanUsage(ctx, sqlcgen.PlanUsageParams{AccountID: accountID, AsOf: timestamp(now)})
	if err != nil {
		return plan.Usage{}, fmt.Errorf("count plan slots: %w", err)
	}
	return plan.Usage{EnabledComputers: usage.EnabledComputers, ActiveReservations: usage.ActiveReservations}, nil
}

func (q *Queries) Subscriptions(ctx context.Context, accountID string) ([]plan.Subscription, error) {
	rows, err := q.generated.PlanSubscriptions(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("read plan subscriptions: %w", err)
	}
	subscriptions := make([]plan.Subscription, 0, len(rows))
	for _, row := range rows {
		subscriptions = append(subscriptions, planSubscription(row))
	}
	return subscriptions, nil
}

// ApplyAccountPlan expects the account row lock. Retention is set once per lapse.
func (q *Queries) ApplyAccountPlan(
	ctx context.Context, accountID string, tier plan.Plan, asOf time.Time, billingChanged bool,
) error {
	limited, err := q.ApplyControlPlan(ctx, accountID, tier)
	if err != nil {
		return err
	}
	var deleteAfter time.Time
	if tier == plan.Free {
		deleteAfter = asOf.Add(hostedPlanRetention)
		released, err := q.generated.ReleaseExcessFreeReservations(ctx, sqlcgen.ReleaseExcessFreeReservationsParams{
			AccountID: accountID, AsOf: timestamp(asOf),
		})
		if err != nil {
			return err
		}
		billingChanged = billingChanged || released > 0
	}
	tunnels, err := q.generated.SetHostedPlanRetention(ctx, sqlcgen.SetHostedPlanRetentionParams{
		AccountID: accountID, DeleteAfter: timestamp(deleteAfter),
	})
	if err != nil || (!billingChanged && !limited && tunnels == 0) {
		return err
	}
	return q.NotifyControlEvent(ctx, ControlEvent{Type: remoteproto.ControlPlanChanged, AccountID: accountID})
}

func (q *Queries) AccountHostedRouteUsage(ctx context.Context, accountID string) (int64, error) {
	return q.generated.AccountHostedRouteUsage(ctx, accountID)
}
