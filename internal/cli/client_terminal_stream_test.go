package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
	"github.com/gorilla/websocket"
)

func TestTerminalClientStreamShouldDetachWithoutReconnect(t *testing.T) {
	t.Parallel()
	client, server := newTerminalClientTestPair(t)
	done := make(chan error, 1)
	input := strings.NewReader(string([]byte{terminalDetachByte, terminalDetachByte}))
	go func() {
		done <- runTerminalClientStream(
			context.Background(), client, terminalStreamModeWrite, input, io.Discard,
		)
	}()
	frame := readTerminalClientTestFrame(t, server)
	if frame.Op != terminalwire.ClientOpDetach {
		t.Fatalf("opcode = %d, want DETACH", frame.Op)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runTerminalClientStream() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not finish after the detach chord")
	}
}

func TestTerminalClientStreamShouldTakeOverBeforeWriteAttach(t *testing.T) {
	t.Parallel()
	client, server := newTerminalClientTestPair(t)
	done := make(chan error, 1)
	go func() {
		done <- runTerminalTakeover(context.Background(), client, true)
	}()
	writeTerminalServerTestFrame(t, server, terminalwire.Frame{
		Op: terminalwire.ServerOpAttached, Payload: []byte(`{"seq":0}`),
	})
	takeover := readTerminalClientTestFrame(t, server)
	if takeover.Op != terminalwire.ClientOpTakeover {
		t.Fatalf("takeover opcode = %d, want TAKEOVER", takeover.Op)
	}
	if string(takeover.Payload) != `{"force":true}` {
		t.Fatalf("takeover payload = %s, want force=true", takeover.Payload)
	}
	writeTerminalServerTestFrame(t, server, terminalwire.Frame{
		Op: terminalwire.ServerOpOwner, Payload: []byte(`{"lease":"human_owned"}`),
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runTerminalTakeover() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("takeover did not finish after OWNER")
	}
}

func TestTerminalClientInputShouldPassSingleDetachByteAfterTimeout(t *testing.T) {
	t.Parallel()
	client, server := newTerminalClientTestPair(t)
	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	var writes sync.Mutex
	go func() { done <- copyTerminalInput(ctx, client, &writes, terminalInputReads(ctx, reader)) }()
	started := time.Now()
	if _, err := writer.Write([]byte{terminalDetachByte}); err != nil {
		t.Fatalf("write input pipe: %v", err)
	}
	frame := readTerminalClientTestFrame(t, server)
	if frame.Op != terminalwire.ClientOpInput || !bytes.Equal(frame.Payload, []byte{terminalDetachByte}) {
		t.Fatalf("single chord frame = %#v", frame)
	}
	if elapsed := time.Since(started); elapsed < terminalDetachTimeout-(25*time.Millisecond) {
		t.Fatalf("single chord sent after %s, want about %s", elapsed, terminalDetachTimeout)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close input pipe: %v", err)
	}
	if frame := readTerminalClientTestFrame(t, server); frame.Op != terminalwire.ClientOpDetach {
		t.Fatalf("EOF opcode = %d, want DETACH", frame.Op)
	}
	select {
	case err := <-done:
		if !errors.Is(err, errTerminalDetached) {
			t.Fatalf("copyTerminalInput() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("input copier did not finish after EOF")
	}
}

func TestTerminalClientStreamShouldAckOnlyWrittenOutput(t *testing.T) {
	t.Parallel()
	client, server := newTerminalClientTestPair(t)
	done := make(chan error, 1)
	var output bytes.Buffer
	go func() {
		done <- runTerminalClientStream(
			context.Background(), client, terminalStreamModeRead, nil, &output,
		)
	}()
	payload := bytes.Repeat([]byte("x"), terminalwire.AckGrainBytes)
	writeTerminalServerTestFrame(t, server, terminalwire.Frame{Op: terminalwire.ServerOpOutput, Payload: payload})
	ack := readTerminalClientTestFrame(t, server)
	ackedBytes, err := terminalwire.AckBytes(ack)
	if err != nil || ackedBytes != terminalwire.AckGrainBytes {
		t.Fatalf("ACK frame = %#v", ack)
	}
	if !bytes.Equal(output.Bytes(), payload) {
		t.Fatal("output bytes differ from the acknowledged payload")
	}
	writeTerminalServerTestFrame(t, server, terminalwire.Frame{Op: terminalwire.ServerOpExit, Payload: []byte(`{}`)})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runTerminalClientStream() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not finish after EXIT")
	}
}

func TestTerminalClientStreamShouldReconnectOnlyTransientFailures(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "network EOF", err: io.EOF, want: true},
		{name: "daemon shutdown", err: &websocket.CloseError{Code: websocket.CloseGoingAway}, want: true},
		{name: "server unavailable", err: &daemonAPIError{statusCode: http.StatusServiceUnavailable}, want: true},
		{name: "invalid ticket", err: &daemonAPIError{statusCode: http.StatusForbidden}, want: false},
		{name: "protocol failure", err: terminalPermanentError(terminalwire.ErrInvalidFrame), want: false},
		{name: "protocol close", err: &websocket.CloseError{Code: websocket.CloseProtocolError}, want: false},
		{name: "policy close", err: &websocket.CloseError{Code: websocket.ClosePolicyViolation}, want: false},
		{name: "normal close", err: &websocket.CloseError{Code: websocket.CloseNormalClosure}, want: false},
	} {
		t.Run("Should classify "+testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := terminalReconnectableError(testCase.err); got != testCase.want {
				t.Fatalf("terminalReconnectableError(%v) = %v, want %v", testCase.err, got, testCase.want)
			}
		})
	}
}

