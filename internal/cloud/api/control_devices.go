// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *controlRoutes) devices(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	devices, err := r.config.Service.ListDevices(ctx, principal)
	r.respond(w, req, http.StatusOK, struct {
		Devices []remoteproto.AccountDevice `json:"devices"`
	}{Devices: devices}, err)
}

func (r *controlRoutes) getDevice(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	device, err := r.config.Service.GetDevice(ctx, principal, remoteproto.DeviceID(req.PathValue("id")))
	r.respond(w, req, http.StatusOK, device, err)
}

func (r *controlRoutes) patchDevice(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var patch remoteproto.AccountDevicePatch
	if _, ok := r.decode(w, req, &patch, false); !ok {
		return
	}
	ctx, cancel := controlContext(req)
	defer cancel()
	device, err := r.config.Service.PatchDevice(ctx, principal, remoteproto.DeviceID(req.PathValue("id")), patch)
	r.respond(w, req, http.StatusOK, device, err)
}

func (r *controlRoutes) revokeDevice(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	err := r.config.Service.RevokeDevice(ctx, principal, remoteproto.DeviceID(req.PathValue("id")))
	r.respond(w, req, http.StatusNoContent, nil, err)
}

func (r *controlRoutes) approvals(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	approvals, err := r.config.Service.ListApprovals(ctx, principal)
	r.respond(w, req, http.StatusOK, struct {
		Approvals []remoteproto.AccountApproval `json:"approvals"`
	}{Approvals: approvals}, err)
}

func (r *controlRoutes) getApproval(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	ctx, cancel := controlContext(req)
	defer cancel()
	approval, err := r.config.Service.GetApproval(ctx, principal, req.PathValue("id"))
	r.respond(w, req, http.StatusOK, approval, err)
}

func (r *controlRoutes) decideApproval(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	var decision remoteproto.ApprovalDecided
	if strings.TrimSpace(strings.SplitN(req.Header.Get(contentType), ";", 2)[0]) == "application/jose" {
		body, err := readControlBody(w, req)
		if err != nil {
			r.fail(w, req, control.ErrInvalidInput)
			return
		}
		decision.DecisionJWS = strings.TrimSpace(string(body))
	} else if _, ok := r.decode(w, req, &decision, false); !ok {
		return
	}
	id := req.PathValue("id")
	ctx, cancel := controlContext(req)
	defer cancel()
	err := r.config.Service.DecideApproval(ctx, principal, id, decision.DecisionJWS)
	r.respond(w, req, http.StatusAccepted, remoteproto.ApprovalDelivery{ApprovalID: id, State: "delivering"}, err)
}
