package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
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
	errWindowManagerClientMessage = errors.New("window manager stream is server-push only")
)

// StreamWindowManager upgrades one HTTP or UDS request to a snapshot-fenced WebSocket stream.
func (h *BaseHandlers) StreamWindowManager(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	after, err := windowManagerAfterRevision(c)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return
	}
	clientID := windowManagerStreamClientID(c)
	stop, done, ok := h.windowManagerStreams.begin()
	if !ok {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return
	}
	defer done()
	subscription, err := h.WindowManager.Subscribe(
		c.Request.Context(),
		windowmanager.SubscriptionRequest{
			WorkspaceID: workspaceID, AfterRevision: after, ClientID: clientID,
		},
	)
	if err != nil {
		if clientID != nil &&
			errors.Is(err, windowmanager.ErrClientNotFound) &&
			websocket.IsWebSocketUpgrade(c.Request) {
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
	socket := windowManagerSocket{conn: conn, subscription: subscription, stop: stop, workspaceID: workspaceID}
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
	conn         *websocket.Conn
	subscription windowmanager.Subscription
	stop         <-chan struct{}
	workspaceID  windowmanager.WorkspaceID
}

func (s *windowManagerSocket) run(ctx context.Context) error {
	if err := s.configureReader(); err != nil {
		return s.cleanup(nil, err, false)
	}
	fence := s.subscription.Fence()
	snapshot, err := contract.WindowManagerSnapshotFromDomain(fence.Snapshot)
	if err != nil {
		encodeErr := fmt.Errorf("encode window-manager stream snapshot: %w", err)
		return s.cleanup(nil, errors.Join(encodeErr, s.writeError(encodeErr)), false)
	}
	var client *contract.WindowManagerClientView
	if fence.Client != nil {
		converted, convertErr := contract.WindowManagerClientFromDomain(*fence.Client)
		if convertErr != nil {
			encodeErr := fmt.Errorf("encode window-manager stream client fence: %w", convertErr)
			return s.cleanup(nil, errors.Join(encodeErr, s.writeError(encodeErr)), false)
		}
		client = &converted
	}
	if err := s.writeJSON(contract.WindowManagerSnapshotFrame{
		Type: contract.WindowManagerFrameSnapshot, WorkspaceID: s.workspaceID,
		Revision: snapshot.Revision, Snapshot: snapshot, Client: client,
	}); err != nil {
		return s.cleanup(nil, err, false)
	}

	readDone := make(chan error, 1)
	go func() {
		readDone <- s.readPump()
	}()
	ticker := time.NewTicker(windowManagerPingInterval)
	defer ticker.Stop()

	var readObserved bool
	for {
		select {
		case update, ok := <-s.subscription.Updates():
			if !ok {
				runErr := s.subscription.Err()
				if runErr != nil {
					runErr = errors.Join(runErr, s.writeError(runErr))
				}
				return s.cleanup(readDone, runErr, readObserved)
			}
			encoded, convertErr := encodeWindowManagerUpdate(s.workspaceID, update)
			if convertErr != nil {
				return s.cleanup(readDone, errors.Join(convertErr, s.writeError(convertErr)), readObserved)
			}
			if writeErr := encoded.write(s); writeErr != nil {
				return s.cleanup(readDone, writeErr, readObserved)
			}
		case readErr := <-readDone:
			readObserved = true
			if errors.Is(readErr, errWindowManagerClientMessage) {
				readErr = errors.Join(readErr, s.writeError(windowmanager.ErrInvalidCommand))
			}
			return s.cleanup(readDone, readErr, readObserved)
		case <-ticker.C:
			if pingErr := s.writePing(); pingErr != nil {
				return s.cleanup(readDone, pingErr, readObserved)
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

func (s *windowManagerSocket) readPump() error {
	for {
		messageType, _, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			return errWindowManagerClientMessage
		}
	}
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
	closeSocketErr := s.conn.Close()
	if closeSubscriptionErr != nil {
		closeSubscriptionErr = fmt.Errorf("close window-manager subscription: %w", closeSubscriptionErr)
	}
	if closeSocketErr != nil && !errors.Is(closeSocketErr, net.ErrClosed) {
		closeSocketErr = fmt.Errorf("close window-manager websocket: %w", closeSocketErr)
	}
	var readErr error
	if readDone != nil && !readObserved {
		readErr = <-readDone
		if isExpectedWindowManagerSocketError(readErr) {
			readErr = nil
		}
	}
	return errors.Join(runErr, readErr, closeSubscriptionErr, closeSocketErr)
}

func isExpectedWindowManagerSocketError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return true
	}
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	)
}
