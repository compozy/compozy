package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	windowManagerWriteTimeout    = 10 * time.Second
	windowManagerPingInterval    = 30 * time.Second
	windowManagerPongTimeout     = 60 * time.Second
	windowManagerEvictionTimeout = 2 * time.Second
	windowManagerMaxMessageBytes = 64 << 10
)

var (
	windowManagerUpgrader         = websocket.Upgrader{HandshakeTimeout: windowManagerWriteTimeout}
	errWindowManagerClientMessage = errors.New("window manager stream client frame is invalid")
)

// StreamWindowManager upgrades one HTTP or UDS request to a snapshot-fenced WebSocket stream.
func (h *BaseHandlers) StreamWindowManager(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerService(c, workspaceID)
	if !ok {
		return
	}
	after, err := windowManagerAfterRevision(c)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	clientID := windowManagerStreamClientID(c)
	stop, done, started := h.windowManagerStreams.begin()
	if !started {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	defer done()
	subscription, err := service.Subscribe(
		c.Request.Context(),
		windowmanager.SubscriptionRequest{
			WorkspaceID: workspaceID, AfterRevision: after, ClientID: clientID,
		},
	)
	if err != nil {
		if websocket.IsWebSocketUpgrade(c.Request) {
			terminalErr := writeWindowManagerPreflightError(c, workspaceID, err)
			if terminalErr != nil && h.Logger != nil {
				h.Logger.Debug(
					"window-manager websocket terminal preflight failed",
					"workspace_id",
					workspaceID,
					"error",
					terminalErr,
				)
			}
			return
		}
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	conn, err := windowManagerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		closeErr := subscription.Close()
		if h.Logger != nil {
			h.Logger.Warn(
				"window-manager websocket upgrade failed",
				"workspace_id",
				workspaceID,
				"error",
				errors.Join(err, closeErr),
			)
		}
		return
	}
	var clientCommands windowmanager.ClientCommandConnection
	if clientID != nil {
		clientCommands, err = service.AttachClientCommands(c.Request.Context(), workspaceID, *clientID)
		if err != nil {
			socket := windowManagerSocket{conn: conn, subscription: subscription, stop: stop, workspaceID: workspaceID}
			terminalErr := socket.cleanup(nil, errors.Join(err, socket.writeError(err)), false)
			if terminalErr != nil && h.Logger != nil {
				h.Logger.Debug("window-manager client command attachment failed", "error", terminalErr)
			}
			return
		}
	}
	socket := windowManagerSocket{
		conn: conn, subscription: subscription, clientCommands: clientCommands,
		stop: stop, workspaceID: workspaceID, clientID: clientID, manager: service,
		pingInterval: h.streamPingInterval(),
	}
	err = socket.run(c.Request.Context())
	if err != nil && !isExpectedWindowManagerSocketError(err) && h.Logger != nil {
		h.Logger.Debug("window-manager websocket closed with error", "workspace_id", workspaceID, "error", err)
	}
}

func writeWindowManagerPreflightError(
	c *gin.Context,
	workspaceID windowmanager.WorkspaceID,
	streamErr error,
) error {
	conn, err := windowManagerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return fmt.Errorf("upgrade window-manager websocket for terminal preflight error: %w", err)
	}
	socket := windowManagerSocket{conn: conn, workspaceID: workspaceID}
	writeErr := socket.writeError(streamErr)
	closeErr := conn.Close()
	if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
		closeErr = fmt.Errorf("close window-manager preflight websocket: %w", closeErr)
	}
	return errors.Join(writeErr, closeErr)
}

type windowManagerSocket struct {
	conn           *websocket.Conn
	subscription   windowmanager.Subscription
	clientCommands windowmanager.ClientCommandConnection
	stop           <-chan struct{}
	workspaceID    windowmanager.WorkspaceID
	clientID       *windowmanager.ClientID
	manager        WindowManagerService
	// revision is the newest topology revision this socket has announced; the
	// heartbeat repeats it so a client can notice a frame it never received.
	revision     contract.WindowManagerRevision
	pingInterval time.Duration
}

func (h *BaseHandlers) streamPingInterval() time.Duration {
	if h != nil && h.windowManagerPingInterval > 0 {
		return h.windowManagerPingInterval
	}
	return windowManagerPingInterval
}

