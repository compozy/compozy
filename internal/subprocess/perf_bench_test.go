package subprocess

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

type discardWriteCloser struct {
	io.Writer
}

func (discardWriteCloser) Close() error {
	return nil
}

func BenchmarkTransportWriteJSONRequest(b *testing.B) {
	process := &Process{stdin: discardWriteCloser{Writer: io.Discard}}
	transport := newTransport(process, defaultMaxMessageBytes)
	request := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      json.RawMessage(`42`),
		Method:  "echo",
		Params: struct {
			Message string `json:"message"`
			DelayMS int64  `json:"delay_ms,omitempty"`
		}{
			Message: "hello",
			DelayMS: 25,
		},
	}

	b.ReportAllocs()

	for b.Loop() {
		if err := transport.writeJSON(request); err != nil {
			b.Fatalf("writeJSON() error = %v", err)
		}
	}
}

func BenchmarkParseRPCIDNumeric(b *testing.B) {
	raw := json.RawMessage(`123456789`)

	b.ReportAllocs()

	for b.Loop() {
		id, err := parseRPCID(raw)
		if err != nil {
			b.Fatalf("parseRPCID() error = %v", err)
		}
		if id.key != "n:123456789" {
			b.Fatalf("parseRPCID() key = %q", id.key)
		}
	}
}

func BenchmarkBoundedBufferWriteOverflow(b *testing.B) {
	prefill := bytes.Repeat([]byte("a"), 6*1024)
	payload := bytes.Repeat([]byte("x"), 4*1024)

	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		buffer := &boundedBuffer{
			buf:   append([]byte(nil), prefill...),
			limit: 8 * 1024,
		}
		b.StartTimer()

		if _, err := buffer.Write(payload); err != nil {
			b.Fatalf("boundedBuffer.Write() error = %v", err)
		}
		if got, want := len(buffer.buf), buffer.limit; got != want {
			b.Fatalf("len(buffer.buf) = %d, want %d", got, want)
		}
	}
}
