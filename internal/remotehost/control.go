package remotehost

import (
	"context"
	"crypto/ed25519"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	controlHandshake = 5 * time.Second
	controlWrite     = 2 * time.Second
	controlPing      = 20 * time.Second
)

type controlCloud interface {
	Origin() string
	OpenComputerControl(context.Context, remoteproto.ComputerID) (*websocket.Conn, error)
}

type controlLink struct {
	cloud     controlCloud
	id        remoteproto.ComputerID
	key       ed25519.PrivateKey
	logger    *slog.Logger
	sessions  *Sessions
	verifier  *remoteproto.JWSVerifier
	outgoing  chan remoteproto.ControlMessage
	connected atomic.Bool
	unlink    func(context.Context)
	onLease   func(context.Context)
	peers     atomic.Pointer[[]remoteproto.HostedPeer]
}

var errControlUnlinked = errors.New("computer unlinked by cloud")

func (c *controlLink) report(kind remoteproto.ControlType, epoch uint64, value any) {
	message, err := controlMessage(kind, epoch, value)
	if err != nil {
		c.logger.Warn("Could not encode remote control report", "type", kind)
		return
	}
	select {
	case c.outgoing <- message:
	default:
		c.logger.Warn("Remote control report queue is full", "type", kind, "computer_id", c.id)
	}
}

