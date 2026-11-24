// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *controlRoutes) computers(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	computers, err := r.config.Service.ListComputers(ctx, principal)
	r.respond(w, req, http.StatusOK, struct {
		Computers []control.Computer `json:"computers"`
	}{Computers: computers}, err)
}

func (r *controlRoutes) reserve(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var body struct {
		PreviousID string `json:"previous_reservation_id,omitempty"`
	}
	if _, ok := r.decode(w, req, &body, true); !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	reservation, err := r.config.Service.Reserve(ctx, principal, body.PreviousID)
	r.respond(w, req, http.StatusCreated, reservation, err)
}

func (r *controlRoutes) releaseReservation(
	w http.ResponseWriter,
	req *http.Request,
	principal control.DevicePrincipal,
) {
	ctx, cancel := controlContext(req)
	defer cancel()
	err := r.config.Service.ReleaseReservation(ctx, principal, req.PathValue("id"))
	r.respond(w, req, http.StatusNoContent, nil, err)
}

func (r *controlRoutes) linkToken(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var body struct {
		ReservationID string `json:"reservation_id,omitempty"`
	}
	if _, ok := r.decode(w, req, &body, true); !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	token, err := r.config.Service.CreateLinkToken(ctx, principal, body.ReservationID)
	r.respond(w, req, http.StatusCreated, token, err)
}

func (r *controlRoutes) link(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(cacheControl, noStore)
	var request struct {
		control.LinkRequest
		LinkToken string `json:"link_token,omitempty"`
	}
	raw, ok := r.decode(w, req, &request, false)
	if !ok {
		return
	}
	if token, err := linkCredential(req, request.LinkToken); err != nil {
		r.fail(w, req, err)
		return
	} else if token != "" {
		r.linkWithToken(w, req, token, request.LinkRequest)
		return
	}
	r.linkWithDevice(w, req, raw, request.LinkRequest)
}

func linkCredential(req *http.Request, bodyToken string) (string, error) {
	auth := req.Header.Get(authorizationHeader)
	if bodyToken != "" {
		if auth != "" || req.Header.Get(remoteproto.ComputerProofHeader) != "" {
			return "", control.ErrInvalidInput
		}
		return bodyToken, nil
	}
	scheme, token, _ := strings.Cut(auth, " ")
	if strings.EqualFold(scheme, "Bearer") && strings.HasPrefix(token, control.LinkTokenPrefix) {
		if req.Header.Get(remoteproto.ComputerProofHeader) != "" {
			return "", control.ErrInvalidInput
		}
		return token, nil
	}
	return "", nil
}

func (r *controlRoutes) linkWithToken(
	w http.ResponseWriter, req *http.Request, token string, request control.LinkRequest,
) {
	ctx, cancel := controlContext(req)
	defer cancel()
	account, err := r.config.Service.LinkTokenAccount(ctx, token)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	r.accountRequest(w, req, account, func(w http.ResponseWriter, req *http.Request) {
		computer, err := r.config.Service.LinkWithToken(ctx, account, token, request)
		r.linkResponse(w, req, computer, err)
	})
}

func (r *controlRoutes) linkWithDevice(
	w http.ResponseWriter, req *http.Request, raw []byte, request control.LinkRequest,
) {
	principal, err := r.authenticateDevice(req)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	proof, err := r.computerProof(req, request.ComputerJWK, raw)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	r.accountRequest(w, req, principal.AccountID, func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := controlContext(req)
		defer cancel()
		computer, err := r.config.Service.LinkWithDevice(ctx, principal, request, proof)
		r.linkResponse(w, req, computer, err)
	})
}

func (r *controlRoutes) linkResponse(w http.ResponseWriter, req *http.Request, computer control.Computer, err error) {
	r.respond(w, req, http.StatusOK, struct {
		ComputerID remoteproto.ComputerID `json:"computer_id"`
		AccountID  string                 `json:"account_id"`
	}{ComputerID: computer.ID, AccountID: computer.AccountID}, err)
}

func (r *controlRoutes) patchComputer(w http.ResponseWriter, req *http.Request) {
	var patch control.ComputerPatch
	body, ok := r.decode(w, req, &patch, false)
	if !ok {
		return
	}
	id := remoteproto.ComputerID(req.PathValue("id"))
	if req.Header.Get(remoteproto.ComputerProofHeader) == "" {
		handler := r.device(func(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
			ctx, cancel := controlContext(req)
			defer cancel()
			computer, err := r.config.Service.PatchComputer(ctx, principal, id, patch)
			r.respond(w, req, http.StatusOK, computer, err)
		})
		handler.ServeHTTP(w, req)
		return
	}
	r.patchComputerByKey(w, req, id, body, patch)
}

func (r *controlRoutes) patchComputerByKey(
	w http.ResponseWriter, req *http.Request, id remoteproto.ComputerID, body []byte, patch control.ComputerPatch,
) {
	if req.Header.Get(authorizationHeader) != "" {
		r.fail(w, req, control.ErrInvalidInput)
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	identity, err := r.config.Service.ComputerIdentity(ctx, id)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	proof, err := r.computerProof(req, identity.Key, body)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	r.accountRequest(w, req, identity.AccountID, func(w http.ResponseWriter, req *http.Request) {
		computer, err := r.config.Service.PatchComputerWithKey(ctx, identity, proof, patch)
		r.respond(w, req, http.StatusOK, computer, err)
	})
}

func (r *controlRoutes) computerProof(
	req *http.Request, jwk remoteproto.JWK, body []byte,
) (control.ComputerRequestAuth, error) {
	proof, err := remoteproto.VerifyComputerProof(
		req.Header.Get(remoteproto.ComputerProofHeader), jwk, req.Method, r.targetURL(req), body, r.now(),
	)
	if err != nil {
		return control.ComputerRequestAuth{}, control.ErrProofInvalid
	}
	jkt, err := jwk.Thumbprint()
	if err != nil {
		return control.ComputerRequestAuth{}, control.ErrProofInvalid
	}
	return control.ComputerRequestAuth{JKT: jkt, JTI: proof.JTI, IssuedAt: proof.IssuedAt}, nil
}

func (r *controlRoutes) unlink(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	err := r.config.Service.UnlinkComputer(ctx, principal, remoteproto.ComputerID(req.PathValue("id")))
	r.respond(w, req, http.StatusNoContent, nil, err)
}

func (r *controlRoutes) grant(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var request struct {
		Route remoteproto.RouteKind `json:"route"`
	}
	if _, ok := r.decode(w, req, &request, false); !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	grant, err := r.config.Service.IssueGrant(
		ctx,
		principal,
		remoteproto.ComputerID(req.PathValue("id")),
		request.Route,
	)
	r.respond(w, req, http.StatusOK, grant, err)
}
