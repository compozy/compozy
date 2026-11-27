// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
)

type queryStarted struct{}

type queryTiming struct {
	metrics *observe.Metrics
}

var _ pgx.QueryTracer = queryTiming{}

func (t queryTiming) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStarted{}, time.Now())
}

func (t queryTiming) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	start, ok := ctx.Value(queryStarted{}).(time.Time)
	if ok {
		t.metrics.Duration("cloud_db_latency_seconds", time.Since(start))
	}
}

func (q *Queries) RecordInstallationActivity(
	ctx context.Context,
	accountID, installationID string,
	at time.Time,
) error {
	if installationID == "" {
		return nil
	}
	return q.generated.RecordInstallationActivity(ctx, sqlcgen.RecordInstallationActivityParams{
		AccountID: accountID, InstallationID: installationID, SeenAt: timestamp(at),
	})
}

type Activation struct {
	Provider            string
	Accounts, Activated int64
}

func (q *Queries) ActivationFunnel(ctx context.Context, since, asOf time.Time) ([]Activation, error) {
	rows, err := q.generated.ActivationFunnel(ctx, sqlcgen.ActivationFunnelParams{
		Since: timestamp(since), AsOf: timestamp(asOf),
	})
	if err != nil {
		return nil, err
	}
	funnel := make([]Activation, len(rows))
	for i, row := range rows {
		funnel[i] = Activation{Provider: row.Provider, Accounts: row.Accounts, Activated: row.Activated}
	}
	return funnel, nil
}

func (s *Store) OperationalGauges(ctx context.Context, now time.Time, month, year string) ([]observe.Sample, error) {
	accounts, err := s.CloudAccountTotals(ctx)
	if err != nil {
		return nil, err
	}
	optedIn, err := s.generated.CloudMarketingOptIns(ctx)
	if err != nil {
		return nil, err
	}
	pro, err := s.generated.CloudProAccounts(ctx, sqlcgen.CloudProAccountsParams{
		AsOf: timestamp(now), Monthly: month, Annual: year,
	})
	if err != nil {
		return nil, err
	}
	allocated, err := s.CountAllocatedHostedTunnels(ctx)
	if err != nil {
		return nil, err
	}
	funnel, err := s.ActivationFunnel(ctx, now.AddDate(0, 0, -30), now)
	if err != nil {
		return nil, err
	}
	var gauges []observe.Sample
	var total int64
	for _, account := range accounts {
		total += account.Total
		gauges = append(gauges, observe.Sample{
			Name:   "cloud_accounts_total",
			Labels: observe.Labels{"provider": account.Provider},
			Value:  float64(account.Total),
		})
	}
	gauges = append(gauges,
		observe.Sample{Name: "cloud_tunnels_allocated", Value: float64(allocated)},
		observe.Sample{Name: "cloud_marketing_opt_in_accounts", Value: float64(optedIn)},
		observe.Sample{
			Name:   "cloud_subscriptions_active",
			Labels: observe.Labels{"plan": "free"},
			Value:  float64(max(0, total-pro)),
		},
		observe.Sample{Name: "cloud_subscriptions_active", Labels: observe.Labels{"plan": "pro"}, Value: float64(pro)},
	)
	for _, days := range [...]int{1, 7, 30} {
		activity, err := s.CloudAccountActivity(ctx, now.AddDate(0, 0, -days))
		if err != nil {
			return nil, err
		}
		window := strconv.Itoa(days) + "d"
		gauges = append(gauges, observe.Sample{
			Name: "cloud_accounts_active", Labels: observe.Labels{"window": window}, Value: float64(activity.Accounts),
		})
		for i, source := range [...]string{"session", "link", "token"} {
			gauges = append(gauges, observe.Sample{
				Name: "cloud_installations_active", Labels: observe.Labels{"window": window, "source": source},
				Value: float64([...]int64{activity.Sessions, activity.Links, activity.Tokens}[i]),
			})
		}
	}
	for _, row := range funnel {
		gauges = append(
			gauges,
			observe.Sample{Name: "cloud_activation_7d_share", Labels: observe.Labels{"provider": row.Provider},
				Value: float64(row.Activated) / float64(row.Accounts)},
		)
	}
	return gauges, nil
}