func (c *controlLink) run(ctx context.Context) {
	retry := 250 * time.Millisecond
	for ctx.Err() == nil {
		connected, err := c.connect(ctx)
		if errors.Is(err, errControlUnlinked) {
			return
		}
		if connected {
			retry = 250 * time.Millisecond
		}
		timer := time.NewTimer(max(retry, cloudclient.RetryDelay(c.sessions.now(), err)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		retry = min(retry*2, 10*time.Second)
	}
}

func (c *controlLink) connect(ctx context.Context) (bool, error) {
	socket, err := c.cloud.OpenComputerControl(ctx, c.id)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := socket.CloseNow(); err != nil && !errors.Is(err, net.ErrClosed) {
			c.logger.DebugContext(ctx, "Remote control link close interrupted")
		}
	}()
	if err := c.authenticate(ctx, socket); err != nil {
		return false, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var writer sync.WaitGroup
	writer.Go(func() {
		defer cancel()
		c.writeReports(ctx, socket)
	})
	defer func() {
		cancel()
		writer.Wait()
		c.connected.Store(false)
		c.peers.Store(nil)
	}()
	return c.readUpdates(ctx, socket)
}

func (c *controlLink) authenticate(ctx context.Context, socket *websocket.Conn) error {
	handshake, cancel := context.WithTimeout(ctx, controlHandshake)
	defer cancel()
	message, err := readControl(handshake, socket)
	if err != nil {
		return err
	}
	var challenge remoteproto.ComputerChallenge
	if message.Type != remoteproto.ControlChallenge || json.Unmarshal(message.Data, &challenge) != nil ||
		challenge.Nonce == "" {
		return errors.New("invalid computer control challenge")
	}
	signature := remoteproto.SignComputerChallenge(c.key, c.cloud.Origin(), c.id, challenge.Nonce)
	return sendControl(
		handshake,
		socket,
		remoteproto.ControlAuthenticate,
		0,
		remoteproto.ComputerAuthentication{Signature: signature},
	)
}

func (c *controlLink) readUpdates(ctx context.Context, socket *websocket.Conn) (bool, error) {
	var ready bool
	for {
		message, err := readControl(ctx, socket)
		if err != nil {
			return ready, err
		}
		if !ready && message.Type != remoteproto.ControlSnapshot {
			return false, remoteproto.ErrSnapshotInvalid
		}
		if err := c.applyControl(ctx, socket, message); err != nil {
			return ready, err
		}
		if !ready {
			if err := c.sessions.ReportApprovals(ctx); err != nil {
				return false, err
			}
			ready = true
			c.connected.Store(true)
		}
	}
}

func (c *controlLink) writeReports(ctx context.Context, socket *websocket.Conn) {
	pings := time.NewTicker(controlPing)
	defer pings.Stop()
	for {
		var err error
		select {
		case <-ctx.Done():
			return
		case <-pings.C:
			ping, cancel := context.WithTimeout(ctx, controlWrite)
			err = socket.Ping(ping)
			cancel()
		case report := <-c.outgoing:
			err = writeControl(ctx, socket, report)
		}
		if err != nil {
			return
		}
	}
}

func (c *controlLink) applyControl(
	ctx context.Context,
	socket *websocket.Conn,
	message remoteproto.ControlMessage,
) error {
	switch message.Type {
	case remoteproto.ControlSnapshot:
		return c.applySignedSnapshot(ctx, socket, message)
	case remoteproto.ControlLease:
		var signed string
		if err := json.Unmarshal(message.Data, &signed); err != nil {
			return err
		}
		lease, err := c.verifier.VerifyLease(ctx, signed, c.sessions.now())
		if err != nil {
			return err
		}
		if lease.SecurityEpoch != message.SecurityEpoch {
			return remoteproto.ErrLeaseInvalid
		}
		if err := c.sessions.ApplyLease(ctx, lease); err != nil {
			return err
		}
		if c.onLease != nil {
			c.onLease(ctx)
		}
		return nil
	case remoteproto.ControlPeers:
		var peers []remoteproto.HostedPeer
		if err := json.Unmarshal(message.Data, &peers); err != nil {
			return err
		}
		c.peers.Store(&peers)
		return nil
	case remoteproto.ControlRevokeDevice, remoteproto.ControlScopesChanged, remoteproto.ControlUnlink:
		return c.applyDelta(ctx, socket, message)
	case remoteproto.ControlJWKS:
		if message.SecurityEpoch != c.sessions.Snapshot().SecurityEpoch {
			return nil
		}
		var keys remoteproto.JWKS
		if err := json.Unmarshal(message.Data, &keys); err != nil {
			return err
		}
		return c.sessions.ApplySigningKeys(ctx, c.verifier, keys)
	case remoteproto.ControlApprovalDecided:
		if message.SecurityEpoch != c.sessions.Snapshot().SecurityEpoch {
			return nil
		}
		var approval remoteproto.ApprovalDecided
		if err := json.Unmarshal(message.Data, &approval); err != nil {
			return err
		}
		return c.sessions.ApplyApproval(ctx, approval.DecisionJWS)
	case remoteproto.ControlGoingAway:
		return errors.New("cloud control link draining")
	default:
		return nil
	}
}

func (c *controlLink) applySignedSnapshot(
	ctx context.Context,
	socket *websocket.Conn,
	message remoteproto.ControlMessage,
) error {
	var signed string
	if err := json.Unmarshal(message.Data, &signed); err != nil {
		return err
	}
	snapshot, err := c.verifier.VerifySnapshot(ctx, signed)
	if err != nil {
		return err
	}
	if snapshot.SecurityEpoch != message.SecurityEpoch {
		return remoteproto.ErrSnapshotInvalid
	}
	changed, err := c.sessions.ApplySnapshot(ctx, snapshot)
	if err != nil {
		return err
	}
	current := c.sessions.Snapshot()
	if current.SecurityEpoch != snapshot.SecurityEpoch {
		if !c.connected.Load() {
			return remoteproto.ErrSnapshotInvalid
		}
		return nil
	}
	if err := c.finishSecurity(ctx, socket, current); err != nil {
		return err
	}
	if changed && c.connected.Load() {
		return c.sessions.ReportApprovals(ctx)
	}
	return nil
}

func (c *controlLink) applyDelta(
	ctx context.Context,
	socket *websocket.Conn,
	message remoteproto.ControlMessage,
) error {
	changed, request, err := c.sessions.ApplyIncremental(ctx, message)
	if err != nil {
		return err
	}
	if request != nil {
		return sendControl(ctx, socket, remoteproto.ControlSnapshotRequest, request.AppliedEpoch, request)
	}
	if !changed {
		return nil
	}
	if err := c.finishSecurity(ctx, socket, c.sessions.Snapshot()); err != nil {
		return err
	}
	return c.sessions.ReportApprovals(ctx)
}

func (c *controlLink) finishSecurity(
	ctx context.Context,
	socket *websocket.Conn,
	snapshot remoteproto.SecuritySnapshot,
) error {
	ack := remoteproto.SecurityAck{SecurityEpoch: snapshot.SecurityEpoch}
	if !snapshot.Linked {
		c.unlink(ctx)
		if err := sendControl(ctx, socket, remoteproto.ControlAck, snapshot.SecurityEpoch, ack); err != nil {
			c.logger.DebugContext(ctx, "Could not acknowledge remote unlink", "computer_id", c.id)
		}
		return errControlUnlinked
	}
	return sendControl(ctx, socket, remoteproto.ControlAck, snapshot.SecurityEpoch, ack)
}

func readControl(ctx context.Context, socket *websocket.Conn) (remoteproto.ControlMessage, error) {
	var message remoteproto.ControlMessage
	kind, data, err := socket.Read(ctx)
	if err != nil {
		return message, err
	}
	if kind != websocket.MessageText {
		return message, errors.New("invalid computer control message type")
	}
	err = json.Unmarshal(data, &message)
	return message, err
}

func sendControl(
	ctx context.Context,
	socket *websocket.Conn,
	kind remoteproto.ControlType,
	epoch uint64,
	value any,
) error {
	message, err := controlMessage(kind, epoch, value)
	if err != nil {
		return err
	}
	return writeControl(ctx, socket, message)
}

func controlMessage(kind remoteproto.ControlType, epoch uint64, value any) (remoteproto.ControlMessage, error) {
	data, err := json.Marshal(value)
	return remoteproto.ControlMessage{Type: kind, SecurityEpoch: epoch, Data: data}, err
}

func writeControl(ctx context.Context, socket *websocket.Conn, message remoteproto.ControlMessage) error {
	frame, err := json.Marshal(message)
	if err != nil {
		return err
	}
	write, cancel := context.WithTimeout(ctx, controlWrite)
	defer cancel()
	return socket.Write(write, websocket.MessageText, frame)
}

func (h *Host) HostedPeers() []remoteproto.HostedPeer {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.run == nil || h.run.hostedEntitlement(h.run.hostedView()) != remoteproto.HostedTunnelActive {
		return nil
	}
	if peers := h.run.control.peers.Load(); peers != nil {
		return slices.Clone(*peers)
	}
	return nil
}
