package cli

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gorilla/websocket"
)

const terminalClientHandshakeTimeout = 10 * time.Second

var errTerminalDetached = errors.New("cli: terminal detached")

type terminalServerRead struct {
	frame terminalwire.Frame
	err   error
}

type terminalClientPermanentError struct {
	err error
}

func (e *terminalClientPermanentError) Error() string { return e.err.Error() }
func (e *terminalClientPermanentError) Unwrap() error { return e.err }

func (c *daemonClient) AttachTerminal(
	ctx context.Context,
	workspace, id string,
	options TerminalAttachOptions,
	input io.Reader,
	output io.Writer,
) error {
	var inputReads <-chan terminalInputRead
	if options.Mode == "write" && input != nil {
		inputReads = terminalInputReads(ctx, input)
	}
	for attempt := 0; ; attempt++ {
		afterSeq, err := c.attachTerminalOnce(ctx, workspace, id, options, inputReads, output)
		options.AfterSeq = max(options.AfterSeq, afterSeq)
		if err == nil || ctx.Err() != nil || !terminalReconnectableError(err) {
			return errors.Join(err, ctx.Err())
		}
		delay := terminalReconnectDelay(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (c *daemonClient) attachTerminalOnce(
	ctx context.Context,
	workspace, id string,
	options TerminalAttachOptions,
	inputReads <-chan terminalInputRead,
	output io.Writer,
) (afterSeq uint64, returnErr error) {
	ticket, err := c.mintTerminalTicket(ctx, workspace, id, options.Mode)
	if err != nil {
		return options.AfterSeq, err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: terminalClientHandshakeTimeout,
		Subprotocols:     []string{terminalwire.Subprotocol},
	}
	if c.target.kind == clientTargetLocal {
		dialer.NetDialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var networkDialer net.Dialer
			return networkDialer.DialUnix(ctx, clientUnixNetwork, nil, &net.UnixAddr{Name: c.target.socketPath, Net: clientUnixNetwork})
		}
	}
	target, headers, err := c.terminalStreamTarget(workspace, id, ticket, options)
	if err != nil {
		return options.AfterSeq, err
	}
	conn, response, err := dialer.DialContext(ctx, target, headers)
	if err != nil {
		if response != nil {
			return options.AfterSeq, readAndCloseWindowManagerHandshakeError(response)
		}
		if c.target.isRemoteGateway() {
			return options.AfterSeq, newGatewayReachabilityError(c.target, err)
		}
		return options.AfterSeq, fmt.Errorf("cli: dial terminal stream: %w", err)
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			returnErr = errors.Join(returnErr, fmt.Errorf("cli: close terminal stream: %w", closeErr))
		}
	}()
	return runTerminalClientStreamWithInput(ctx, conn, inputReads, output, options.AfterSeq)
}

func (c *daemonClient) mintTerminalTicket(ctx context.Context, workspace, id, mode string) (string, error) {
	var response struct {
		Ticket string `json:"ticket"`
	}
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/attach-ticket"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]string{"mode": mode}, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Ticket) == "" {
		return "", terminalPermanentError(errors.New("cli: terminal ticket response is empty"))
	}
	return response.Ticket, nil
}

func (c *daemonClient) terminalStreamTarget(
	workspace, id, ticket string,
	options TerminalAttachOptions,
) (string, http.Header, error) {
	base, err := c.target.websocketBaseURL()
	if err != nil {
		return "", nil, terminalPermanentError(err)
	}
	query := url.Values{
		"ticket": {ticket}, "mode": {options.Mode}, "flow": {options.Flow},
	}
	if options.AfterSeq > 0 {
		query.Set("after_seq", strconv.FormatUint(options.AfterSeq, 10))
	}
	if options.Cols > 0 && options.Rows > 0 {
		query.Set("cols", strconv.FormatUint(uint64(options.Cols), 10))
		query.Set("rows", strconv.FormatUint(uint64(options.Rows), 10))
	}
	target := base + terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/stream?" + query.Encode()
	headers := http.Header{}
	if c.target.kind != clientTargetLocal {
		parsed, parseErr := url.Parse(target)
		if parseErr != nil {
			return "", nil, terminalPermanentError(parseErr)
		}
		scheme := "http"
		if parsed.Scheme == "wss" {
			scheme = "https"
		}
		headers.Set("Origin", scheme+"://"+parsed.Host)
	}
	return target, headers, nil
}

func runTerminalClientStream(
	ctx context.Context,
	conn *websocket.Conn,
	mode string,
	input io.Reader,
	output io.Writer,
) error {
	var inputReads <-chan terminalInputRead
	if mode == "write" && input != nil {
		inputReads = terminalInputReads(ctx, input)
	}
	_, err := runTerminalClientStreamWithInput(ctx, conn, inputReads, output, 0)
	return err
}

