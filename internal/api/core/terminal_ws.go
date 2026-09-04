package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	terminalWriteTimeout = 10 * time.Second
	terminalPingInterval = 30 * time.Second
	terminalPongTimeout  = 60 * time.Second
)

var errTerminalDetached = errors.New("terminal websocket detached")

func (h *BaseHandlers) StreamTerminal(c *gin.Context) {
	if h == nil || h.Terminal == nil {
		if h != nil {
			h.respondTerminalUnavailable(c)
		}
		return
	}
	if !h.terminalHostAllowed(c.Request) || !h.terminalOriginAllowed(c.Request) {
		h.respondTerminalStatus(
			c,
			http.StatusForbidden,
			"terminal stream origin or host is not allowed",
		)
		return
	}
	if !terminalProtocolAllowed(c.Request) {
		c.Header("Sec-WebSocket-Protocol", terminalwire.Subprotocol)
		h.respondTerminalStatus(
			c,
			http.StatusUpgradeRequired,
			"terminal stream requires "+terminalwire.Subprotocol,
		)
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	terminalID := terminalpkg.ID(strings.TrimSpace(c.Param("id")))
	mode := strings.TrimSpace(c.Query("mode"))
	handle, subscription, ticket, ok := h.attachTerminalStream(c, workspaceID, terminalID, mode)
	if !ok {
		return
	}
	stop, done, accepting := h.terminalStreams.begin()
	if !accepting {
		closeErr := subscription.Close()
		h.respondTerminalError(c, errors.Join(terminalpkg.ErrShuttingDown, closeErr))
		return
	}
	defer done()
	upgrader := websocket.Upgrader{
		HandshakeTimeout: terminalWriteTimeout,
		Subprotocols:     []string{terminalwire.Subprotocol},
		CheckOrigin:      h.terminalOriginAllowed,
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		closeErr := subscription.Close()
		h.terminalStreamLogger().Warn(
			"terminal websocket upgrade failed",
			"terminal_id",
			terminalID,
			"error",
			errors.Join(err, closeErr),
		)
		return
	}
	socket := &terminalSocket{
		conn: conn, handle: handle, subscription: subscription, actor: ticket.Actor,
		mode: mode, stop: stop,
	}
	if err := socket.run(c.Request.Context()); err != nil && !terminalExpectedSocketError(err) {
		h.terminalStreamLogger().Debug("terminal websocket closed with error", "terminal_id", terminalID, "error", err)
	}
}

func (h *BaseHandlers) terminalStreamLogger() *slog.Logger {
	if h != nil && h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func (h *BaseHandlers) attachTerminalStream(
	c *gin.Context,
	workspaceID string,
	terminalID terminalpkg.ID,
	mode string,
) (terminalpkg.Handle, terminalpkg.Subscription, terminalpkg.AttachTicket, bool) {
	if h == nil || h.Terminal == nil {
		h.respondTerminalUnavailable(c)
		return nil, nil, terminalpkg.AttachTicket{}, false
	}
	options, err := terminalAttachOptions(c, mode)
	if err != nil {
		h.respondTerminalError(c, err)
		return nil, nil, terminalpkg.AttachTicket{}, false
	}
	handle, subscription, ticket, err := h.Terminal.AttachWithTicket(
		c.Request.Context(), c.Query("ticket"), workspaceID, terminalID, mode, options,
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return nil, nil, terminalpkg.AttachTicket{}, false
	}
	return handle, subscription, ticket, true
}

func terminalAttachOptions(c *gin.Context, mode string) (terminalpkg.AttachOptions, error) {
	afterSeq, err := parseTerminalUint(c.Query("after_seq"), 64)
	if err != nil {
		return terminalpkg.AttachOptions{}, err
	}
	cols, err := parseTerminalUint(c.Query("cols"), 16)
	if err != nil {
		return terminalpkg.AttachOptions{}, err
	}
	rows, err := parseTerminalUint(c.Query("rows"), 16)
	if err != nil {
		return terminalpkg.AttachOptions{}, err
	}
	cols16, err := terminalUint16(cols)
	if err != nil {
		return terminalpkg.AttachOptions{}, err
	}
	rows16, err := terminalUint16(rows)
	if err != nil {
		return terminalpkg.AttachOptions{}, err
	}
	return terminalpkg.AttachOptions{
		Mode: mode, Flow: c.Query("flow"), AfterSeq: afterSeq,
		Cols: cols16, Rows: rows16,
	}, nil
}

func terminalUint16(value uint64) (uint16, error) {
	if value > math.MaxUint16 {
		return 0, fmt.Errorf("terminal stream parameter is invalid: %w", terminalpkg.ErrUnsupported)
	}
	return uint16(value), nil
}

func parseTerminalUint(value string, bits int) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("terminal stream parameter is invalid: %w", terminalpkg.ErrUnsupported)
	}
	return parsed, nil
}

