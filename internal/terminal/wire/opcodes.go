package wire

import "errors"

// Direction selects the opcode namespace used to validate a frame.
type Direction uint8

const (
	ServerToClient Direction = iota + 1
	ClientToServer
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
		return opcode >= ServerOpOutput && opcode <= ServerOpRedactedInput
	case ClientToServer:
		return opcode >= ClientOpInput && opcode <= ClientOpRelease
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
