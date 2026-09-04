package wire

// Suite: terminal wire codec.
// Invariant: permanent opcode bytes round-trip and malformed frames are rejected at the protocol boundary.
// Boundary IN: encoded terminal frames. Boundary OUT: typed frames or protocol errors.

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestCodecShouldRoundTripFrozenOpcodes(t *testing.T) {
	t.Parallel()
	if Subprotocol != "compozy.terminal.v3" {
		t.Fatalf("Subprotocol = %q, want hard-cut v3", Subprotocol)
	}

	t.Run("Should round trip every server opcode", func(t *testing.T) {
		t.Parallel()
		for _, fixture := range []struct {
			opcode byte
			value  byte
		}{
			{ServerOpOutput, 0x01}, {ServerOpAttached, 0x02}, {ServerOpExit, 0x03},
			{ServerOpError, 0x04}, {ServerOpTitle, 0x05}, {ServerOpResized, 0x06},
			{ServerOpGap, 0x07}, {ServerOpPresence, 0x08}, {ServerOpRedactedInput, 0x09},
		} {
			opcode := fixture.opcode
			if opcode != fixture.value {
				t.Fatalf("server opcode = 0x%02x, want permanent value 0x%02x", opcode, fixture.value)
			}
			frame := Frame{Op: opcode, Payload: json.RawMessage(`{"ok":true}`)}
			if opcode == ServerOpOutput {
				frame.Seq, frame.Payload = 41, []byte("hello")
			}
			encoded, err := EncodeServer(frame)
			if err != nil {
				t.Fatalf("EncodeServer(0x%02x) error = %v", opcode, err)
			}
			decoded, err := DecodeServer(encoded)
			if err != nil {
				t.Fatalf("DecodeServer(0x%02x) error = %v", opcode, err)
			}
			if decoded.Op != frame.Op || decoded.Seq != frame.Seq || !bytes.Equal(decoded.Payload, frame.Payload) {
				t.Fatalf("server round trip = %#v, want %#v", decoded, frame)
			}
		}
	})

	t.Run("Should round trip every client opcode", func(t *testing.T) {
		t.Parallel()
		for _, fixture := range []struct {
			opcode byte
			value  byte
		}{
			{ClientOpInput, 0x01}, {ClientOpAck, 0x02}, {ClientOpResize, 0x03},
			{ClientOpSignal, 0x04}, {ClientOpDetach, 0x05},
		} {
			opcode := fixture.opcode
			if opcode != fixture.value {
				t.Fatalf("client opcode = 0x%02x, want permanent value 0x%02x", opcode, fixture.value)
			}
			frame := Frame{Op: opcode, Payload: json.RawMessage(`{}`)}
			switch opcode {
			case ClientOpInput:
				frame.Payload = []byte("input")
			case ClientOpAck:
				frame = NewACK(16384)
			}
			encoded, err := EncodeClient(frame)
			if err != nil {
				t.Fatalf("EncodeClient(0x%02x) error = %v", opcode, err)
			}
			decoded, err := DecodeClient(encoded)
			if err != nil {
				t.Fatalf("DecodeClient(0x%02x) error = %v", opcode, err)
			}
			if decoded.Op != frame.Op || !bytes.Equal(decoded.Payload, frame.Payload) {
				t.Fatalf("client round trip = %#v, want %#v", decoded, frame)
			}
		}
	})
}

func TestCodecShouldRejectInvalidFrames(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unknown opcode", func(t *testing.T) {
		t.Parallel()
		if _, err := DecodeServer([]byte{0xff, '{', '}'}); !errors.Is(err, ErrUnknownOpcode) {
			t.Fatalf("DecodeServer() error = %v, want ErrUnknownOpcode", err)
		}
		if _, err := DecodeClient([]byte{0xff, '{', '}'}); !errors.Is(err, ErrUnknownOpcode) {
			t.Fatalf("DecodeClient() error = %v, want ErrUnknownOpcode", err)
		}
	})

	t.Run("Should reject input above sixty four KiB", func(t *testing.T) {
		t.Parallel()
		encoded := append([]byte{ClientOpInput}, make([]byte, MaxInputBytes+1)...)
		if _, err := DecodeClient(encoded); !errors.Is(err, ErrInputTooLarge) {
			t.Fatalf("DecodeClient() error = %v, want ErrInputTooLarge", err)
		}
	})

	t.Run("Should reject malformed control and ACK frames", func(t *testing.T) {
		t.Parallel()
		for _, testCase := range []struct {
			name string
			run  func() error
		}{
			{name: "server encode JSON", run: func() error { _, err := EncodeServer(Frame{Op: ServerOpExit, Payload: []byte("{")}); return err }},
			{name: "server decode JSON", run: func() error { _, err := DecodeServer([]byte{ServerOpExit, '{'}); return err }},
			{name: "client encode JSON", run: func() error { _, err := EncodeClient(Frame{Op: ClientOpResize, Payload: []byte("{")}); return err }},
			{name: "client decode JSON", run: func() error { _, err := DecodeClient([]byte{ClientOpResize, '{'}); return err }},
			{name: "short ACK encode", run: func() error { _, err := EncodeClient(Frame{Op: ClientOpAck, Payload: make([]byte, 3)}); return err }},
			{name: "long ACK encode", run: func() error { _, err := EncodeClient(Frame{Op: ClientOpAck, Payload: make([]byte, 5)}); return err }},
			{name: "short ACK decode", run: func() error { _, err := DecodeClient([]byte{ClientOpAck, 0, 0, 0}); return err }},
			{name: "long ACK decode", run: func() error { _, err := DecodeClient([]byte{ClientOpAck, 0, 0, 0, 0, 0}); return err }},
		} {
			t.Run("Should reject "+testCase.name, func(t *testing.T) {
				t.Parallel()
				if err := testCase.run(); !errors.Is(err, ErrInvalidFrame) {
					t.Fatalf("error = %v, want ErrInvalidFrame", err)
				}
			})
		}
	})

	t.Run("Should clamp dimensions and ignore zeros", func(t *testing.T) {
		t.Parallel()
		cols, rows, ok := ClampDimensions(1, 65535)
		if !ok || cols != MinCols || rows != MaxRows {
			t.Fatalf("ClampDimensions() = (%d,%d,%v)", cols, rows, ok)
		}
		if _, _, ok := ClampDimensions(0, 24); ok {
			t.Fatal("ClampDimensions(0,24) accepted a zero dimension")
		}
	})
}
