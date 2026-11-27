// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	billingTimeout  = 25 * time.Second
	stripeBodyLimit = 1 << 20
)

func billingDevice(principal control.DevicePrincipal) billing.Authorize {
	return func(ctx context.Context, q *store.Queries) error {
		active, err := q.ControlPrincipalActive(ctx, principal.AccountID,
			principal.DeviceID, principal.JKT, principal.SessionID)
		if err == nil && !active {
			return billing.ErrUnauthorized
		}
		return err
	}
}

func (r *controlRoutes) billingCheckout(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var input remoteproto.CheckoutRequest
	if _, ok := r.decode(w, req, &input, false); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), billingTimeout)
	defer cancel()
	target, err := r.config.Billing.Checkout(ctx, principal.AccountID,
		billing.Target{Interval: input.Interval}, billingDevice(principal))
	r.billingResult(w, req, target, err)
}

func (r *controlRoutes) billingPortal(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := context.WithTimeout(req.Context(), billingTimeout)
	defer cancel()
	target, err := r.config.Billing.Portal(ctx, principal.AccountID, billingDevice(principal))
	r.billingResult(w, req, target, err)
}

func (r *controlRoutes) billingResult(w http.ResponseWriter, req *http.Request, target string, err error) {
	if err != nil {
		r.billingError(w, req, err)
		return
	}
	WriteJSON(w, http.StatusOK, remoteproto.BillingURL{URL: target})
}

func (r *controlRoutes) stripeWebhook(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(cacheControl, noStore)
	ctx, cancel := context.WithTimeout(req.Context(), billingTimeout)
	defer cancel()
	body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, stripeBodyLimit))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid Stripe event.")
		return
	}
	if err := r.config.Billing.Webhook(ctx, req.Header.Get("Stripe-Signature"), body); err != nil {
		r.billingError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *controlRoutes) billingError(w http.ResponseWriter, req *http.Request, err error) {
	if f, ok := errors.AsType[*billing.Failure](err); ok && f.Status != http.StatusServiceUnavailable {
		WriteError(w, f.Status, f.Code, f.Message)
		return
	}
	r.config.Logger.WarnContext(req.Context(), "Cloud billing request failed", "route", req.Pattern)
	writeUnavailable(w)
}
