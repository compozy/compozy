// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *controlRoutes) revalidateComputer(w http.ResponseWriter, req *http.Request) {
	var input remoteproto.ComputerRevalidation
	body, ok := r.decode(w, req, &input, true)
	if !ok {
		return
	}
	r.withComputerKey(
		w,
		req,
		body,
		func(req *http.Request, identity store.ControlComputer, proof control.ComputerRequestAuth) {
			err := r.config.Service.RevalidateComputer(req.Context(), identity, proof, input.InstallationID)
			r.respond(w, req, http.StatusNoContent, nil, revalidationFailure(err))
		},
	)
}

func (r *controlRoutes) unlinkComputer(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get(remoteproto.ComputerProofHeader) == "" {
		r.device(r.unlink).ServeHTTP(w, req)
		return
	}
	r.withComputerKey(
		w,
		req,
		nil,
		func(req *http.Request, identity store.ControlComputer, proof control.ComputerRequestAuth) {
			err := r.config.Service.UnlinkComputerWithKey(req.Context(), identity, proof)
			r.respond(w, req, http.StatusNoContent, nil, err)
		},
	)
}

func (r *controlRoutes) withComputerKey(
	w http.ResponseWriter, req *http.Request, body []byte,
	run func(*http.Request, store.ControlComputer, control.ComputerRequestAuth),
) {
	if req.Header.Get(authorizationHeader) != "" || req.Header.Get(remoteproto.ComputerProofHeader) == "" {
		r.fail(w, req, control.ErrInvalidInput)
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	identity, err := r.config.Service.ComputerIdentity(ctx, remoteproto.ComputerID(req.PathValue("id")))
	if err != nil {
		r.fail(w, req, revalidationFailure(err))
		return
	}
	proof, err := r.computerProof(req, identity.Key, body)
	if err != nil {
		r.fail(w, req, err)
		return
	}
	r.accountRequest(w, req, identity.AccountID, func(_ http.ResponseWriter, req *http.Request) {
		run(req.WithContext(ctx), identity, proof)
	})
}

func revalidationFailure(err error) error {
	if failure, ok := errors.AsType[*control.Error](err); ok {
		switch failure.Code {
		case "computer_unlinked", "account_suspended", "account_deleted":
			return &auth.Failure{Status: http.StatusUnauthorized, Code: "reauth_required",
				Reason: failure.Code, Message: failure.Message}
		}
	}
	if errors.Is(err, control.ErrUnauthorized) {
		return &auth.Failure{Status: http.StatusUnauthorized, Code: "reauth_required",
			Reason: "computer_unlinked", Message: "This computer was unlinked from your account."}
	}
	return err
}
