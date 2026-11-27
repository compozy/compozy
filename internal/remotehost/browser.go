package remotehost

import (
	"context"
	"crypto/ed25519"
	"encoding/json/v2"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const browserSessionCookie = "__Host-compozy_rs"

type browserCloud interface {
	RedeemBrowserCode(
		context.Context,
		ed25519.PrivateKey,
		remoteproto.BrowserTokenRequest,
	) (remoteproto.GrantResponse, error)
}

func (h listenerHandler) registerBrowserRoutes(routes *http.ServeMux) {
	routes.HandleFunc("POST "+remoteproto.BrowserSessionPath, h.startBrowserSession)
	routes.HandleFunc("DELETE "+remoteproto.BrowserSessionPath, h.authenticateBrowser(h.endBrowserSession))
	routes.HandleFunc("GET "+remoteproto.BrowserPeersPath, h.authenticateBrowser(h.browserPeers))
	routes.HandleFunc("GET "+remoteproto.BrowserApprovalPath, h.browserApproval)
	routes.HandleFunc("POST "+remoteproto.BrowserStreamTicketsPath, h.authenticateBrowser(h.mintBrowserStreamTicket))
	operator := h.authenticateBrowser(h.serveOperator)
	routes.HandleFunc("/api", operator)
	routes.HandleFunc("/api/", operator)
	if h.hostedApp != nil {
		routes.Handle("GET "+remoteproto.BrowserCallbackPath, h.hostedApp)
	}
}

func (h listenerHandler) startBrowserSession(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.BrowserSessionRequest
	if !decodeBrowserRequest(w, req, &input) || input.Code == "" || input.CodeVerifier == "" {
		writeFailure(w, errInvalidBrowserRequest)
		return
	}
	if h.redeemBrowserCode == nil {
		writeFailure(w, ErrHostedUnavailable)
		return
	}
	proof := requestProof(req, ListenerHosted, req.Header.Get("DPoP"))
	jkt, err := remoteproto.BrowserDPoPKey(proof.Token, proof.Method, proof.URL, h.sessions.now())
	if err != nil {
		writeFailure(w, err)
		return
	}
	grant, err := h.redeemBrowserCode(
		req.Context(),
		remoteproto.BrowserTokenRequest{BrowserSessionRequest: input, JKT: jkt},
	)
	if err != nil {
		if failure, ok := errors.AsType[*cloudclient.Failure](err); ok {
			writeFailure(w, &Failure{failure.Status, failure.Code, failure.Message, nil})
			return
		}
		writeFailure(w, err)
		return
	}
	result, err := h.sessions.mintBrowser(req.Context(), grant.Grant, proof)
	if err != nil {
		writeFailure(w, err)
		return
	}
	http.SetCookie(w, browserCookie(result.Credential, int(SessionTTL/time.Second)))
	writeJSON(w, http.StatusCreated, remoteproto.BrowserSessionResponse{
		SessionID: result.SessionID, ExpiresAt: result.ExpiresAt, Scopes: result.Scopes,
	})
}

func (s *Sessions) mintBrowser(ctx context.Context, token string, proof DPoPProof) (MintResult, error) {
	grant, err := s.verifyMintGrant(ctx, token, s.now())
	if err != nil {
		return MintResult{}, err
	}
	if !grant.Browser || grant.Route != remoteproto.RouteCloud {
		return MintResult{}, ErrRouteMismatch
	}
	if _, err := s.proofs.VerifyDPoP(proof.Token, remoteproto.DPoPBinding{
		Method: proof.Method, URL: proof.URL, JKT: grant.Cnf.JKT,
	}, s.now()); err != nil {
		return MintResult{}, err
	}
	return s.mintWhenReady(ctx, grant)
}

func (h listenerHandler) authenticateBrowser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		cookie, err := req.Cookie(browserSessionCookie)
		if err != nil {
			writeFailure(w, ErrSessionInvalid)
			return
		}
		ticket, err := h.consumeBrowserStreamTicket(req, cookie.Value)
		if err != nil {
			writeFailure(w, err)
			return
		}
		proof := requestProof(req, ListenerHosted, req.Header.Get("DPoP"))
		proof.Browser = true
		p, err := h.sessions.authenticate(req.Context(), cookie.Value, proof, h.sessions.now(), ticket)
		if err != nil {
			writeFailure(w, err)
			return
		}
		mutation := req.Method != http.MethodGet && req.Method != http.MethodHead && req.Method != http.MethodOptions &&
			req.URL.Path != remoteproto.BrowserSessionPath && req.URL.Path != remoteproto.BrowserStreamTicketsPath
		ctx, release, err := h.sessions.BeginRequest(req.Context(), p, mutation)
		if err != nil {
			writeFailure(w, err)
			return
		}
		defer release()
		next(w, req.WithContext(ctx))
	}
}

func (h listenerHandler) endBrowserSession(w http.ResponseWriter, req *http.Request) {
	if err := h.revokeRequestSession(req); err != nil {
		writeFailure(w, err)
		return
	}
	http.SetCookie(w, browserCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func browserCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: browserSessionCookie, Value: value, Path: "/", Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: maxAge,
	}
}

func (h listenerHandler) browserPeers(w http.ResponseWriter, _ *http.Request) {
	peers := h.run.control.peers.Load()
	response := remoteproto.BrowserPeersResponse{Peers: []remoteproto.HostedPeer{}}
	if peers != nil {
		response.Peers = slices.Clone(*peers)
	}
	writeJSON(w, http.StatusOK, response)
}

func (h listenerHandler) browserApproval(w http.ResponseWriter, req *http.Request) {
	proof := requestProof(req, ListenerHosted, req.Header.Get("DPoP"))
	jkt, err := remoteproto.BrowserDPoPKey(proof.Token, proof.Method, proof.URL, h.sessions.now())
	if err != nil {
		writeFailure(w, err)
		return
	}
	status, err := h.sessions.browserApproval(req.Context(), req.PathValue("id"), jkt, proof)
	if err != nil {
		writeFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Sessions) browserApproval(
	ctx context.Context,
	id, jkt string,
	proof DPoPProof,
) (remoteproto.BrowserApprovalStatus, error) {
	s.serial.Lock()
	defer s.serial.Unlock()
	a, err := s.store.RemoteApprovalByID(ctx, id)
	if errors.Is(err, ErrMissing) || (err == nil && a.JKT != jkt) {
		return remoteproto.BrowserApprovalStatus{}, errBrowserApprovalNotFound
	}
	if err != nil {
		return remoteproto.BrowserApprovalStatus{}, err
	}
	if s.locallyRevoked[a.DeviceID] || !s.security.Snapshot().Allows(a.DeviceID, a.JKT, nil) {
		return remoteproto.BrowserApprovalStatus{}, ErrDeviceRevoked
	}
	now := s.now()
	if _, err := s.proofs.VerifyDPoP(proof.Token, remoteproto.DPoPBinding{
		Method: proof.Method, URL: proof.URL, JKT: a.JKT,
	}, now); err != nil {
		return remoteproto.BrowserApprovalStatus{}, err
	}
	if a.State == approvalPending && !now.Before(a.ExpiresAt) {
		a.State = approvalExpired
	}
	return remoteproto.BrowserApprovalStatus{ApprovalID: a.ID, State: a.State, ExpiresAt: a.ExpiresAt}, nil
}

func decodeBrowserRequest(w http.ResponseWriter, req *http.Request, output any) bool {
	return json.UnmarshalRead(http.MaxBytesReader(w, req.Body, 8<<10), output, json.RejectUnknownMembers(true)) == nil
}
