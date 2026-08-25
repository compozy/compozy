package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestCodecShouldRoundTripFrozenOpcodes(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip every server opcode", func(t *testing.T) {
		t.Parallel()
		for opcode := ServerOpOutput; opcode <= ServerOpOwner; opcode++ {
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
		for opcode := ClientOpInput; opcode <= ClientOpDetach; opcode++ {
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
