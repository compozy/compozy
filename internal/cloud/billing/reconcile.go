// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

const ReconcileInterval = 6 * time.Hour

func (s *Service) Reconcile(ctx context.Context) error {
	if s.cfg.Stripe == nil {
		return nil
	}
	cutoff := s.cfg.Now().Add(-ReconcileInterval)
	ids, err := s.db.BillingStripeSubscriptionIDs(ctx, cutoff)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		failure := s.reconcileOne(ctx, id, cutoff)
		if failure != nil {
			failures = append(failures, failure)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *Service) reconcileOne(ctx context.Context, id string, cutoff time.Time) error {
	work, cancel := context.WithTimeout(ctx, 3*stripeTimeout)
	defer cancel()
	return s.applySubscriptionRefresh(work, id, stripeEvent{}, "", cutoff)
}

func (s *Service) ReconcileComplimentary(ctx context.Context) error {
	now := s.cfg.Now()
	accounts, err := s.db.ExpiredComplimentaryAccounts(ctx, now)
	if err != nil {
		return err
	}
	var failures []error
	for _, accountID := range accounts {
		err := s.db.Transaction(ctx, func(q *store.Queries) error {
			marked, err := q.MarkComplimentaryReconciled(ctx, accountID, now)
			if err != nil || !marked {
				return err
			}
			subscriptions, err := q.Subscriptions(ctx, accountID)
			if err != nil {
				return err
			}
			tier := s.status(subscriptions, now).Plan
			if tier == plan.Free {
				if err := q.StartProLapse(ctx, accountID, now); err != nil {
					return err
				}
			}
			return q.ApplyAccountPlan(ctx, accountID, tier, now, true)
		})
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
