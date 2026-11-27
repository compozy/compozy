// SPDX-License-Identifier: FSL-1.1-MIT

package admin

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/remoteproto"
)

type metric struct {
	Label string
	Value string
}

type overviewView struct {
	Totals, Conversions, Activation []metric
	Gate                            remoteproto.GatePolicy
}

func (c *console) overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	samples, err := c.config.Metrics(ctx)
	if err != nil {
		c.fail(w, err)
		return
	}
	policy, err := c.db.GatePolicy(ctx)
	if err != nil {
		c.fail(w, err)
		return
	}
	data := overviewView{Gate: policy}
	var accountCount, activeCount, proCount float64
	for _, sample := range samples {
		labels := sample.Labels
		switch sample.Name {
		case "cloud_accounts_total":
			accountCount += sample.Value
		case "cloud_accounts_active":
			if labels["window"] == "7d" {
				activeCount = sample.Value
			}
		case "cloud_installations_active":
			if labels["window"] == "7d" {
				data.Totals = append(data.Totals, count("Active installations ("+labels["source"]+")", sample.Value))
			}
		case "cloud_subscriptions_active":
			if labels["plan"] == "pro" {
				proCount = sample.Value
			}
		case "cloud_activation_7d_share":
			label := "Activated in 7 days · " + labels["provider"]
			value := strconv.FormatFloat(sample.Value*100, 'f', 1, 64) + "%"
			data.Activation = append(data.Activation, metric{label, value})
		default:
			if label := conversionLabel(sample); label != "" {
				data.Conversions = append(data.Conversions, count(label, sample.Value))
			}
		}
	}
	for _, items := range [][]metric{data.Totals, data.Conversions, data.Activation} {
		slices.SortFunc(items, func(a, b metric) int { return cmp.Compare(a.Label, b.Label) })
	}
	data.Totals = append([]metric{
		count(
			"Accounts",
			accountCount,
		),
		count("Active accounts in 7 days", activeCount),
		count("Pro accounts", proCount),
	}, data.Totals...)
	c.render(w, http.StatusOK, page{Title: "Overview", Overview: &data})
}

func count(label string, value float64) metric {
	return metric{Label: label, Value: strconv.FormatFloat(value, 'f', 0, 64)}
}

func conversionLabel(sample observe.Sample) string {
	labels := sample.Labels
	switch sample.Name {
	case "cloud_paywall_hits_total":
		return "Paywall hits · " + strings.ReplaceAll(labels["reason"], "_", " ")
	case "cloud_checkout_sessions_total":
		return "Checkouts · " + labels["interval"] + " · " + labels["result"]
	case "cloud_subscription_ends_total":
		return "Subscription ends · " + strings.ReplaceAll(labels["reason"], "_", " ")
	case "cloud_new_accounts_total":
		return "New accounts · " + labels["provider"]
	default:
		return ""
	}
}
