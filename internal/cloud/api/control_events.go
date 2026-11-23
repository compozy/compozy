package api

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const accountKeepAlive = 15 * time.Second

type accountEvent struct {
	ComputerID    remoteproto.ComputerID `json:"computer_id,omitempty"`
	DeviceID      remoteproto.DeviceID   `json:"device_id,omitempty"`
	SecurityEpoch uint64                 `json:"security_epoch,omitzero"`
	ApprovalID    string                 `json:"approval_id,omitempty"`
	Result        string                 `json:"result,omitempty"`
}

func (r *controlRoutes) accountEvents(w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal) {
	sub, ok := r.config.Hub.Subscribe(principal.AccountID, "")
	if !ok {
		writeUnavailable(w)
		return
	}
	defer r.config.Hub.Release(sub)
	ctx, cancel := controlContext(req)
	err := r.config.Service.CheckDevice(ctx, principal)
	cancel()
	if err != nil {
		r.fail(w, req, err)
		return
	}
	w.Header().Set(contentType, "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	if req.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := writeAccountFrame(w, ": connected\n\n"); err != nil {
		return
	}
	heartbeat := time.NewTicker(accountKeepAlive)
	defer heartbeat.Stop()
	for r.accountEventStep(w, req, principal, sub, heartbeat.C) {
	}
}

func (r *controlRoutes) accountEventStep(
	w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal,
	sub *control.Subscription, heartbeat <-chan time.Time,
) bool {
	select {
	case <-req.Context().Done():
		return false
	case <-sub.Closing:
		return false
	case <-heartbeat:
		ctx, cancel := controlContext(req)
		defer cancel()
		return r.config.Service.CheckDevice(ctx, principal) == nil && writeAccountFrame(w, ": keepalive\n\n") == nil
	case event := <-sub.Events:
		return r.onAccountEvent(w, req, principal, event) == nil
	}
}

func (r *controlRoutes) onAccountEvent(
	w http.ResponseWriter, req *http.Request, principal control.DevicePrincipal, event store.ControlEvent,
) error {
	ctx, cancel := controlContext(req)
	defer cancel()
	if err := r.config.Service.CheckDevice(ctx, principal); err != nil {
		return err
	}
	name, payload := accountEventPayload(event)
	if name == "" {
		return nil
	}
	if event.Type == remoteproto.ControlApprovalApplied {
		approval, err := r.config.Service.GetApproval(ctx, principal, payload.ApprovalID)
		if err != nil {
			return err
		}
		if approval.AppliedResult == nil {
			return nil
		}
		payload.Result = *approval.AppliedResult
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeAccountFrame(w, fmt.Sprintf("event: %s\ndata: %s\n\n", name, body))
}

func accountEventPayload(event store.ControlEvent) (string, accountEvent) {
	switch event.Type {
	case remoteproto.ControlSnapshot, remoteproto.ControlRevokeDevice, remoteproto.ControlScopesChanged,
		remoteproto.ControlDevicesChanged:
		return "devices.changed", accountEvent{
			DeviceID:      remoteproto.DeviceID(event.ResourceID),
			SecurityEpoch: event.SecurityEpoch,
		}
	case remoteproto.ControlUnlink, remoteproto.ControlComputersChanged:
		return "computers.changed", accountEvent{ComputerID: event.ComputerID, SecurityEpoch: event.SecurityEpoch}
	case remoteproto.ControlPlanChanged:
		return "plan.changed", accountEvent{}
	case remoteproto.ControlApprovalPending:
		return "approval.pending", accountEvent{ApprovalID: event.ResourceID, ComputerID: event.ComputerID}
	case remoteproto.ControlApprovalDecided:
		return "approval_decided", accountEvent{ApprovalID: event.ResourceID, ComputerID: event.ComputerID}
	case remoteproto.ControlApprovalApplied:
		return "approval.applied", accountEvent{ApprovalID: event.ResourceID, ComputerID: event.ComputerID}
	default:
		return "", accountEvent{}
	}
}

func writeAccountFrame(w http.ResponseWriter, frame string) error {
	response := http.NewResponseController(w)
	err := response.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := fmt.Fprint(w, frame); err != nil {
		return err
	}
	if err := response.Flush(); err != nil {
		return err
	}
	if err := response.SetWriteDeadline(time.Time{}); !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}
