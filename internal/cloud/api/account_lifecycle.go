// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *controlRoutes) patchAccountEmail(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var patch remoteproto.AccountEmailPatch
	if _, ok := r.decode(w, req, &patch, false); !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	account, err := r.config.Service.PatchAccountEmail(ctx, principal, patch)
	r.respond(w, req, http.StatusOK, account, err)
}

func (r *controlRoutes) deleteAccount(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var input remoteproto.AccountDeleteRequest
	if _, ok := r.decode(w, req, &input, false); !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	result, err := r.config.Service.DeleteAccount(ctx, principal, input.ConfirmEmail)
	r.respond(w, req, http.StatusAccepted, result, err)
}
