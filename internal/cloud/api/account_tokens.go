// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
)

func registerAccountTokenRoutes(mux *http.ServeMux, service *auth.Service) {
	h := authRoutes{service: service}
	mux.HandleFunc("POST "+remoteproto.AccountTokenExchangePath, h.exchangeAccountToken)
	mux.HandleFunc("GET /v1/account/tokens", h.accountTokens)
	mux.HandleFunc("POST /v1/account/tokens", h.accountTokens)
	mux.HandleFunc("DELETE /v1/account/tokens/{id}", h.accountTokens)
}

func (h authRoutes) exchangeAccountToken(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.AccountTokenExchange
	if !decodeAuthRequest(w, req, &input) {
		return
	}
	scheme, token, _ := strings.Cut(req.Header.Get(authorizationHeader), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		token = ""
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	out, err := h.service.ExchangeAccountToken(ctx, token, input)
	writeAuthResult(w, out, err)
}

func (h authRoutes) accountTokens(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := controlContext(req)
	defer cancel()
	p, err := h.service.AuthenticateDevice(req.WithContext(ctx))
	if err != nil {
		WriteAuthError(w, err)
		return
	}
	switch req.Method {
	case http.MethodGet:
		tokens, err := h.service.ListAccountTokens(ctx, p)
		writeAuthResult(w, remoteproto.AccountTokenList{Tokens: tokens}, err)
	case http.MethodPost:
		var input remoteproto.AccountTokenCreate
		if decodeAuthRequest(w, req, &input) {
			result, err := h.service.CreateAccountToken(ctx, p, input.Name)
			if err != nil {
				WriteAuthError(w, err)
				return
			}
			w.Header().Set(cacheControl, noStore)
			WriteJSON(w, http.StatusCreated, result)
		}
	case http.MethodDelete:
		writeAuthResult(w, nil, h.service.RevokeAccountToken(ctx, p, req.PathValue("id")))
	}
}

func rejectAccountTokenOutsideExchange(req *http.Request) bool {
	if req.Method == http.MethodPost && req.URL.EscapedPath() == remoteproto.AccountTokenExchangePath {
		return false
	}
	if !strings.HasPrefix(req.URL.Path, "/v1/") {
		return false
	}
	credential := strings.TrimSpace(req.Header.Get(authorizationHeader))
	_, value, _ := strings.Cut(credential, " ")
	return auth.IsAccountTokenCredential(credential) || auth.IsAccountTokenCredential(strings.TrimSpace(value))
}
