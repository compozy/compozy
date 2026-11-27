// SPDX-License-Identifier: FSL-1.1-MIT

package api

import (
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	controlHandshakeTimeout = 5 * time.Second
	controlWriteTimeout     = 2 * time.Second
	controlPingInterval     = 20 * time.Second
	controlReadTimeout      = 45 * time.Second
)

var errLinkUnlinked = errors.New("computer unlinked")

type computerControlLink struct {
	routes    *controlRoutes
	link      control.Link
	socket    *websocket.Conn
	sentEpoch uint64
	ackEpoch  uint64
}

func (r *controlRoutes) controlLink(w http.ResponseWriter, req *http.Request) {
	if !r.config.Hub.Ready() {
		writeUnavailable(w)
		return
	}
	id := remoteproto.ComputerID(req.URL.Query().Get("computer_id"))
	if id == "" || len(id) > 128 {
		r.fail(w, req, control.ErrInvalidInput)
		return
	}
	ctx, cancel := controlContext(req)
	identity, err := r.config.Service.ComputerIdentity(ctx, id)
	cancel()
	if err != nil {
		r.fail(w, req, err)
		return
	}
	upgrader := websocket.Upgrader{HandshakeTimeout: controlHandshakeTimeout}
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}
	defer socket.Close()
	link := &computerControlLink{routes: r, socket: socket}
	if !link.authenticate(identity) {
		return
	}
	link.serve(req.Context(), identity)
}

func (p *computerControlLink) authenticate(identity store.ControlComputer) bool {
	p.socket.SetReadLimit(controlRequestLimit)
	if err := p.socket.SetReadDeadline(time.Now().Add(controlHandshakeTimeout)); err != nil {
		return false
	}
	nonce := rand.Text()
	if err := p.send(remoteproto.ControlChallenge, 0, remoteproto.ComputerChallenge{Nonce: nonce}); err != nil {
		return false
	}
	message, err := readControlMessage(p.socket)
	if err != nil || message.Type != remoteproto.ControlAuthenticate {
		return false
	}
	var proof remoteproto.ComputerAuthentication
	return json.Unmarshal(message.Data, &proof) == nil && remoteproto.VerifyComputerChallenge(
		identity.Key, p.routes.config.Issuer, identity.ID, nonce, proof.Signature,
	)
}

func (p *computerControlLink) serve(ctx context.Context, identity store.ControlComputer) {
	hub := p.routes.config.Hub
	subscription, ok := hub.Subscribe(identity.AccountID, identity.ID)
	if !ok {
		p.sendControlGoingAway()
		return
	}
	defer hub.Release(subscription)
	open, cancel := context.WithTimeout(ctx, controlRequestTimeout)
	link, err := p.routes.config.Service.OpenLink(open, identity)
	cancel()
	if err != nil {
		return
	}
	p.link = link
	defer p.closeLink(ctx)
	if err := p.refreshSecurity(ctx, store.ControlEvent{}, true); err != nil {
		return
	}
	p.run(ctx, subscription)
}

func (p *computerControlLink) closeLink(ctx context.Context) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), controlWriteTimeout)
	defer cancel()
	if err := p.routes.config.Service.CloseLink(cleanup, p.link); err != nil {
		p.routes.config.Logger.WarnContext(
			cleanup,
			"Cloud control link cleanup failed",
			"computer_id",
			p.link.ComputerID,
		)
	}
}

func (p *computerControlLink) run(ctx context.Context, subscription *control.Subscription) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := p.socket.SetReadDeadline(time.Now().Add(controlReadTimeout)); err != nil {
		return
	}
	p.socket.SetPongHandler(func(string) error {
		return p.socket.SetReadDeadline(time.Now().Add(controlReadTimeout))
	})
	incoming := make(chan remoteproto.ControlMessage)
	var reader sync.WaitGroup
	reader.Go(func() { p.readMessages(ctx, incoming) })
	defer func() {
		cancel()
		if err := p.socket.Close(); err != nil {
			p.routes.config.Logger.DebugContext(ctx, "Cloud control socket was already closed")
		}
		reader.Wait()
	}()
	pings := time.NewTicker(controlPingInterval)
	defer pings.Stop()
	leases := time.NewTicker(control.LeaseRefresh)
	defer leases.Stop()
	for {
		if err := p.runStep(ctx, subscription, incoming, pings.C, leases.C); err != nil {
			return
		}
	}
}

