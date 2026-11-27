// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"

	"github.com/compozy/compozy/internal/cloud/observe"
)

func (r *Router) Observe(metrics *observe.Metrics, reporter *observe.Sentry) {
	r.metrics, r.reporter = metrics, reporter
}

type observedResponse struct {
	http.ResponseWriter
	router  *Router
	request *http.Request
	final   bool
}

var _ http.ResponseWriter = (*observedResponse)(nil)

func (w *observedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *observedResponse) Write(data []byte) (int, error) {
	w.final = true
	return w.ResponseWriter.Write(data)
}

func (w *observedResponse) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	if w.final || status < 200 {
		return
	}
	w.final = true
	if status == http.StatusTooManyRequests {
		w.router.metrics.Count("cloud_rate_limited_total", observe.Labels{"route": w.request.Pattern})
	}
	if status < 500 {
		return
	}
	operation := "http_request"
	switch w.request.Pattern {
	case "POST /v1/billing/stripe-webhook":
		operation = "stripe_webhook"
		w.router.metrics.Count("cloud_stripe_webhook_failures_total", nil)
	case "GET /healthz":
		operation = "readiness"
	}
	w.router.reporter.Capture(w.request.Context(), operation, observe.Client{
		Version: w.request.Header.Get("X-Compozy-Version"), Platform: w.request.Header.Get("X-Compozy-Platform"),
	})
}
