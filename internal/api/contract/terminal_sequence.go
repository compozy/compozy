package contract

import (
	"fmt"
	"strconv"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

// TerminalSequence is the decimal uint64 representation used by terminal JSON contracts.
type TerminalSequence string

// TerminalSequenceFromUint64 returns the canonical decimal representation of sequence.
func TerminalSequenceFromUint64(sequence uint64) TerminalSequence {
	return TerminalSequence(strconv.FormatUint(sequence, 10))
}

// Uint64 validates and converts a terminal sequence received from a public boundary.
func (sequence TerminalSequence) Uint64() (uint64, error) {
	value := string(sequence)
	if value == "" || len(value) > int(terminalpkg.DecimalUint64MaxLength) {
		return 0, fmt.Errorf("terminal: sequence %q is not a decimal uint64", value)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || strconv.FormatUint(parsed, 10) != value {
		return 0, fmt.Errorf("terminal: sequence %q is not a decimal uint64", value)
	}
	return parsed, nil
}
