package api

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/control"
)

const (
	controlRequestLimit   = 64 << 10
	controlRequestTimeout = 4 * time.Second
	authorizationHeader   = "Authorization"
)

type ControlRoutesConfig struct {
	Service       *control.Service
	Hub           *control.Hub
	Authenticator DeviceAuthenticator
	Limiter       *AccountLimiter
	Issuer        string
	Logger        *slog.Logger
}

type controlRoutes struct {
	config ControlRoutesConfig
	keys   *SigningKeys
	now    func() time.Time
}

func (r *Router) RegisterControl(cfg ControlRoutesConfig) {
	h := &controlRoutes{config: cfg, keys: r.keys, now: r.now}
	r.mux.Handle("GET /v1/devices", h.device(h.devices))
	r.mux.Handle("GET /v1/devices/{id}", h.device(h.getDevice))
	r.mux.Handle("PATCH /v1/devices/{id}", h.device(h.patchDevice))
	r.mux.Handle("DELETE /v1/devices/{id}", h.device(h.revokeDevice))
	r.mux.Handle("GET /v1/computers", h.device(h.computers))
	r.mux.Handle("POST /v1/computers/reservations", h.device(h.reserve))
	r.mux.Handle("DELETE /v1/computers/reservations/{id}", h.device(h.releaseReservation))
	r.mux.Handle("POST /v1/computers/link-tokens", h.device(h.linkToken))
	r.mux.HandleFunc("POST /v1/computers/link", h.link)
	r.mux.HandleFunc("PATCH /v1/computers/{id}", h.patchComputer)
	r.mux.Handle("DELETE /v1/computers/{id}", h.device(h.unlink))
	r.mux.Handle("POST /v1/computers/{id}/grants", h.device(h.grant))
	r.mux.HandleFunc("GET /v1/computers/control", h.controlLink)
	r.mux.Handle("GET /v1/approvals", h.device(h.approvals))
	r.mux.Handle("GET /v1/approvals/{id}", h.device(h.getApproval))
	r.mux.Handle("POST /v1/approvals/{id}/decision", h.device(h.decideApproval))
	r.mux.Handle("GET /v1/account/events", h.device(h.accountEvents))
	r.mux.Handle("GET /v1/account", h.device(h.account))
}

func (r *controlRoutes) account(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	account, err := r.config.Service.Account(ctx, principal)
	r.respond(w, req, http.StatusOK, account, err)
}

type deviceHandler func(http.ResponseWriter, *http.Request, control.DevicePrincipal)

func (r *controlRoutes) device(next deviceHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		principal, err := r.authenticateDevice(req)
		if err != nil {
			r.fail(w, req, err)
			return
		}
		r.accountRequest(w, req, principal.AccountID, func(w http.ResponseWriter, req *http.Request) {
			next(w, req, principal)
		})
	})
}

func (r *controlRoutes) accountRequest(w http.ResponseWriter, req *http.Request, id string, next http.HandlerFunc) {
	w.Header().Set(cacheControl, noStore)
	r.config.Limiter.AccountWrites(func(*http.Request) (string, bool) { return id, id != "" }, next).ServeHTTP(w, req)
}

func (r *controlRoutes) authenticateDevice(req *http.Request) (control.DevicePrincipal, error) {
	if r.config.Authenticator == nil {
		return control.DevicePrincipal{}, control.ErrUnauthorized
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	principal, err := r.config.Authenticator.AuthenticateDevice(req.WithContext(ctx))
	if err != nil {
		return control.DevicePrincipal{}, err
	}
	return control.DevicePrincipal{
		AccountID: principal.AccountID,
		DeviceID:  principal.DeviceID,
		JKT:       principal.JKT,
		SessionID: principal.SessionID,
	}, nil
}

func (r *controlRoutes) targetURL(req *http.Request) string {
	return r.config.Issuer + req.URL.EscapedPath()
}

func (r *controlRoutes) fail(w http.ResponseWriter, req *http.Request, err error) {
	if failure, ok := errors.AsType[*control.Error](err); ok {
		WriteError(w, failure.Status, failure.Code, failure.Message)
		return
	}
	if _, ok := errors.AsType[*auth.Failure](err); ok {
		WriteAuthError(w, err)
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeUnavailable(w)
		return
	}
	r.config.Logger.ErrorContext(
		req.Context(),
		"Cloud control request failed",
		"method",
		req.Method,
		"route",
		req.Pattern,
	)
	WriteError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
}

func (r *controlRoutes) respond(w http.ResponseWriter, req *http.Request, status int, data any, err error) {
	if err != nil {
		r.fail(w, req, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	WriteJSON(w, status, data)
}

func (r *controlRoutes) decode(w http.ResponseWriter, req *http.Request, value any, allowEmpty bool) ([]byte, bool) {
	body, err := readControlBody(w, req)
	if err == nil && allowEmpty && len(bytes.TrimSpace(body)) == 0 {
		return body, true
	}
	if err != nil || json.Unmarshal(body, value, json.RejectUnknownMembers(true)) != nil ||
		bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		r.fail(w, req, control.ErrInvalidInput)
		return nil, false
	}
	return body, true
}

func readControlBody(w http.ResponseWriter, req *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(w, req.Body, controlRequestLimit))
}

func controlContext(req *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(req.Context(), controlRequestTimeout)
}