func runTerminalClientStreamWithInput(
	ctx context.Context,
	conn *websocket.Conn,
	inputReads <-chan terminalInputRead,
	output io.Writer,
	afterSeq uint64,
) (resultSeq uint64, returnErr error) {
	streamCtx, cancel := context.WithCancel(ctx)
	var writes sync.Mutex
	inputDone := make(chan error, 1)
	inputFinished := false
	if inputReads != nil {
		go func() { inputDone <- copyTerminalInput(streamCtx, conn, &writes, inputReads) }()
	} else {
		inputDone = nil
	}
	defer func() {
		cancel()
		if inputDone != nil && !inputFinished {
			if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
				returnErr = errors.Join(returnErr, fmt.Errorf("cli: close terminal input stream: %w", closeErr))
			}
			<-inputDone
		}
	}()
	serverReads := make(chan terminalServerRead, 1)
	go readTerminalServer(streamCtx, conn, serverReads)
	ackPending := 0
	attachedSeq := uint64(0)
	for {
		select {
		case inputErr := <-inputDone:
			inputFinished = true
			if errors.Is(inputErr, errTerminalDetached) || errors.Is(inputErr, io.EOF) {
				return afterSeq, nil
			}
			return afterSeq, inputErr
		case read := <-serverReads:
			if read.err != nil {
				return afterSeq, fmt.Errorf("cli: read terminal stream: %w", read.err)
			}
			done, err := handleTerminalServerFrame(
				conn, &writes, output, read.frame, &ackPending, &afterSeq, &attachedSeq,
			)
			if err != nil {
				return afterSeq, err
			}
			if done {
				return afterSeq, nil
			}
		case <-ctx.Done():
			return afterSeq, ctx.Err()
		}
	}
}

func readTerminalServer(ctx context.Context, conn *websocket.Conn, reads chan<- terminalServerRead) {
	for {
		messageType, encoded, err := conn.ReadMessage()
		if err == nil && messageType != websocket.BinaryMessage {
			err = terminalPermanentError(errors.New("cli: terminal server sent a non-binary frame"))
		}
		var frame terminalwire.Frame
		if err == nil {
			frame, err = terminalwire.DecodeServer(encoded)
			if err != nil {
				err = terminalPermanentError(err)
			}
		}
		select {
		case reads <- terminalServerRead{frame: frame, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func handleTerminalServerFrame(
	conn *websocket.Conn,
	writes *sync.Mutex,
	output io.Writer,
	frame terminalwire.Frame,
	ackPending *int,
	afterSeq *uint64,
	attachedSeq *uint64,
) (bool, error) {
	switch frame.Op {
	case terminalwire.ServerOpOutput:
		written := len(frame.Payload)
		if output != nil {
			var err error
			written, err = output.Write(frame.Payload)
			if err != nil {
				return false, terminalPermanentError(fmt.Errorf("cli: write terminal output: %w", err))
			}
			if written != len(frame.Payload) {
				return false, terminalPermanentError(fmt.Errorf("cli: write terminal output: %w", io.ErrShortWrite))
			}
		}
		*ackPending += written
		for *ackPending >= terminalwire.AckGrainBytes {
			if err := writeTerminalClientFrame(conn, writes, terminalwire.NewACK(terminalwire.AckGrainBytes)); err != nil {
				return false, err
			}
			*ackPending -= terminalwire.AckGrainBytes
		}
		if *attachedSeq > 0 {
			*afterSeq = max(*afterSeq, *attachedSeq)
			*attachedSeq = 0
		} else {
			*afterSeq = max(*afterSeq, frame.Seq+uint64(written))
		}
	case terminalwire.ServerOpAttached:
		var payload struct {
			Seq uint64 `json:"seq"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return false, terminalPermanentError(fmt.Errorf("cli: decode ATTACHED frame: %w", err))
		}
		if payload.Seq > *afterSeq {
			*attachedSeq = payload.Seq
		}
	case terminalwire.ServerOpGap:
		var payload struct {
			ToSeq uint64 `json:"to_seq"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err != nil {
			return false, terminalPermanentError(fmt.Errorf("cli: decode GAP frame: %w", err))
		}
		*afterSeq = max(*afterSeq, payload.ToSeq)
	case terminalwire.ServerOpExit:
		return true, nil
	case terminalwire.ServerOpError:
		return false, terminalPermanentError(fmt.Errorf("cli: terminal stream error: %s", frame.Payload))
	}
	return false, nil
}

func writeTerminalClientFrame(conn *websocket.Conn, writes *sync.Mutex, frame terminalwire.Frame) error {
	encoded, err := terminalwire.EncodeClient(frame)
	if err != nil {
		return terminalPermanentError(err)
	}
	writes.Lock()
	defer writes.Unlock()
	if err := conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
		return fmt.Errorf("cli: write terminal frame: %w", err)
	}
	return nil
}

func terminalReconnectDelay(attempt int) time.Duration {
	base := 500 * time.Millisecond * time.Duration(1<<min(attempt, 4))
	base = min(base, 8*time.Second)
	var random [1]byte
	if _, err := cryptorand.Read(random[:]); err != nil {
		return base
	}
	return base + time.Duration(random[0])*time.Millisecond
}

func terminalPermanentError(err error) error {
	if err == nil {
		return nil
	}
	return &terminalClientPermanentError{err: err}
}

func terminalReconnectableError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var permanent *terminalClientPermanentError
	if errors.As(err, &permanent) {
		return false
	}
	var profileErr *profileCommandError
	if errors.As(err, &profileErr) {
		return false
	}
	var apiErr *daemonAPIError
	if errors.As(err, &apiErr) {
		return apiErr.statusCode >= http.StatusInternalServerError
	}
	var gatewayErr *gatewayClientError
	if errors.As(err, &gatewayErr) {
		return true
	}
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		switch closeErr.Code {
		case websocket.CloseGoingAway,
			websocket.CloseAbnormalClosure,
			websocket.CloseInternalServerErr,
			websocket.CloseServiceRestart,
			websocket.CloseTryAgainLater:
			return true
		default:
			return false
		}
	}
	return true
}