func (p *computerControlLink) runStep(
	ctx context.Context, subscription *control.Subscription, incoming <-chan remoteproto.ControlMessage,
	pings, leases <-chan time.Time,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case reason := <-subscription.Closing:
		if reason == control.GoingAway {
			p.sendControlGoingAway()
		}
		return context.Canceled
	case message, ok := <-incoming:
		if !ok {
			return context.Canceled
		}
		return p.handleMessage(ctx, message)
	case event := <-subscription.Events:
		return p.handleEvent(ctx, event)
	case <-pings:
		return p.socket.WriteControl(websocket.PingMessage, nil, time.Now().Add(controlWriteTimeout))
	case <-leases:
		return p.refreshLease(ctx)
	}
}

func (p *computerControlLink) sendControlGoingAway() {
	if err := p.send(remoteproto.ControlGoingAway, 0, nil); err != nil {
		return
	}
	if err := p.socket.WriteControl(
		websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, string(control.GoingAway)),
		time.Now().Add(controlWriteTimeout),
	); err != nil {
		p.routes.config.Logger.Debug("Cloud control link closed during drain")
	}
}

func (p *computerControlLink) readMessages(ctx context.Context, messages chan<- remoteproto.ControlMessage) {
	defer close(messages)
	for {
		message, err := readControlMessage(p.socket)
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case messages <- message:
		}
	}
}

func readControlMessage(socket *websocket.Conn) (remoteproto.ControlMessage, error) {
	var message remoteproto.ControlMessage
	kind, data, err := socket.ReadMessage()
	if err != nil {
		return message, err
	}
	if kind != websocket.TextMessage {
		return message, control.ErrInvalidInput
	}
	if err := json.Unmarshal(data, &message); err != nil {
		return message, err
	}
	return message, nil
}

func (p *computerControlLink) send(kind remoteproto.ControlType, epoch uint64, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	frame, err := json.Marshal(remoteproto.ControlMessage{Type: kind, SecurityEpoch: epoch, Data: data})
	if err != nil {
		return err
	}
	if err := p.socket.SetWriteDeadline(time.Now().Add(controlWriteTimeout)); err != nil {
		return err
	}
	return p.socket.WriteMessage(websocket.TextMessage, frame)
}

func (p *computerControlLink) handleMessage(ctx context.Context, message remoteproto.ControlMessage) error {
	switch message.Type {
	case remoteproto.ControlAck:
		var ack remoteproto.SecurityAck
		if err := json.Unmarshal(message.Data, &ack); err != nil || ack.SecurityEpoch == 0 ||
			ack.SecurityEpoch > p.sentEpoch {
			return control.ErrInvalidInput
		}
		if ack.SecurityEpoch != p.sentEpoch || ack.SecurityEpoch <= p.ackEpoch {
			return nil
		}
		p.ackEpoch = ack.SecurityEpoch
		return p.afterAck(ctx)
	case remoteproto.ControlSnapshotRequest:
		var request remoteproto.SnapshotRequest
		if err := json.Unmarshal(message.Data, &request); err != nil {
			return control.ErrInvalidInput
		}
		return p.refreshSecurity(ctx, store.ControlEvent{}, true)
	default:
		ctx, cancel := context.WithTimeout(ctx, controlRequestTimeout)
		defer cancel()
		return p.routes.config.Service.AcceptLinkReport(ctx, p.link, message)
	}
}

func (p *computerControlLink) handleEvent(ctx context.Context, event store.ControlEvent) error {
	switch event.Type {
	case remoteproto.ControlSnapshot, remoteproto.ControlRevokeDevice, remoteproto.ControlScopesChanged,
		remoteproto.ControlUnlink, remoteproto.ControlLinkReplaced, remoteproto.ControlApprovalDecided:
		return p.refreshSecurity(ctx, event, false)
	case remoteproto.ControlComputersChanged:
		return p.refreshLease(ctx)
	case remoteproto.ControlPlanChanged:
		if err := p.refreshLease(ctx); err != nil {
			return err
		}
		return p.refreshPeers(ctx)
	case remoteproto.ControlJWKS:
		return p.publishKeys(ctx, false)
	case remoteproto.ControlPeers:
		return p.refreshPeers(ctx)
	default:
		return nil
	}
}