func (h *BaseHandlers) terminalHostAllowed(request *http.Request) bool {
	if h.isUDSTransport() {
		return true
	}
	host := terminalHostname(request.Host)
	configured := terminalHostname(h.Config.HTTP.Host)
	return isTerminalLoopback(host) || configured != "" && strings.EqualFold(host, configured)
}

func (h *BaseHandlers) isUDSTransport() bool {
	return h != nil && h.TransportName == transportNameUDSAPI
}

func (h *BaseHandlers) terminalOriginAllowed(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return h.isUDSTransport()
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(parsed.Host), strings.TrimSpace(request.Host))
}

func terminalHostname(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(value, "[]")
}

func isTerminalLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func terminalProtocolAllowed(request *http.Request) bool {
	return slices.Contains(websocket.Subprotocols(request), terminalwire.Subprotocol)
}

type terminalSocket struct {
	conn         *websocket.Conn
	handle       terminalpkg.Handle
	subscription terminalpkg.Subscription
	actor        terminalpkg.Actor
	mode         string
	stop         <-chan struct{}
}

func (s *terminalSocket) run(ctx context.Context) error {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	s.conn.SetReadLimit(terminalwire.MaxInputBytes + 1024)
	if err := s.conn.SetReadDeadline(time.Now().Add(terminalPongTimeout)); err != nil {
		return s.cleanup(fmt.Errorf("set terminal read deadline: %w", err))
	}
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(terminalPongTimeout))
	})
	readDone := make(chan error, 1)
	operationErrors := make(chan error, 1)
	go func() { readDone <- s.readPump(runCtx, operationErrors) }()
	ticker := time.NewTicker(terminalPingInterval)
	defer ticker.Stop()
	for {
		select {
		case frame, ok := <-s.subscription.Frames():
			if !ok {
				if streamErr := s.subscription.Err(); streamErr != nil {
					writeErr := s.writeProtocolError(streamErr)
					closeErr := s.writeClose(websocket.ClosePolicyViolation, "slow_consumer")
					return s.cleanup(errors.Join(streamErr, writeErr, closeErr))
				}
				return s.cleanup(nil)
			}
			if err := s.writeFrame(frame); err != nil {
				return s.cleanup(err)
			}
		case err := <-readDone:
			if err != nil && !errors.Is(err, errTerminalDetached) {
				writeErr := s.writeProtocolError(err)
				return s.cleanup(errors.Join(err, writeErr))
			}
			return s.cleanup(err)
		case operationErr := <-operationErrors:
			if err := s.writeProtocolError(operationErr); err != nil {
				return s.cleanup(errors.Join(operationErr, err))
			}
		case <-ticker.C:
			if err := s.writePing(); err != nil {
				return s.cleanup(err)
			}
		case <-s.stop:
			return s.cleanup(s.writeClose(websocket.CloseGoingAway, "daemon shutdown"))
		case <-ctx.Done():
			return s.cleanup(ctx.Err())
		}
	}
}

func (s *terminalSocket) readPump(ctx context.Context, operationErrors chan<- error) error {
	for {
		messageType, encoded, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.BinaryMessage {
			return terminalRequestError(fmt.Errorf("%w: terminal frames must be binary", terminalwire.ErrInvalidFrame))
		}
		frame, err := terminalwire.DecodeClient(encoded)
		if err != nil {
			return terminalRequestError(err)
		}
		if err := s.applyClientFrame(ctx, frame); err != nil {
			if terminalRecoverableClientOperationError(err) {
				select {
				case operationErrors <- err:
					continue
				case <-ctx.Done():
					return context.Cause(ctx)
				}
			}
			return err
		}
	}
}

