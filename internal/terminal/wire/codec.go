package wire

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

func EncodeServer(frame Frame) ([]byte, error) {
	if !ValidOpcode(ServerToClient, frame.Op) {
		return nil, fmt.Errorf("%w: server opcode 0x%02x", ErrUnknownOpcode, frame.Op)
	}
	if frame.Op == ServerOpOutput {
		encoded := make([]byte, 1+8+len(frame.Payload))
		encoded[0] = frame.Op
		binary.BigEndian.PutUint64(encoded[1:9], frame.Seq)
		copy(encoded[9:], frame.Payload)
		return encoded, nil
	}
	if !json.Valid(frame.Payload) {
		return nil, fmt.Errorf("%w: server control payload is not JSON", ErrInvalidFrame)
	}
	return append([]byte{frame.Op}, frame.Payload...), nil
}

func DecodeServer(encoded []byte) (Frame, error) {
	if len(encoded) == 0 {
		return Frame{}, fmt.Errorf("%w: empty server frame", ErrInvalidFrame)
	}
	opcode := encoded[0]
	if !ValidOpcode(ServerToClient, opcode) {
		return Frame{}, fmt.Errorf("%w: server opcode 0x%02x", ErrUnknownOpcode, opcode)
	}
	if opcode == ServerOpOutput {
		if len(encoded) < 9 {
			return Frame{}, fmt.Errorf("%w: short OUTPUT frame", ErrInvalidFrame)
		}
		return Frame{
			Op:      opcode,
			Seq:     binary.BigEndian.Uint64(encoded[1:9]),
			Payload: append([]byte(nil), encoded[9:]...),
		}, nil
	}
	if !json.Valid(encoded[1:]) {
		return Frame{}, fmt.Errorf("%w: server control payload is not JSON", ErrInvalidFrame)
	}
	return Frame{Op: opcode, Payload: append([]byte(nil), encoded[1:]...)}, nil
}

func EncodeClient(frame Frame) ([]byte, error) {
	if !ValidOpcode(ClientToServer, frame.Op) {
		return nil, fmt.Errorf("%w: client opcode 0x%02x", ErrUnknownOpcode, frame.Op)
	}
	switch frame.Op {
	case ClientOpInput:
		if len(frame.Payload) > MaxInputBytes {
			return nil, ErrInputTooLarge
		}
		return append([]byte{frame.Op}, frame.Payload...), nil
	case ClientOpAck:
		if len(frame.Payload) != 4 {
			return nil, fmt.Errorf("%w: ACK payload must be four bytes", ErrInvalidFrame)
		}
		return append([]byte{frame.Op}, frame.Payload...), nil
	default:
		if !json.Valid(frame.Payload) {
			return nil, fmt.Errorf("%w: client control payload is not JSON", ErrInvalidFrame)
		}
		return append([]byte{frame.Op}, frame.Payload...), nil
	}
}

func DecodeClient(encoded []byte) (Frame, error) {
	if len(encoded) == 0 {
		return Frame{}, fmt.Errorf("%w: empty client frame", ErrInvalidFrame)
	}
	frame := Frame{Op: encoded[0], Payload: append([]byte(nil), encoded[1:]...)}
	if !ValidOpcode(ClientToServer, frame.Op) {
		return Frame{}, fmt.Errorf("%w: client opcode 0x%02x", ErrUnknownOpcode, frame.Op)
	}
	switch frame.Op {
	case ClientOpInput:
		if len(frame.Payload) > MaxInputBytes {
			return Frame{}, ErrInputTooLarge
		}
	case ClientOpAck:
		if len(frame.Payload) != 4 {
			return Frame{}, fmt.Errorf("%w: ACK payload must be four bytes", ErrInvalidFrame)
		}
	default:
		if !json.Valid(frame.Payload) {
			return Frame{}, fmt.Errorf("%w: client control payload is not JSON", ErrInvalidFrame)
		}
	}
	return frame, nil
}

func AckBytes(frame Frame) (uint32, error) {
	if frame.Op != ClientOpAck || len(frame.Payload) != 4 {
		return 0, fmt.Errorf("%w: expected ACK frame", ErrInvalidFrame)
	}
	return binary.BigEndian.Uint32(frame.Payload), nil
}

func NewACK(bytes uint32) Frame {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, bytes)
	return Frame{Op: ClientOpAck, Payload: payload}
}
