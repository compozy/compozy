package remotehost

import (
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"

	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	DescriptorPath      = "/remote/v1/descriptor"
	TunnelPath          = "/remote/v1/tunnel"
	SessionPath         = "/remote/v1/session"
	authorizationScheme = "CompozyRemote "
)

type listenerHandler struct {
	descriptor remoteproto.Descriptor
	kind       Listener
	tls        *tls.Config
	sessions   *Sessions
	operator   http.Handler
}

func (h listenerHandler) outer() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET "+DescriptorPath, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, h.descriptor)
	})
	routes.HandleFunc("GET "+TunnelPath, h.serveTunnel)
	return routes
}

func (h listenerHandler) serveTunnel(w http.ResponseWriter, req *http.Request) {
	socket, err := websocket.Accept(w, req, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	stream := websocket.NetConn(req.Context(), socket, websocket.MessageBinary)
	defer func() { _ = socket.CloseNow() }()
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
	server := &http.Server{
		Handler: h.inner(), ReadHeaderTimeout: headerTimeout,
		BaseContext: func(net.Listener) context.Context { return req.Context() },
	}
	server.SetKeepAlivesEnabled(false)
	defer func() { _ = server.Close() }()
	stop := context.AfterFunc(req.Context(), func() { _ = mux.Close() })
	defer stop()
	_ = server.Serve(mux)
}

func (h listenerHandler) inner() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("POST "+SessionPath, h.mint)
	routes.HandleFunc("DELETE "+SessionPath, h.authenticate(h.endSession))
	routes.HandleFunc("/api/", h.authenticate(h.serveOperator))
	routes.HandleFunc("/c/", h.authenticate(func(w http.ResponseWriter, _ *http.Request) {
		writeFailure(w, &Failure{http.StatusForbidden, "remote_bridge_not_transitive",
			"A remote connection cannot use this computer to reach another computer.", nil})
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if len(req.Header.Values("X-Compozy-Claim-Token")) != 0 {
			writeFailure(w, &Failure{http.StatusBadRequest, "claim_token_not_remote",
				"Task leases stay on the computer that owns them.", nil})
			return
		}
		routes.ServeHTTP(w, req)
	})
}

func (h listenerHandler) mint(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.MintRequest
	if err := json.UnmarshalRead(http.MaxBytesReader(w, req.Body, 128<<10), &input); err != nil {
		writeFailure(w, &Failure{http.StatusBadRequest, "invalid_remote_request", "The mint request is invalid.", nil})
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
	if h.operator == nil {
		writeFailure(w, ErrRouteNotRemote)
		return
	}
	h.operator.ServeHTTP(w, req)
}

func (h listenerHandler) endSession(w http.ResponseWriter, req *http.Request) {
	p, _ := PrincipalFromContext(req.Context())
	if err := h.sessions.RevokeSession(req.Context(), p.SessionID); err != nil {
		writeFailure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	_ = json.MarshalWrite(w, value)
}