func (p *computerControlLink) refreshSecurity(ctx context.Context, event store.ControlEvent, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, controlRequestTimeout)
	defer cancel()
	service := p.routes.config.Service
	snapshot, err := service.LinkSnapshot(ctx, p.link)
	if err != nil {
		return err
	}
	if !force && snapshot.SecurityEpoch <= p.sentEpoch {
		return nil
	}
	token, err := service.SignedSnapshot(snapshot)
	if err != nil {
		return err
	}
	if p.sentEpoch > 0 && event.SecurityEpoch == snapshot.SecurityEpoch && snapshot.SecurityEpoch == p.sentEpoch+1 {
		if err := p.sendDelta(event, snapshot); err != nil {
			return err
		}
	}
	if err := p.send(remoteproto.ControlSnapshot, snapshot.SecurityEpoch, token); err != nil {
		return err
	}
	p.sentEpoch = snapshot.SecurityEpoch
	if !snapshot.Linked {
		return errLinkUnlinked
	}
	return nil
}

func (p *computerControlLink) sendDelta(event store.ControlEvent, snapshot remoteproto.SecuritySnapshot) error {
	switch event.Type {
	case remoteproto.ControlRevokeDevice:
		return p.send(event.Type, event.SecurityEpoch, remoteproto.RevokeDevice{
			DeviceID: remoteproto.DeviceID(event.ResourceID),
		})
	case remoteproto.ControlScopesChanged:
		index := slices.IndexFunc(snapshot.Devices, func(device remoteproto.SnapshotDevice) bool {
			return string(device.DeviceID) == event.ResourceID
		})
		if index >= 0 {
			device := snapshot.Devices[index]
			return p.send(
				event.Type,
				event.SecurityEpoch,
				remoteproto.ScopesChanged{DeviceID: device.DeviceID, Scopes: device.Scopes},
			)
		}
	case remoteproto.ControlUnlink:
		if event.ComputerID == p.link.ComputerID && !snapshot.Linked {
			return p.send(event.Type, event.SecurityEpoch, nil)
		}
	}
	return nil
}

func (p *computerControlLink) refreshLease(ctx context.Context) error {
	if p.ackEpoch == 0 || p.ackEpoch != p.sentEpoch {
		return nil
	}
	read, cancel := context.WithTimeout(ctx, controlRequestTimeout)
	defer cancel()
	started := time.Now()
	token, err := p.routes.config.Service.LinkLease(read, p.link, p.ackEpoch)
	if errors.Is(err, control.ErrSnapshotNeeded) {
		return p.refreshSecurity(ctx, store.ControlEvent{}, false)
	}
	if err != nil || token == "" {
		return err
	}
	if err := p.send(remoteproto.ControlLease, p.ackEpoch, token); err != nil {
		return err
	}
	p.routes.metrics.Duration("cloud_lease_push_lag_seconds", time.Since(started))
	return nil
}

func (p *computerControlLink) refreshPeers(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, controlRequestTimeout)
	defer cancel()
	peers, err := p.routes.config.Service.LinkPeers(ctx, p.link)
	if err != nil {
		return err
	}
	return p.send(remoteproto.ControlPeers, 0, peers)
}

func (p *computerControlLink) publishKeys(ctx context.Context, retiredOnly bool) error {
	read, cancel := context.WithTimeout(ctx, controlRequestTimeout)
	defer cancel()
	keys, err := p.routes.currentKeys(read)
	if err != nil || (retiredOnly && len(keys.Retired) == 0) {
		return err
	}
	return p.send(remoteproto.ControlJWKS, p.sentEpoch, keys)
}

func (p *computerControlLink) afterAck(ctx context.Context) error {
	if err := p.publishKeys(ctx, true); err != nil {
		return err
	}
	if err := p.refreshLease(ctx); err != nil || p.ackEpoch != p.sentEpoch {
		return err
	}
	if err := p.refreshPeers(ctx); err != nil {
		return err
	}
	read, cancel := context.WithTimeout(ctx, controlRequestTimeout)
	defer cancel()
	envelopes, err := p.routes.config.Service.LinkApprovals(read, p.link)
	if err != nil {
		return err
	}
	for _, envelope := range envelopes {
		if err := p.send(
			remoteproto.ControlApprovalDecided,
			p.ackEpoch,
			remoteproto.ApprovalDecided{DecisionJWS: envelope},
		); err != nil {
			return err
		}
	}
	return nil
}