func (s *windowManagerSocket) run(ctx context.Context) error {
	if err := s.configureReader(); err != nil {
		return s.cleanup(nil, err, false)
	}
	if err := s.writeInitialFence(); err != nil {
		return s.cleanup(nil, err, false)
	}

	readDone := make(chan error, 1)
	go func() {
		readDone <- s.readPump(ctx)
	}()
	interval := s.pingInterval
	if interval <= 0 {
		interval = windowManagerPingInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var clientCommandUpdates <-chan windowmanager.ClientCommand
	var clientCommandsDone <-chan struct{}
	if s.clientCommands != nil {
		clientCommandUpdates = s.clientCommands.Commands()
		clientCommandsDone = s.clientCommands.Done()
	}

	return s.runEventLoop(ctx, readDone, ticker.C, clientCommandUpdates, clientCommandsDone)
}

func (s *windowManagerSocket) writeInitialFence() error {
	fence := s.subscription.Fence()
	snapshot, err := contract.WindowManagerSnapshotFromDomain(fence.Snapshot)
	if err != nil {
		encodeErr := fmt.Errorf("encode window-manager stream snapshot: %w", err)
		return errors.Join(encodeErr, s.writeError(encodeErr))
	}
	var client *contract.WindowManagerClientView
	if fence.Client != nil {
		converted, convertErr := contract.WindowManagerClientFromDomain(*fence.Client)
		if convertErr != nil {
			encodeErr := fmt.Errorf("encode window-manager stream client fence: %w", convertErr)
			return errors.Join(encodeErr, s.writeError(encodeErr))
		}
		client = &converted
	}
	s.revision = snapshot.Revision
	if err := s.writeJSON(contract.WindowManagerSnapshotFrame{
		Type: contract.WindowManagerFrameSnapshot, WorkspaceID: s.workspaceID,
		Revision: snapshot.Revision, Snapshot: snapshot, Client: client,
	}); err != nil {
		return err
	}
	return nil
}

func (s *windowManagerSocket) runEventLoop(
	ctx context.Context,
	readDone <-chan error,
	pings <-chan time.Time,
	clientCommandUpdates <-chan windowmanager.ClientCommand,
	clientCommandsDone <-chan struct{},
) error {
	var readObserved bool
	for {
		select {
		case update, ok := <-s.subscription.Updates():
			if !ok {
				return s.cleanup(readDone, s.subscriptionError(), readObserved)
			}
			if writeErr := s.writeSubscriptionUpdate(update); writeErr != nil {
				return s.cleanup(readDone, writeErr, readObserved)
			}
		case command := <-clientCommandUpdates:
			if writeErr := s.writeJSON(contract.WindowManagerClientCommandFrame{
				Type: contract.WindowManagerFrameClientCommand, WorkspaceID: s.workspaceID,
				CommandID: command.CommandID, Op: command.Op, Payload: command.Payload,
			}); writeErr != nil {
				return s.cleanup(readDone, writeErr, readObserved)
			}
		case <-clientCommandsDone:
			return s.cleanup(readDone, s.clientCommandError(), readObserved)
		case readErr := <-readDone:
			readObserved = true
			return s.cleanup(readDone, s.decorateReadError(readErr), readObserved)
		case <-pings:
			if pingErr := s.writePing(); pingErr != nil {
				return s.cleanup(readDone, pingErr, readObserved)
			}
			if heartbeatErr := s.writeHeartbeat(); heartbeatErr != nil {
				return s.cleanup(readDone, heartbeatErr, readObserved)
			}
		case <-s.stop:
			return s.cleanup(readDone, s.writeClose(websocket.CloseGoingAway, "daemon shutdown"), readObserved)
		case <-ctx.Done():
			return s.cleanup(readDone, ctx.Err(), readObserved)
		}
	}
}

type encodedWindowManagerUpdate struct {
	event  *contract.WindowManagerEventFrame
	client *contract.WindowManagerClientFrame
}

func encodeWindowManagerUpdate(
	workspaceID windowmanager.WorkspaceID,
	update windowmanager.SubscriptionUpdate,
) (encodedWindowManagerUpdate, error) {
	switch {
	case update.Event != nil && update.Client == nil:
		payload, err := contract.WindowManagerEventFromDomain(*update.Event)
		if err != nil {
			return encodedWindowManagerUpdate{}, fmt.Errorf("encode window-manager event update: %w", err)
		}
		return encodedWindowManagerUpdate{
			event: &contract.WindowManagerEventFrame{
				Type: contract.WindowManagerFrameEvent, WorkspaceID: workspaceID,
				Revision: payload.Revision, Event: payload,
			},
		}, nil
	case update.Client != nil && update.Event == nil:
		payload, err := contract.WindowManagerClientFromDomain(*update.Client)
		if err != nil {
			return encodedWindowManagerUpdate{}, fmt.Errorf("encode window-manager client update: %w", err)
		}
		return encodedWindowManagerUpdate{
			client: &contract.WindowManagerClientFrame{
				Type: contract.WindowManagerFrameClient, WorkspaceID: workspaceID,
				Revision: payload.PresentationRevision, Client: payload,
			},
		}, nil
	default:
		return encodedWindowManagerUpdate{}, errors.New(
			"window-manager subscription update must contain exactly one payload",
		)
	}
}

func (update encodedWindowManagerUpdate) write(socket *windowManagerSocket) error {
	if update.event != nil {
		if update.event.Revision > socket.revision {
			socket.revision = update.event.Revision
		}
		return socket.writeJSON(*update.event)
	}
	if update.client != nil {
		return socket.writeJSON(*update.client)
	}
	return errors.New("window-manager encoded update has no payload")
}

func (s *windowManagerSocket) configureReader() error {
	s.conn.SetReadLimit(windowManagerMaxMessageBytes)
	if err := s.conn.SetReadDeadline(time.Now().Add(windowManagerPongTimeout)); err != nil {
		return fmt.Errorf("set window-manager read deadline: %w", err)
	}
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(windowManagerPongTimeout))
	})
	return nil
}

