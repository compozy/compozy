package api

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
)

func RegisterAuthRoutes(mux *http.ServeMux, service *auth.Service) {
	h := authRoutes{service: service}
	mux.HandleFunc("POST /v1/device-flow", h.start)
	mux.HandleFunc("DELETE /v1/device-flow/{device_code}", h.cancel)
	mux.HandleFunc("POST /v1/device-flow/token", h.poll)
	mux.HandleFunc("POST /v1/devices/enroll", h.enroll)
	mux.HandleFunc("POST /v1/session/refresh", h.refresh)
	mux.HandleFunc("POST /v1/session/logout", h.logout)
}

type authRoutes struct{ service *auth.Service }

func (h authRoutes) start(w http.ResponseWriter, r *http.Request) {
	var input remoteproto.DeviceFlowRequest
	if !decodeAuthRequest(w, r, &input) {
		return
	}
	out, err := h.service.StartDeviceGrant(r.Context(), input, h.service.RequestLocation(r.Header))
	writeAuthResult(w, out, err)
}

func (h authRoutes) cancel(w http.ResponseWriter, r *http.Request) {
	writeAuthResult(w, nil, h.service.CancelDeviceGrant(r.Context(), r.PathValue("device_code")))
}

func (h authRoutes) poll(w http.ResponseWriter, r *http.Request) {
	var input remoteproto.DeviceFlowTokenRequest
	if !decodeAuthRequest(w, r, &input) {
		return
	}
	out, err := h.service.PollDeviceGrant(r.Context(), input.DeviceCode)
	if f, ok := errors.AsType[*auth.Failure](err); ok && f.Status == http.StatusBadRequest {
		w.Header().Set(cacheControl, noStore)
		WriteJSON(w, f.Status, struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
			Code        string `json:"code"`
		}{Error: f.Code, Description: f.Message, Code: f.Code})
		return
	}
	writeAuthResult(w, out, err)
}

func (h authRoutes) enroll(w http.ResponseWriter, r *http.Request) {
	var input remoteproto.DeviceEnrollRequest
	if !decodeAuthRequest(w, r, &input) {
		return
	}
	out, err := h.service.Enroll(r.Context(), auth.RequestDeviceProof(r), input)
	writeAuthResult(w, out, err)
}

func (h authRoutes) refresh(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.Refresh(r.Context(), auth.RequestDeviceProof(r))
	writeAuthResult(w, out, err)
}

func (h authRoutes) logout(w http.ResponseWriter, r *http.Request) {
	writeAuthResult(w, nil, h.service.Logout(r.Context(), auth.RequestDeviceProof(r)))
}

func decodeAuthRequest(w http.ResponseWriter, r *http.Request, body any) bool {
	const maxAuthBody = 16 << 10
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	if err := json.UnmarshalRead(r.Body, body, json.RejectUnknownMembers(true)); err != nil {
		WriteAuthError(w, auth.ErrInvalidRequest)
		return false
	}
	return true
}

func writeAuthResult(w http.ResponseWriter, value any, err error) {
	w.Header().Set(cacheControl, noStore)
	switch {
	case err != nil:
		WriteAuthError(w, err)
	case value == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		WriteJSON(w, http.StatusOK, value)
	}
}

func WriteAuthError(w http.ResponseWriter, err error) {
	f, ok := errors.AsType[*auth.Failure](err)
	if !ok {
		writeUnavailable(w)
		return
	}
	w.Header().Set(cacheControl, noStore)
	if f.RetryAfter > 0 {
		setRetryAfter(w, f.RetryAfter)
	}
	response := ErrorResponse{Code: f.Code, Error: f.Message, Reason: f.Reason}
	WriteJSON(w, f.Status, response)
}

type DeviceAuthenticator interface {
	AuthenticateDevice(*http.Request) (auth.DevicePrincipal, error)
}

type devicePrincipalKey struct{}

func DevicePrincipalFromContext(ctx context.Context) (auth.DevicePrincipal, bool) {
	principal, ok := ctx.Value(devicePrincipalKey{}).(auth.DevicePrincipal)
	return principal, ok
}

func RequireDevice(authenticator DeviceAuthenticator, limiter *AccountLimiter, next http.Handler) http.Handler {
	limited := limiter.AccountWrites(func(r *http.Request) (string, bool) {
		principal, ok := DevicePrincipalFromContext(r.Context())
		return principal.AccountID, ok
	}, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := authenticator.AuthenticateDevice(r)
		if err != nil {
			WriteAuthError(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), devicePrincipalKey{}, principal)
		limited.ServeHTTP(w, r.WithContext(ctx))
	})
}
