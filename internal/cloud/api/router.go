package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	CodeUnavailable = "cloud_unavailable"
	contentType     = "Content-Type"
	cacheControl    = "Cache-Control"
	jsonType        = "application/json"
	noStore         = "no-store"
)

type EndpointStore interface {
	Ping(context.Context) error
	GatePolicy(context.Context) (remoteproto.GatePolicy, error)
}

type Router struct {
	mux      *http.ServeMux
	store    EndpointStore
	keys     *SigningKeys
	ready    func() bool
	draining func() bool
	now      func() time.Time
}

var _ http.Handler = (*Router)(nil)

func NewRouter(
	db EndpointStore, keys *SigningKeys, ready, draining func() bool, now func() time.Time,
) *Router {
	r := &Router{mux: http.NewServeMux(), store: db, keys: keys, ready: ready, draining: draining, now: now}
	r.mux.HandleFunc("GET /healthz", r.health)
	r.mux.HandleFunc("GET /v1/jwks", r.jwks)
	r.mux.HandleFunc("GET /.well-known/jwks.json", r.jwks)
	r.mux.HandleFunc("GET /v1/gate-policy", r.gatePolicy)
	r.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		WriteError(w, http.StatusNotFound, "not_found", "Route not found.")
	})
	return r
}

func (r *Router) Routes() *http.ServeMux {
	return r.mux
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.draining() {
		writeUnavailable(w)
		return
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) health(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	if !r.ready() || r.store.Ping(ctx) != nil {
		writeUnavailable(w)
		return
	}
	w.Header().Set(cacheControl, noStore)
	WriteJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}

func (r *Router) jwks(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(cacheControl, "public, max-age=60")
	WriteJSON(w, http.StatusOK, r.keys.JWKS())
}

func (r *Router) gatePolicy(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	policy, err := r.store.GatePolicy(ctx)
	if err != nil {
		writeUnavailable(w)
		return
	}
	policy.IssuedAt = r.now().Unix()
	token, err := r.keys.Signer().SignGatePolicy(policy)
	if err != nil {
		writeUnavailable(w)
		return
	}
	w.Header().Set(contentType, "application/jose")
	w.Header().Set(cacheControl, "public, max-age=0, s-maxage=60")
	if _, err := io.WriteString(w, token); err != nil {
		slog.Debug("Cloud gate policy response interrupted")
	}
}

type ErrorResponse struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Details map[string]any `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		slog.Error("Cloud response encoding failed")
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, jsonType)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		slog.Debug("Cloud response interrupted", "status", status)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set(cacheControl, noStore)
	WriteJSON(w, status, ErrorResponse{Code: code, Error: message})
}

func writeUnavailable(w http.ResponseWriter) {
	setRetryAfter(w, 1)
	WriteError(w, http.StatusServiceUnavailable, CodeUnavailable, "Compozy Cloud is temporarily unavailable.")
}

func setRetryAfter(w http.ResponseWriter, seconds int) {
	w.Header().Set("Retry-After", fmt.Sprint(seconds))
}