func (s *windowManagerSocket) readPump(ctx context.Context) error {
	for {
		messageType, payload, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			if s.clientCommands == nil {
				return errWindowManagerClientMessage
			}
			if err := s.resolveClientFrame(ctx, payload); err != nil {
				return err
			}
		}
	}
}

func (s *windowManagerSocket) resolveClientFrame(ctx context.Context, payload []byte) error {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode window-manager client frame envelope: %v: %w", err, errWindowManagerClientMessage)
	}
	switch envelope.Type {
	case contract.WindowManagerFrameClientCommandAck:
		var frame contract.WindowManagerClientCommandAckFrame
		if err := decodeWindowManagerClientFrame(payload, &frame); err != nil {
			return err
		}
		return s.clientCommands.Resolve(windowmanager.ClientCommandResponse{
			CommandID: frame.CommandID,
			Status:    windowmanager.ClientCommandAcknowledged,
		})
	case contract.WindowManagerFrameClientCommandResult:
		var frame contract.WindowManagerClientCommandResultFrame
		if err := decodeWindowManagerClientFrame(payload, &frame); err != nil {
			return err
		}
		status := windowmanager.ClientCommandCompleted
		if frame.Error != "" {
			status = windowmanager.ClientCommandFailed
		}
		return s.clientCommands.Resolve(windowmanager.ClientCommandResponse{
			CommandID: frame.CommandID,
			Status:    status,
			Result:    frame.Result,
			Error:     frame.Error,
		})
	case contract.WindowManagerFrameClientContext:
		var frame contract.WindowManagerClientContextFrame
		if err := decodeWindowManagerClientFrame(payload, &frame); err != nil {
			return err
		}
		if s.manager == nil || s.clientID == nil {
			return errWindowManagerClientMessage
		}
		_, err := s.manager.UpdateClientContext(ctx, windowmanager.ClientContextUpdate{
			WorkspaceID: s.workspaceID,
			ClientID:    *s.clientID,
			Context: windowmanager.ClientContextInput{
				ScopeGlobal:         frame.Context.ScopeGlobal,
				FocusedSessionState: frame.Context.FocusedSessionState,
				WorkspaceTrusted:    frame.Context.WorkspaceTrusted,
				DestinationIntent:   frame.Context.DestinationIntent,
				GlobalShortcuts: windowmanager.CloneGlobalShortcutRegistrations(
					frame.Context.GlobalShortcuts,
				),
			},
		})
		if err != nil {
			return fmt.Errorf("update window-manager client context: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown client frame type %q: %w", envelope.Type, errWindowManagerClientMessage)
	}
}