func TestTerminalClientStreamShouldPreserveTransientServerClose(t *testing.T) {
	t.Parallel()
	client, server := newTerminalClientTestPair(t)
	done := make(chan error, 1)
	go func() {
		done <- runTerminalClientStream(
			context.Background(), client, terminalStreamModeRead, nil, io.Discard,
		)
	}()
	deadline := time.Now().Add(time.Second)
	if err := server.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseGoingAway, "daemon shutdown"),
		deadline,
	); err != nil {
		t.Fatalf("WriteControl(close) error = %v", err)
	}
	select {
	case err := <-done:
		if !terminalReconnectableError(err) {
			t.Fatalf("stream error = %v, want reconnectable", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client stream did not return after server close")
	}
}

func TestTerminalClientStreamShouldAdvanceOnlyWrittenSequence(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name        string
		initial     uint64
		attached    uint64
		outputSeq   uint64
		outputBytes int
		want        uint64
	}{
		{name: "live output", initial: 10, attached: 10, outputSeq: 10, outputBytes: 3, want: 13},
		{name: "truncated resync", initial: 5, attached: 20, outputSeq: 0, outputBytes: 30, want: 20},
	} {
		t.Run("Should advance "+testCase.name, func(t *testing.T) {
			t.Parallel()
			client, server := newTerminalClientTestPair(t)
			type result struct {
				seq uint64
				err error
			}
			done := make(chan result, 1)
			go func() {
				seq, err := runTerminalClientStreamWithInput(
					context.Background(), client, nil, io.Discard, testCase.initial,
				)
				done <- result{seq: seq, err: err}
			}()
			writeTerminalServerTestFrame(t, server, terminalwire.Frame{
				Op: terminalwire.ServerOpAttached,
				Payload: fmt.Appendf(nil, `{"seq":%d,"truncated":%t}`,
					testCase.attached, testCase.attached > testCase.initial),
			})
			writeTerminalServerTestFrame(t, server, terminalwire.Frame{
				Op: terminalwire.ServerOpOutput, Seq: testCase.outputSeq,
				Payload: bytes.Repeat([]byte("x"), testCase.outputBytes),
			})
			writeTerminalServerTestFrame(t, server, terminalwire.Frame{
				Op: terminalwire.ServerOpExit, Payload: []byte(`{}`),
			})
			select {
			case got := <-done:
				if got.err != nil || got.seq != testCase.want {
					t.Fatalf("stream result = %d/%v, want %d/nil", got.seq, got.err, testCase.want)
				}
			case <-time.After(time.Second):
				t.Fatal("client stream did not finish")
			}
		})
	}
}

