package core

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
		h.respondTerminalStatus(c, http.StatusForbidden, "terminal_origin_forbidden", "terminal stream origin or host is not allowed")
		return
	}
	if !terminalProtocolAllowed(c.Request) {
		c.Header("Sec-WebSocket-Protocol", terminalwire.Subprotocol)
		h.respondTerminalStatus(c, http.StatusUpgradeRequired, "terminal_protocol_required", "terminal stream requires compozy.terminal.v1")
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	terminalID := terminalpkg.ID(strings.TrimSpace(c.Param("id")))
	mode := strings.TrimSpace(c.Query("mode"))
	var ticket terminalTicket
	var service terminalpkg.Manager
	var err error
	if h.isUDSTransport() {
		var profileID string
		var ok bool
		service, profileID, ok = h.terminalService(c, mode == "write")
		if !ok {
			return
		}
		actor, actorOK := h.terminalActor(c, workspaceID, profileID, "terminal.attach")
		if !actorOK {
			return
		}
		ticket = terminalTicket{
			Binding: terminalTicketBinding{
				WorkspaceID: workspaceID, ProfileID: profileID, TerminalID: terminalID, Mode: mode,
			},
			Actor: actor,
		}
	} else {
		if h.terminalTickets == nil {
			h.respondTerminalUnavailable(c)
			return
		}
		ticket, err = h.terminalTickets.ConsumeStream(c.Query("ticket"), workspaceID, terminalID, mode)
		if err != nil {
			h.respondTerminalError(c, err)
			return
		}
		service, err = h.Terminal.TerminalFor(ticket.Binding.ProfileID)
		if err != nil {
			h.respondTerminalError(c, err)
			return
		}
	}
	handle, err := service.Handle(c.Request.Context(), workspaceID, ticket.Binding.ProfileID, terminalID)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	options, err := terminalAttachOptions(c, ticket)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	subscription, err := handle.Attach(c.Request.Context(), options)
	if err != nil {
		h.respondTerminalError(c, err)
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
		if h.Logger != nil {
			h.Logger.Warn("terminal websocket upgrade failed", "terminal_id", terminalID, "error", errors.Join(err, closeErr))
		}
		return
	}
	socket := &terminalSocket{
		conn: conn, handle: handle, subscription: subscription, actor: ticket.Actor,
		mode: mode, stop: stop,
	}
	if err := socket.run(c.Request.Context()); err != nil && !terminalExpectedSocketError(err) && h.Logger != nil {
		h.Logger.Debug("terminal websocket closed with error", "terminal_id", terminalID, "error", err)
	}
}

func terminalAttachOptions(c *gin.Context, ticket terminalTicket) (terminalpkg.AttachOptions, error) {
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
	return terminalpkg.AttachOptions{
		Mode: ticket.Binding.Mode, Flow: c.Query("flow"), AfterSeq: afterSeq,
		Cols: uint16(cols), Rows: uint16(rows), Actor: ticket.Actor,
	}, nil
}

func parseTerminalUint(value string, bits int) (uint64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(value, 10, bits)
	if err != nil {
		return 0, &terminalpkg.Error{Code: "terminal_stream_parameter_invalid", Message: "terminal stream parameter is invalid", Err: terminalpkg.ErrUnsupported}
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
	for _, protocol := range websocket.Subprotocols(request) {
		if protocol == terminalwire.Subprotocol {
			return true
		}
	}
	return false
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
	s.conn.SetReadLimit(terminalwire.MaxInputBytes + 1024)
	if err := s.conn.SetReadDeadline(time.Now().Add(terminalPongTimeout)); err != nil {
		return s.cleanup(fmt.Errorf("set terminal read deadline: %w", err))
	}
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(terminalPongTimeout))
	})
	readDone := make(chan error, 1)
	go func() { readDone <- s.readPump(ctx) }()
	ticker := time.NewTicker(terminalPingInterval)
	defer ticker.Stop()
	for {
		select {
		case frame, ok := <-s.subscription.Frames():
			if !ok {
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

func (s *terminalSocket) readPump(ctx context.Context) error {
	for {
		messageType, encoded, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.BinaryMessage {
			return fmt.Errorf("%w: terminal frames must be binary", terminalwire.ErrInvalidFrame)
		}
		frame, err := terminalwire.DecodeClient(encoded)
		if err != nil {
			return err
		}
		if err := s.applyClientFrame(ctx, frame); err != nil {
			return err
		}
	}
}

func (s *terminalSocket) applyClientFrame(ctx context.Context, frame terminalwire.Frame) error {
	switch frame.Op {
	case terminalwire.ClientOpInput:
		if s.mode != "write" {
			return &terminalpkg.Error{Code: "write_owner_held", Message: "read-only terminal attachment cannot write", Err: terminalpkg.ErrWriteOwnerHeld}
		}
		return s.handle.Write(ctx, s.actor, frame.Payload)
	case terminalwire.ClientOpAck:
		bytes, err := terminalwire.AckBytes(frame)
		if err != nil {
			return err
		}
		s.subscription.Ack(int(bytes))
		return nil
	case terminalwire.ClientOpResize:
		var payload struct{ Cols, Rows uint16 }
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return fmt.Errorf("terminal: decode resize: %w", err)
		}
		return s.subscription.Resize(payload.Cols, payload.Rows)
	case terminalwire.ClientOpSignal:
		var payload struct {
			Signal terminalpkg.Signal `json:"signal"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return fmt.Errorf("terminal: decode signal: %w", err)
		}
		return s.handle.Signal(ctx, s.actor, payload.Signal)
	case terminalwire.ClientOpTakeover:
		var payload struct {
			Force bool `json:"force"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return fmt.Errorf("terminal: decode takeover: %w", err)
		}
		return s.handle.Takeover(ctx, s.actor, payload.Force)
	case terminalwire.ClientOpDetach:
		return errTerminalDetached
	case terminalwire.ClientOpRelease:
		return s.handle.Yield(ctx, s.actor)
	default:
		return terminalwire.ErrUnknownOpcode
	}
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
	_, code := terminalErrorStatusCode(streamErr)
	payload, err := json.Marshal(map[string]string{"code": code, "message": streamErr.Error()})
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
	if err := s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), deadline); err != nil {
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
	if err == nil || errors.Is(err, errTerminalDetached) || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var closeErr *websocket.CloseError
	return errors.As(err, &closeErr) && closeErr.Code == websocket.CloseNormalClosure
}