func decodeWindowManagerClientFrame(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode window-manager client frame: %v: %w", err, errWindowManagerClientMessage)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf(
			"decode window-manager client frame trailing data: %v: %w",
			err,
			errWindowManagerClientMessage,
		)
	}
	return nil
}

func (s *windowManagerSocket) writeJSON(payload any) error {
	if err := s.conn.SetWriteDeadline(time.Now().Add(windowManagerWriteTimeout)); err != nil {
		return fmt.Errorf("set window-manager write deadline: %w", err)
	}
	if err := s.conn.WriteJSON(payload); err != nil {
		return fmt.Errorf("write window-manager frame: %w", err)
	}
	return nil
}

// writeHeartbeat follows every protocol ping with an application frame: browsers
// cannot observe pings, so this is the only liveness signal the web client sees.
func (s *windowManagerSocket) writeHeartbeat() error {
	return s.writeJSON(contract.WindowManagerHeartbeatFrame{
		Type: contract.WindowManagerFrameHeartbeat, WorkspaceID: s.workspaceID, Revision: s.revision,
	})
}

func (s *windowManagerSocket) writePing() error {
	deadline := time.Now().Add(windowManagerWriteTimeout)
	if err := s.conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set window-manager ping deadline: %w", err)
	}
	if err := s.conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
		return fmt.Errorf("write window-manager ping: %w", err)
	}
	return nil
}

func (s *windowManagerSocket) writeError(err error) error {
	_, payload := windowManagerErrorPayload(s.workspaceID, err)
	deadline := time.Now().Add(windowManagerEvictionTimeout)
	if deadlineErr := s.conn.SetWriteDeadline(deadline); deadlineErr != nil {
		return fmt.Errorf("set window-manager error deadline: %w", deadlineErr)
	}
	frameErr := s.conn.WriteJSON(contract.WindowManagerErrorFrame{
		Type:  contract.WindowManagerFrameError,
		Error: payload,
	})
	closeErr := s.writeClose(websocket.ClosePolicyViolation, string(payload.Code))
	if frameErr != nil {
		frameErr = fmt.Errorf("write window-manager error frame: %w", frameErr)
	}
	return errors.Join(frameErr, closeErr)
}

func (s *windowManagerSocket) writeClose(code int, reason string) error {
	deadline := time.Now().Add(windowManagerEvictionTimeout)
	err := s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("write window-manager close: %w", err)
	}
	return nil
}

func (s *windowManagerSocket) cleanup(readDone <-chan error, runErr error, readObserved bool) error {
	closeSubscriptionErr := s.subscription.Close()
	var closeClientCommandsErr error
	if s.clientCommands != nil {
		closeClientCommandsErr = s.clientCommands.Close()
	}
	closeSocketErr := s.conn.Close()
	if closeSubscriptionErr != nil {
		closeSubscriptionErr = fmt.Errorf("close window-manager subscription: %w", closeSubscriptionErr)
	}
	if closeSocketErr != nil && !errors.Is(closeSocketErr, net.ErrClosed) {
		closeSocketErr = fmt.Errorf("close window-manager websocket: %w", closeSocketErr)
	}
	if closeClientCommandsErr != nil {
		closeClientCommandsErr = fmt.Errorf("close window-manager client commands: %w", closeClientCommandsErr)
	}
	var readErr error
	if readDone != nil && !readObserved {
		readErr = <-readDone
		if isExpectedWindowManagerSocketError(readErr) {
			readErr = nil
		}
	}
	return errors.Join(runErr, readErr, closeSubscriptionErr, closeClientCommandsErr, closeSocketErr)
}

func isExpectedWindowManagerSocketError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, windowmanager.ErrClientDisconnected) {
		return true
	}
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	)
}
