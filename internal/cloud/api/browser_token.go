// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *controlRoutes) browserToken(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(cacheControl, noStore)
	if req.Header.Get(authorizationHeader) != "" || req.Header.Get(remoteproto.ComputerProofHeader) == "" {
		r.fail(w, req, control.ErrProofInvalid)
		return
	}
	var input remoteproto.BrowserTokenRequest
	body, ok := r.decode(w, req, &input, false)
	if !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	computer, err := r.config.Service.BrowserCodeComputer(ctx, input)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	proof, err := r.computerProof(req, computer.Key, body)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	r.accountRequest(w, req, computer.AccountID, func(w http.ResponseWriter, req *http.Request) {
		grant, err := r.config.Service.RedeemBrowserCode(ctx, computer, proof, input)
		r.respond(w, req, http.StatusOK, grant, err)
	})
}
