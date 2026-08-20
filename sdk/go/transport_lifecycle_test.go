package compozysdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestStdioTransportLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should keep lazy transport read loop alive after first call cancellation", func(t *testing.T) {
		t.Parallel()

		input := newControlledLineReader()
		outputReader, outputWriter := io.Pipe()
		transport := NewStdioTransport(StdioTransportOptions{
			Input:  input,
			Output: outputWriter,
		})
		t.Cleanup(func() {
			if err := transport.Close(); err != nil {
				t.Errorf("transport.Close() error = %v", err)
			}
			input.Close()
			closeTestPipe(t, outputReader)
			closeTestPipe(t, outputWriter)
		})

		firstCtx, cancelFirst := context.WithCancel(context.Background())
		firstErr := make(chan error, 1)
		go func() {
			var result map[string]string
			firstErr <- transport.Call(firstCtx, "slow", nil, &result)
		}()
		input.WaitForRead(t)
		firstRequest := readTestMessage(t, bufio.NewReader(outputReader))
		firstID := numericMessageID(t, firstRequest)
		cancelFirst()
		select {
		case err := <-firstErr:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("first Call() error = %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("first Call() did not return after cancellation")
		}
		cancelRequest := readTestMessage(t, bufio.NewReader(outputReader))
		if got, want := cancelRequest["method"], jsonRPCCancelMethod; got != want {
			t.Fatalf("cancel method = %#v, want %q", got, want)
		}
		params, ok := cancelRequest["params"].(map[string]any)
		cancelID, idOK := params["id"].(float64)
		if !ok || !idOK || int(cancelID) != firstID {
			t.Fatalf("cancel params = %#v, want id %d", cancelRequest["params"], firstID)
		}
		input.WriteLine(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"value":"ignored"}}`, firstID))
		select {
		case <-input.readStarted:
		case <-transport.done:
			t.Fatal("transport closed after first call cancellation; want read loop to stay alive")
		case <-time.After(time.Second):
			t.Fatal("transport did not continue reading after first call cancellation")
		}

		secondErr := make(chan error, 1)
		go func() {
			var result map[string]string
			if err := transport.Call(
				context.Background(),
				"echo",
				map[string]string{"value": "beta"},
				&result,
			); err != nil {
				secondErr <- err
				return
			}
			if got, want := result["echo"], "beta"; got != want {
				secondErr <- fmt.Errorf("second Call() response = %#v, want echo %q", result, want)
				return
			}
			secondErr <- nil
		}()
		secondRequest := readTestMessage(t, bufio.NewReader(outputReader))
		secondID := numericMessageID(t, secondRequest)
		input.WriteLine(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"echo":"beta"}}`, secondID))
		select {
		case err := <-secondErr:
			if err != nil {
				t.Fatalf("second Call() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("second Call() did not return")
		}
	})

	t.Run("Should reject an oversized unterminated frame when it crosses the limit", func(t *testing.T) {
		t.Parallel()

		inputReader, inputWriter := io.Pipe()
		transport := NewStdioTransport(StdioTransportOptions{
			Input:           inputReader,
			Output:          io.Discard,
			MaxMessageBytes: 64,
		})
		t.Cleanup(func() {
			closeTestPipe(t, inputWriter)
			closeTestPipe(t, inputReader)
		})

		runErr := make(chan error, 1)
		go func() {
			runErr <- transport.Run(context.Background())
		}()
		writeErr := make(chan error, 1)
		go func() {
			_, err := inputWriter.Write(bytes.Repeat([]byte("x"), 65))
			writeErr <- err
		}()

		select {
		case err := <-writeErr:
			if err != nil {
				t.Fatalf("write oversized frame error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("oversized frame writer remained blocked")
		}

		select {
		case err := <-runErr:
			if err == nil || err.Error() != "message exceeds 64 bytes" {
				t.Fatalf("Run() error = %v, want message size error", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Run() did not reject oversized frame before newline or EOF")
		}
	})

	t.Run("Should preserve a final partial frame at EOF", func(t *testing.T) {
		t.Parallel()

		reader := newTransportReader(bytes.NewBufferString("partial"), 64)
		line, err := readBoundedTransportLine(reader, 64)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("readBoundedTransportLine() error = %v, want io.EOF", err)
		}
		if got, want := string(line), "partial"; got != want {
			t.Fatalf("readBoundedTransportLine() = %q, want %q", got, want)
		}
	})
}

type controlledLineReader struct {
	lines       chan []byte
	readStarted chan struct{}
}

func newControlledLineReader() *controlledLineReader {
	return &controlledLineReader{
		lines:       make(chan []byte, 4),
		readStarted: make(chan struct{}, 4),
	}
}

func (r *controlledLineReader) Read(p []byte) (int, error) {
	r.readStarted <- struct{}{}
	line, ok := <-r.lines
	if !ok {
		return 0, io.EOF
	}
	copy(p, line)
	return len(line), nil
}

func (r *controlledLineReader) WaitForRead(t *testing.T) {
	t.Helper()

	select {
	case <-r.readStarted:
	case <-time.After(time.Second):
		t.Fatal("transport read loop did not start reading")
	}
}

func (r *controlledLineReader) WriteLine(t *testing.T, line string) {
	t.Helper()

	select {
	case r.lines <- []byte(line + "\n"):
	case <-time.After(time.Second):
		t.Fatal("transport test reader did not accept line")
	}
}

func (r *controlledLineReader) Close() {
	close(r.lines)
}

func readTestMessage(t *testing.T, reader *bufio.Reader) map[string]any {
	t.Helper()

	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read message error = %v", err)
	}
	var message map[string]any
	if err := json.Unmarshal(line, &message); err != nil {
		t.Fatalf("json.Unmarshal(message %s) error = %v", string(line), err)
	}
	return message
}

func numericMessageID(t *testing.T, message map[string]any) int {
	t.Helper()

	id, ok := message["id"].(float64)
	if !ok {
		t.Fatalf("message id = %#v, want numeric id", message["id"])
	}
	return int(id)
}

func closeTestPipe(t *testing.T, pipe interface{ Close() error }) {
	t.Helper()

	if err := pipe.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("pipe.Close() error = %v", err)
	}
}
