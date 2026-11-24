package control

import (
	"context"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Service) Account(ctx context.Context, principal DevicePrincipal) (remoteproto.AccountSummary, error) {
	var result remoteproto.AccountSummary
	err := s.deviceTransaction(
		ctx,
		principal,
		func(q *store.Queries, account store.ControlAccount, device store.ControlDevice) error {
			identity, err := q.LockAuthAccount(ctx, account.ID)
			if err != nil {
				return err
			}
			session, err := q.ReadDeviceSession(ctx, principal.SessionID, account.ID)
			if err != nil {
				return err
			}
			entitlement, err := s.accountPlan(ctx, q, identity)
			if err != nil {
				return err
			}
			result = remoteproto.AccountSummary{
				AccountPlan: entitlement, Email: identity.Email, Name: identity.Name, Provider: session.Provider,
				MarketingOptIn: identity.MarketingOptIn, Device: viewDevice(device, principal.DeviceID),
			}
			return nil
		},
	)
	return result, err
}

func (s *Service) accountPlan(
	ctx context.Context,
	q *store.Queries,
	account store.AuthAccount,
) (remoteproto.AccountPlan, error) {
	var result remoteproto.AccountPlan
	now := s.config.Now()
	subscriptions, err := q.Subscriptions(ctx, account.ID)
	if err != nil {
		return result, err
	}
	billing := plan.BillingStatus(subscriptions, s.config.MonthlyPrice, s.config.AnnualPrice, now)
	if err := q.ApplyAccountPlan(ctx, account.ID, billing.Plan, now, false); err != nil {
		return result, err
	}
	usage, err := q.PlanUsage(ctx, account.ID, now)
	if err != nil {
		return result, err
	}
	result.Plan = string(billing.Plan)
	result.PaymentIssue = billing.PaymentIssue
	result.Subscription = billing.AccountSubscription()
	result.RemoteComputersUsed = usage.EnabledComputers + usage.ActiveReservations
	if billing.Plan == plan.Free {
		result.RemoteComputersLimit = new(int64(1))
		return result, nil
	}
	result.CloudRoutesLimit = int64(account.CloudRoutesMax)
	result.CloudRoutesUsed, err = q.AccountHostedRouteUsage(ctx, account.ID)
	return result, err
}
