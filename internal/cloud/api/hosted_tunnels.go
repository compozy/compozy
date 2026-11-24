// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *controlRoutes) provisionHostedTunnel(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.HostedTunnelRequest
	body, ok := r.decode(w, req, &input, true)
	if !ok {
		return
	}
	r.hostedComputerRequest(w, req, body, func(computer store.ControlComputer, proof control.ComputerRequestAuth) {
		tunnel, err := r.config.Service.ProvisionHostedTunnel(req.Context(), computer, proof, input)
		r.respond(w, req, http.StatusOK, tunnel, err)
	})
}

func (r *controlRoutes) deleteComputerResource(w http.ResponseWriter, req *http.Request) {
	if req.PathValue("resource") != "tunnel" {
		http.NotFound(w, req)
		return
	}
	body, err := readControlBody(w, req)
	if err != nil || len(body) > 0 {
		r.fail(w, req, control.ErrInvalidInput)
		return
	}
	r.hostedComputerRequest(w, req, body, func(computer store.ControlComputer, proof control.ComputerRequestAuth) {
		ctx, cancel := controlContext(req)
		defer cancel()
		err := r.config.Service.DeleteHostedTunnel(ctx, computer, proof)
		r.respond(w, req, http.StatusNoContent, nil, err)
	})
}

func (r *controlRoutes) hostedComputerRequest(
	w http.ResponseWriter, req *http.Request, body []byte,
	run func(store.ControlComputer, control.ComputerRequestAuth),
) {
	w.Header().Set(cacheControl, noStore)
	if req.Header.Get(authorizationHeader) != "" || req.Header.Get(remoteproto.ComputerProofHeader) == "" {
		r.fail(w, req, control.ErrProofInvalid)
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	identity, err := r.config.Service.ComputerIdentity(ctx, remoteproto.ComputerID(req.PathValue("id")))
	if err != nil {
		r.fail(w, req, err)
		return
	}
	proof, err := r.computerProof(req, identity.Key, body)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	r.accountRequest(w, req, identity.AccountID, func(http.ResponseWriter, *http.Request) {
		run(identity, proof)
	})
}
