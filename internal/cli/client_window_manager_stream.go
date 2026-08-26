package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gorilla/websocket"
)

const windowManagerClientHandshakeTimeout = 10 * time.Second

func (c *daemonClient) WatchWindowManager(
	ctx context.Context,
	workspace string,
	clientID *windowmanager.ClientID,
	afterRevision *contract.WindowManagerRevision,
	handlers WindowManagerStreamHandlers,
) (returnErr error) {
	if err := validateWindowManagerWatch(ctx, clientID, handlers); err != nil {
		return err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: windowManagerClientHandshakeTimeout,
	}
	if c.target.kind == clientTargetLocal {
		dialer.NetDialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var networkDialer net.Dialer
			return networkDialer.DialUnix(
				ctx,
				clientUnixNetwork,
				nil,
				&net.UnixAddr{Name: c.target.socketPath, Net: clientUnixNetwork},
			)
		}
	}
	query := url.Values{}
	if afterRevision != nil {
		query.Set("after_revision", strconv.FormatUint(uint64(*afterRevision), 10))
	}
	if clientID != nil {
		query.Set("client_id", strings.TrimSpace(string(*clientID)))
	}
	// Desks are per-profile, and this dialer builds its own URL instead of going
	// through the request path that stamps the resolved profile — so it stamps it
	// here or it would watch the default profile's windows.
	query = profileQueryValues(ctx, query)
	query, err := c.withFreshStreamTicket(ctx, query)
	if err != nil {
		return err
	}
	websocketBase, err := c.target.websocketBaseURL()
	if err != nil {
		return err
	}
	target := websocketBase + windowManagerClientPath(workspace) + "/stream"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	conn, response, err := dialer.DialContext(ctx, target, nil)
	if err != nil {
		if response != nil {
			return readAndCloseWindowManagerHandshakeError(response)
		}
		if c.target.isRemoteGateway() {
			return newGatewayReachabilityError(c.target, err)
		}
		return fmt.Errorf("cli: dial window-manager stream: %w", err)
	}

	cancelCloseErr := make(chan error, 1)
	stopCancelClose := context.AfterFunc(ctx, func() {
		cancelCloseErr <- conn.Close()
	})
	defer func() {
		var closeErr error
		if stopCancelClose() {
			closeErr = conn.Close()
		} else {
			closeErr = <-cancelCloseErr
		}
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			closeErr = fmt.Errorf("cli: close window-manager stream: %w", closeErr)
		} else {
			closeErr = nil
		}
		returnErr = errors.Join(returnErr, closeErr)
	}()

	streamErr := readWindowManagerFrames(ctx, conn, handlers)
	if c.target.isRemoteGateway() && ctx.Err() == nil {
		return newGatewayStreamInterruptedError(c.target, streamErr)
	}
	return streamErr
}

func validateWindowManagerWatch(
	ctx context.Context,
	clientID *windowmanager.ClientID,
	handlers WindowManagerStreamHandlers,
) error {
	if ctx == nil {
		return errors.New("cli: window-manager watch context is required")
	}
	if handlers.Snapshot == nil || handlers.Event == nil {
		return errors.New("cli: window-manager watch handlers are required")
	}
	if clientID != nil && handlers.Client == nil {
		return errors.New("cli: window-manager client watch handler is required")
	}
	return nil
}

func readAndCloseWindowManagerHandshakeError(response *http.Response) error {
	apiErr := readAPIError(response)
	closeErr := response.Body.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("cli: close window-manager handshake response: %w", closeErr)
	}
	return errors.Join(apiErr, closeErr)
}

func readWindowManagerFrames(
	ctx context.Context,
	conn *websocket.Conn,
	handlers WindowManagerStreamHandlers,
) error {
	snapshotReceived := false
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("cli: read window-manager stream: %w", err)
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("cli: decode window-manager frame: %w", err)
		}
		switch strings.TrimSpace(envelope.Type) {
		case contract.WindowManagerFrameSnapshot:
			if snapshotReceived {
				return errors.New("cli: window-manager stream sent more than one snapshot")
			}
			var frame contract.WindowManagerSnapshotFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				return fmt.Errorf("cli: decode window-manager snapshot: %w", err)
			}
			snapshotReceived = true
			if err := handlers.Snapshot(frame); err != nil {
				return err
			}
		case contract.WindowManagerFrameEvent:
			if !snapshotReceived {
				return errors.New("cli: window-manager event arrived before snapshot")
			}
			var frame contract.WindowManagerEventFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				return fmt.Errorf("cli: decode window-manager event: %w", err)
			}
			if err := handlers.Event(frame); err != nil {
				return err
			}
		case contract.WindowManagerFrameClient:
			if !snapshotReceived {
				return errors.New("cli: window-manager client frame arrived before snapshot")
			}
			if handlers.Client == nil {
				return errors.New("cli: window-manager stream sent an unexpected client frame")
			}
			var frame contract.WindowManagerClientFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				return fmt.Errorf("cli: decode window-manager client frame: %w", err)
			}
			if err := handlers.Client(frame); err != nil {
				return err
			}
		case contract.WindowManagerFrameError:
			var frame contract.WindowManagerErrorFrame
			if err := json.Unmarshal(payload, &frame); err != nil {
				return fmt.Errorf("cli: decode window-manager error: %w", err)
			}
			return &windowManagerAPIError{statusCode: http.StatusConflict, payload: frame.Error}
		default:
			return fmt.Errorf("cli: unsupported window-manager frame %q", envelope.Type)
		}
	}
}