func TestTerminalClientInputShouldSurviveConnectionReplacement(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close input reader: %v", err)
		}
	})
	inputReads := terminalInputReads(ctx, reader)

	firstClient, firstServer := newTerminalClientTestPair(t)
	firstDone := make(chan error, 1)
	go func() {
		_, err := runTerminalClientStreamWithInput(ctx, firstClient, inputReads, io.Discard, 0)
		firstDone <- err
	}()
	if err := firstServer.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseGoingAway, "daemon shutdown"),
		time.Now().Add(time.Second),
	); err != nil {
		t.Fatalf("first WriteControl(close) error = %v", err)
	}
	if err := <-firstDone; !terminalReconnectableError(err) {
		t.Fatalf("first stream error = %v, want reconnectable", err)
	}

	secondClient, secondServer := newTerminalClientTestPair(t)
	secondDone := make(chan error, 1)
	go func() {
		_, err := runTerminalClientStreamWithInput(ctx, secondClient, inputReads, io.Discard, 0)
		secondDone <- err
	}()
	if _, err := writer.Write([]byte("continued")); err != nil {
		t.Fatalf("write replacement input: %v", err)
	}
	frame := readTerminalClientTestFrame(t, secondServer)
	if frame.Op != terminalwire.ClientOpInput || string(frame.Payload) != "continued" {
		t.Fatalf("replacement input frame = %#v", frame)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close replacement input: %v", err)
	}
	if frame := readTerminalClientTestFrame(t, secondServer); frame.Op != terminalwire.ClientOpDetach {
		t.Fatalf("replacement EOF opcode = %d, want DETACH", frame.Op)
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second stream error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second stream did not finish after EOF")
	}
}

type terminalClientTestUpgrade struct {
	conn *websocket.Conn
	err  error
}

func newTerminalClientTestPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	upgraded := make(chan terminalClientTestUpgrade, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		upgraded <- terminalClientTestUpgrade{conn: conn, err: err}
	}))
	t.Cleanup(httpServer.Close)
	target := "ws" + strings.TrimPrefix(httpServer.URL, terminalClientHTTPProtocol)
	client, response, err := websocket.DefaultDialer.Dial(target, nil)
	if response != nil && response.Body != nil {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatalf("close upgrade response: %v", closeErr)
		}
	}
	if err != nil {
		t.Fatalf("dial test WebSocket: %v", err)
	}
	result := <-upgraded
	if result.err != nil {
		t.Fatalf("upgrade test WebSocket: %v", result.err)
	}
	t.Cleanup(func() {
		closeTerminalClientTestSocket(t, client)
		closeTerminalClientTestSocket(t, result.conn)
	})
	return client, result.conn
}

func readTerminalClientTestFrame(t *testing.T, conn *websocket.Conn) terminalwire.Frame {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set test read deadline: %v", err)
	}
	messageType, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read test client frame: %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("message type = %d, want binary", messageType)
	}
	frame, err := terminalwire.DecodeClient(payload)
	if err != nil {
		t.Fatalf("decode test client frame: %v", err)
	}
	return frame
}

func writeTerminalServerTestFrame(t *testing.T, conn *websocket.Conn, frame terminalwire.Frame) {
	t.Helper()
	payload, err := terminalwire.EncodeServer(frame)
	if err != nil {
		t.Fatalf("encode test server frame: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatalf("write test server frame: %v", err)
	}
}

func closeTerminalClientTestSocket(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if conn == nil {
		return
	}
	if err := conn.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		t.Errorf("close test WebSocket: %v", err)
	}
}
