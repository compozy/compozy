package wire

import "errors"

// Subprotocol is the permanent WebSocket protocol identifier for terminal frames.
const Subprotocol = "compozy.terminal.v1"

// Direction selects the opcode namespace used to validate a frame.
type Direction uint8

const (
	ServerToClient Direction = iota + 1
	ClientToServer
)

const (
	ServerOpOutput   byte = 0x01
	ServerOpAttached byte = 0x02
	ServerOpExit     byte = 0x03
	ServerOpError    byte = 0x04
	ServerOpTitle    byte = 0x05
	ServerOpResized  byte = 0x06
	ServerOpGap      byte = 0x07
	ServerOpOwner    byte = 0x08
)

const (
	ClientOpInput    byte = 0x01
	ClientOpAck      byte = 0x02
	ClientOpResize   byte = 0x03
	ClientOpSignal   byte = 0x04
	ClientOpTakeover byte = 0x05
	ClientOpDetach   byte = 0x06
)

const (
	MaxInputBytes = 64 << 10
	MinCols       = 20
	MaxCols       = 2000
	MinRows       = 5
	MaxRows       = 1000
)

var (
	ErrUnknownOpcode = errors.New("terminal wire: unknown opcode")
	ErrInvalidFrame  = errors.New("terminal wire: invalid frame")
	ErrInputTooLarge = errors.New("terminal wire: input frame exceeds 64 KiB")
)

type Frame struct {
	Op      byte
	Seq     uint64
	Payload []byte
}

// ValidOpcode reports whether opcode is assigned in the selected direction.
func ValidOpcode(direction Direction, opcode byte) bool {
	switch direction {
	case ServerToClient:
		return opcode >= ServerOpOutput && opcode <= ServerOpOwner
	case ClientToServer:
		return opcode >= ClientOpInput && opcode <= ClientOpDetach
	default:
		return false
	}
}

// ClampDimensions normalizes a non-zero terminal size to protocol bounds.
func ClampDimensions(cols, rows uint16) (uint16, uint16, bool) {
	if cols == 0 || rows == 0 {
		return 0, 0, false
	}
	return min(max(cols, MinCols), MaxCols), min(max(rows, MinRows), MaxRows), true
}
