package remotehost

import (
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"

	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	DescriptorPath      = remoteproto.DescriptorPath
	TunnelPath          = remoteproto.TunnelPath
	SessionPath         = remoteproto.SessionPath
	authorizationScheme = "CompozyRemote "
)

type listenerHandler struct {
	descriptor        remoteproto.Descriptor
	kind              Listener
	tls               *tls.Config
	sessions          *Sessions
	operator          http.Handler
	run               *hostRun
	webhook           func() http.Handler
	wrapTunnelHandler func(http.Handler) http.Handler
	hostedApp         http.Handler
	redeemBrowserCode func(context.Context, remoteproto.BrowserTokenRequest) (remoteproto.GrantResponse, error)
	browserTickets    *StreamTicketStore
}

func (h listenerHandler) outer() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET "+DescriptorPath, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, h.descriptor)
	})
	routes.HandleFunc("GET "+TunnelPath, h.serveTunnel)
	if h.run != nil && h.kind != ListenerDirect {
		routes.HandleFunc("/api/webhooks/", h.webhookRoutes())
	}
	if h.run != nil && h.kind == ListenerHosted {
		h.registerBrowserRoutes(routes)
		return h.guardHosted(routes)
	}
	return routes
}

func (h listenerHandler) serveTunnel(w http.ResponseWriter, req *http.Request) {
	socket, err := websocket.Accept(w, req, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	stream := websocket.NetConn(req.Context(), socket, websocket.MessageBinary)
	defer func() {
		if err := socket.CloseNow(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.DebugContext(req.Context(), "Remote tunnel close interrupted")
		}
	}()
	connection := tls.Server(stream, h.tls)
	handshake, cancel := context.WithTimeout(req.Context(), headerTimeout)
	defer cancel()
	if err := connection.HandshakeContext(handshake); err != nil {
		return
	}
	options := yamux.DefaultConfig()
	options.LogOutput = io.Discard
	mux, err := yamux.Server(connection, options)
	if err != nil {
		return
	}
	defer func() { _ = mux.Close() }()
	handler := h.inner()
	if h.wrapTunnelHandler != nil {
		handler = h.wrapTunnelHandler(handler)
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: headerTimeout,
		BaseContext: func(net.Listener) context.Context { return req.Context() },
	}
	server.SetKeepAlivesEnabled(false)
	defer func() { _ = server.Close() }()
	stop := context.AfterFunc(req.Context(), func() { _ = mux.Close() })
	defer stop()
	if err := server.Serve(mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.DebugContext(req.Context(), "Remote tunnel stopped")
	}
}

func (h listenerHandler) inner() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("POST "+SessionPath, h.mint)
	routes.HandleFunc("GET "+SessionPath, h.authenticate(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	routes.HandleFunc("DELETE "+SessionPath, h.authenticate(h.endSession))
	operator := h.authenticate(h.serveOperator)
	routes.HandleFunc("/api", operator)
	routes.HandleFunc("/api/", operator)
	routes.HandleFunc("/c", operator)
	routes.HandleFunc("/c/", operator)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := RejectClaimToken(req); err != nil {
			writeFailure(w, err)
			return
		}
		routes.ServeHTTP(w, req)
	})
}

func (h listenerHandler) mint(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.MintRequest
	if err := json.UnmarshalRead(http.MaxBytesReader(w, req.Body, 128<<10), &input); err != nil {
		writeFailure(w, &Failure{http.StatusBadRequest, invalidRemoteRequestCode, "The mint request is invalid.", nil})
		return
	}
	proof := requestProof(req, h.kind, input.DPoP)
	result, err := h.sessions.Mint(req.Context(), MintInput{Grant: input.Grant, Proof: proof})
	if err != nil {
		writeFailure(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h listenerHandler) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		credential, ok := strings.CutPrefix(req.Header.Get("Authorization"), authorizationScheme)
		if !ok {
			writeFailure(w, ErrSessionInvalid)
			return
		}
		proof := requestProof(req, h.kind, req.Header.Get("DPoP"))
		p, err := h.sessions.Authenticate(req.Context(), credential, proof, h.sessions.now())
		if err != nil {
			writeFailure(w, err)
			return
		}
		mutation := req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions &&
			req.URL.Path != SessionPath
		ctx, release, err := h.sessions.BeginRequest(req.Context(), p, mutation)
		if err != nil {
			writeFailure(w, err)
			return
		}
		defer release()
		next(w, req.WithContext(ctx))
	}
}

func (h listenerHandler) serveOperator(w http.ResponseWriter, req *http.Request) {
	if err := OperatorRouteRejection(req.URL.Path, false); err != nil {
		writeFailure(w, err)
		return
	}
	if h.operator == nil {
		writeFailure(w, ErrRouteNotRemote)
		return
	}
	h.operator.ServeHTTP(w, req)
}

func (h listenerHandler) endSession(w http.ResponseWriter, req *http.Request) {
	if err := h.revokeRequestSession(req); err != nil {
		writeFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h listenerHandler) revokeRequestSession(req *http.Request) error {
	p, _ := PrincipalFromContext(req.Context())
	ctx, cancel := context.WithTimeout(context.WithoutCancel(req.Context()), headerTimeout)
	defer cancel()
	return h.sessions.RevokeSession(ctx, p.SessionID)
}

func requestProof(req *http.Request, kind Listener, token string) DPoPProof {
	target := url.URL{Scheme: "https", Host: req.Host, Path: req.URL.Path, RawPath: req.URL.RawPath}
	return DPoPProof{Token: token, Method: req.Method, URL: target.String(), Listener: kind}
}

func writeFailure(w http.ResponseWriter, err error) {
	failure := DescribeError(err)
	writeJSON(w, failure.Status, struct {
		Error   string            `json:"error"`
		Code    string            `json:"code"`
		Details map[string]string `json:"details,omitempty"`
	}{failure.Message, failure.Code, failure.Details})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.MarshalWrite(w, value); err != nil {
		slog.Debug("Could not write remote host response", "status", status)
	}
}
