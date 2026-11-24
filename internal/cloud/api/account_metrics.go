// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

type AccountMetricsStore interface {
	CloudAccountTotals(context.Context) ([]store.AccountTotal, error)
	CloudAccountActivity(context.Context, time.Time) (store.AccountActivity, error)
}

func (r *Router) RegisterAccountMetrics(db AccountMetricsStore, bearer string) {
	if bearer == "" {
		return
	}
	r.mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, req *http.Request) {
		if subtle.ConstantTimeCompare([]byte(req.Header.Get(authorizationHeader)), []byte("Bearer "+bearer)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
			return
		}
		ctx, cancel := controlContext(req)
		defer cancel()
		metrics, err := accountMetrics(ctx, db, r.now())
		if err != nil {
			writeUnavailable(w)
			return
		}
		w.Header().Set(contentType, "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set(cacheControl, noStore)
		if _, err := w.Write(metrics); err != nil {
			slog.DebugContext(ctx, "Cloud metrics response interrupted")
		}
	})
}

func accountMetrics(ctx context.Context, db AccountMetricsStore, now time.Time) ([]byte, error) {
	totals, err := db.CloudAccountTotals(ctx)
	if err != nil {
		return nil, err
	}
	byProvider := make(map[string]int64, len(totals))
	for _, row := range totals {
		byProvider[row.Provider] = row.Total
	}
	metrics := []byte("# TYPE cloud_accounts_total gauge\n")
	for _, provider := range []string{"google", "github"} {
		metrics = fmt.Appendf(metrics, "cloud_accounts_total{provider=%q} %d\n", provider, byProvider[provider])
	}
	metrics = append(metrics, "# TYPE cloud_accounts_active gauge\n# TYPE cloud_installations_active gauge\n"...)
	const installationActivityMetric = "cloud_installations_active{window=\"%dd\",source=%q} %d\n"
	for _, days := range []int{1, 7, 30} {
		active, err := db.CloudAccountActivity(ctx, now.Add(-time.Duration(days)*24*time.Hour))
		if err != nil {
			return nil, err
		}
		metrics = fmt.Appendf(metrics, "cloud_accounts_active{window=\"%dd\"} %d\n", days, active.Accounts)
		metrics = fmt.Appendf(metrics, installationActivityMetric, days, "session", active.Sessions)
		metrics = fmt.Appendf(metrics, installationActivityMetric, days, "link", active.Links)
		metrics = fmt.Appendf(metrics, installationActivityMetric, days, "token", active.Tokens)
	}
	return metrics, nil
}