func terminalRecoverableClientOperationError(err error) bool {
	return errors.Is(err, terminalpkg.ErrWriteAttachmentRequired) ||
		errors.Is(err, terminalpkg.ErrJournalUnavailable) ||
		errors.Is(err, terminalpkg.ErrGenerationFenced)
}

func (s *terminalSocket) applyClientFrame(ctx context.Context, frame terminalwire.Frame) error {
	switch frame.Op {
	case terminalwire.ClientOpInput:
		if s.mode != terminalModeWrite {
			return terminalReadOnlyOperationError("INPUT")
		}
		return s.handle.Write(ctx, s.actor, frame.Payload)
	case terminalwire.ClientOpAck:
		bytes, err := terminalwire.AckBytes(frame)
		if err != nil {
			return terminalRequestError(err)
		}
		s.subscription.Ack(int(bytes))
		return nil
	case terminalwire.ClientOpResize:
		var payload struct{ Cols, Rows uint16 }
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return terminalRequestError(fmt.Errorf("terminal: decode resize: %w", err))
		}
		return s.subscription.Resize(payload.Cols, payload.Rows)
	case terminalwire.ClientOpSignal:
		if s.mode != terminalModeWrite {
			return terminalReadOnlyOperationError("SIGNAL")
		}
		var payload struct {
			Signal terminalpkg.Signal `json:"signal"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return terminalRequestError(fmt.Errorf("terminal: decode signal: %w", err))
		}
		return s.handle.Signal(ctx, s.actor, payload.Signal)
	case terminalwire.ClientOpDetach:
		return errTerminalDetached
	default:
		return terminalRequestError(terminalwire.ErrUnknownOpcode)
	}
}

func terminalReadOnlyOperationError(operation string) error {
	return fmt.Errorf("%s requires a write attachment: %w", operation, terminalpkg.ErrWriteAttachmentRequired)
}

func (s *terminalSocket) writeFrame(frame terminalwire.Frame) error {
	encoded, err := terminalwire.EncodeServer(frame)
	if err != nil {
		return err
	}
	if err := s.conn.SetWriteDeadline(time.Now().Add(terminalWriteTimeout)); err != nil {
		return fmt.Errorf("set terminal write deadline: %w", err)
	}
	if err := s.conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
		return fmt.Errorf("write terminal frame: %w", err)
	}
	return nil
}

func (s *terminalSocket) writeProtocolError(streamErr error) error {
	_, code, _ := terminalErrorStatusCode(streamErr)
	payload, err := json.Marshal(terminalErrorResponse(code, streamErr))
	if err != nil {
		return err
	}
	return s.writeFrame(terminalwire.Frame{Op: terminalwire.ServerOpError, Payload: payload})
}

func (s *terminalSocket) writePing() error {
	deadline := time.Now().Add(terminalWriteTimeout)
	if err := s.conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
		return fmt.Errorf("write terminal ping: %w", err)
	}
	return nil
}

func (s *terminalSocket) writeClose(code int, reason string) error {
	deadline := time.Now().Add(terminalWriteTimeout)
	if err := s.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		deadline,
	); err != nil {
		return fmt.Errorf("write terminal close: %w", err)
	}
	return nil
}

func (s *terminalSocket) cleanup(runErr error) error {
	subscriptionErr := s.subscription.Close()
	closeErr := s.conn.Close()
	if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
		closeErr = fmt.Errorf("close terminal websocket: %w", closeErr)
	}
	return errors.Join(runErr, subscriptionErr, closeErr)
}

func terminalExpectedSocketError(err error) bool {
	if err == nil || errors.Is(err, errTerminalDetached) || errors.Is(err, context.Canceled) ||
		errors.Is(err, net.ErrClosed) {
		return true
	}
	var closeErr *websocket.CloseError
	return errors.As(err, &closeErr) && closeErr.Code == websocket.CloseNormalClosure
}
