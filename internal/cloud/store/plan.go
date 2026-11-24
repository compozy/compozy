package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

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
		subscriptions = append(subscriptions, plan.Subscription{
			Source: row.Source, PriceID: row.PriceID.String, Status: row.Status, EndsAt: row.EndsAt.Time,
		})
	}
	return subscriptions, nil
}

func (q *Queries) AccountHostedRouteUsage(ctx context.Context, accountID string) (int64, error) {
	return q.generated.AccountHostedRouteUsage(ctx, accountID)
}
