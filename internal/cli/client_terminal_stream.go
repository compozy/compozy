package cli

import (
	"context"
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

func (c *daemonClient) AttachTerminal(
	ctx context.Context,
	workspace, id string,
	options TerminalAttachOptions,
	input io.Reader,
	output io.Writer,
) error {
	if options.Takeover {
		if err := c.takeoverTerminal(ctx, workspace, id, options.Force); err != nil {
			return err
		}
		options.Takeover = false
	}
	var inputReads <-chan terminalInputRead
	if input != nil {
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

func (c *daemonClient) takeoverTerminal(
	ctx context.Context,
	workspace, id string,
	force bool,
) (returnErr error) {
	ticket := ""
	if c.target.kind != clientTargetLocal {
		var err error
		ticket, err = c.mintTerminalTicket(ctx, workspace, id, terminalStreamModeRead)
		if err != nil {
			return err
		}
	}
	dialer := c.terminalWebSocketDialer()
	target, headers, err := c.terminalStreamTarget(ctx, workspace, id, ticket, TerminalAttachOptions{
		Mode: terminalStreamModeRead, Flow: terminalStreamFlowDrop,
	})
	if err != nil {
		return err
	}
	conn, response, err := dialer.DialContext(ctx, target, headers)
	if err != nil {
		if response != nil {
			return readAndCloseStreamHandshakeError(response)
		}
		if c.target.isRemoteGateway() {
			return newGatewayReachabilityError(c.target, err)
		}
		return fmt.Errorf("cli: dial terminal takeover stream: %w", err)
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			returnErr = errors.Join(returnErr, fmt.Errorf("cli: close terminal takeover stream: %w", closeErr))
		}
	}()
	return runTerminalTakeover(ctx, conn, force)
}

func runTerminalTakeover(ctx context.Context, conn *websocket.Conn, force bool) error {
	var writes sync.Mutex
	requested := false
	for {
		if err := conn.SetReadDeadline(time.Now().Add(terminalClientHandshakeTimeout)); err != nil {
			return fmt.Errorf("cli: set terminal takeover deadline: %w", err)
		}
		frame, err := readTerminalServerFrame(conn)
		if err != nil {
			return fmt.Errorf("cli: read terminal takeover stream: %w", err)
		}
		switch frame.Op {
		case terminalwire.ServerOpAttached:
			if requested {
				continue
			}
			payload, marshalErr := json.Marshal(map[string]bool{terminalForceKey: force})
			if marshalErr != nil {
				return fmt.Errorf("cli: encode terminal takeover: %w", marshalErr)
			}
			if err := writeTerminalClientFrame(conn, &writes, terminalwire.Frame{
				Op: terminalwire.ClientOpTakeover, Payload: payload,
			}); err != nil {
				return err
			}
			requested = true
		case terminalwire.ServerOpOwner:
			if !requested {
				continue
			}
			return nil
		case terminalwire.ServerOpError:
			return terminalPermanentError(fmt.Errorf("cli: terminal takeover error: %s", frame.Payload))
		case terminalwire.ServerOpExit:
			return terminalPermanentError(errors.New("cli: terminal exited during takeover"))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
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
	ticket := ""
	if c.target.kind != clientTargetLocal {
		var err error
		ticket, err = c.mintTerminalTicket(ctx, workspace, id, options.Mode)
		if err != nil {
			return options.AfterSeq, err
		}
	}
	dialer := c.terminalWebSocketDialer()
	target, headers, err := c.terminalStreamTarget(ctx, workspace, id, ticket, options)
	if err != nil {
		return options.AfterSeq, err
	}
	conn, response, err := dialer.DialContext(ctx, target, headers)
	if err != nil {
		if response != nil {
			return options.AfterSeq, readAndCloseStreamHandshakeError(response)
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
	return runTerminalClientStreamWithInput(
		ctx,
		conn,
		inputReads,
		output,
		options.AfterSeq,
		options.Mode == terminalStreamModeWrite,
	)
}

func (c *daemonClient) mintTerminalTicket(ctx context.Context, workspace, id, mode string) (string, error) {
	var response struct {
		Ticket string `json:"ticket"`
	}
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/attach-ticket"
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		map[string]string{terminalModeKey: mode},
		&response,
	); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Ticket) == "" {
		return "", terminalPermanentError(errors.New("cli: terminal ticket response is empty"))
	}
	return response.Ticket, nil
}

func (c *daemonClient) terminalStreamTarget(
	ctx context.Context,
	workspace, id, ticket string,
	options TerminalAttachOptions,
) (string, http.Header, error) {
	base, err := c.target.websocketBaseURL()
	if err != nil {
		return "", nil, terminalPermanentError(err)
	}
	query := url.Values{terminalModeKey: {options.Mode}, "flow": {options.Flow}}
	query = profileQueryValues(ctx, query)
	if ticket != "" {
		query.Set("ticket", ticket)
	}
	if options.AfterSeq > 0 {
		query.Set("after_seq", strconv.FormatUint(options.AfterSeq, 10))
	}
	if options.Cols > 0 && options.Rows > 0 {
		query.Set("cols", strconv.FormatUint(uint64(options.Cols), 10))
		query.Set("rows", strconv.FormatUint(uint64(options.Rows), 10))
	}
	target := base + terminalClientPath(workspace) + "/" +
		url.PathEscape(strings.TrimSpace(id)) + "/stream?" + query.Encode()
	headers := http.Header{}
	if c.target.kind != clientTargetLocal {
		parsed, parseErr := url.Parse(target)
		if parseErr != nil {
			return "", nil, terminalPermanentError(parseErr)
		}
		scheme := terminalClientHTTPProtocol
		if parsed.Scheme == "wss" {
			scheme = clientHTTPSProtocol
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
	if input != nil {
		inputReads = terminalInputReads(ctx, input)
	}
	_, err := runTerminalClientStreamWithInput(
		ctx,
		conn,
		inputReads,
		output,
		0,
		mode == terminalStreamModeWrite,
	)
	return err
}

func runTerminalClientStreamWithInput(
	ctx context.Context,
	conn *websocket.Conn,
	inputReads <-chan terminalInputRead,
	output io.Writer,
	afterSeq uint64,
	forwardInput bool,
) (resultSeq uint64, returnErr error) {
	streamCtx, cancel := context.WithCancel(ctx)
	var writes sync.Mutex
	inputDone := make(chan error, 1)
	inputFinished := false
	if inputReads != nil {
		go func() {
			inputDone <- copyTerminalInput(streamCtx, conn, &writes, inputReads, forwardInput)
		}()
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
		frame, err := readTerminalServerFrame(conn)
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

func (c *daemonClient) terminalWebSocketDialer() websocket.Dialer {
	dialer := websocket.Dialer{
		HandshakeTimeout: terminalClientHandshakeTimeout,
		Subprotocols:     []string{terminalwire.Subprotocol},
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
	return dialer
}
