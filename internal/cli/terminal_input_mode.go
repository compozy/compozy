package cli

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/term"
)

type terminalInputFD interface {
	Fd() uintptr
}

type terminalModeOperations struct {
	isTerminal func(int) bool
	makeRaw    func(int) (*term.State, error)
	restore    func(int, *term.State) error
}

var platformTerminalModeOperations = terminalModeOperations{
	isTerminal: term.IsTerminal,
	makeRaw:    term.MakeRaw,
	restore:    term.Restore,
}

func withTerminalRawInput(input io.Reader, run func() error) error {
	return withTerminalRawInputMode(input, platformTerminalModeOperations, run)
}

func withTerminalRawInputMode(
	input io.Reader,
	operations terminalModeOperations,
	run func() error,
) (returnErr error) {
	fdInput, ok := input.(terminalInputFD)
	if !ok || !operations.isTerminal(int(fdInput.Fd())) {
		return run()
	}
	fd := int(fdInput.Fd())
	state, err := operations.makeRaw(fd)
	if err != nil {
		return fmt.Errorf("cli: enter terminal raw mode: %w", err)
	}
	defer func() {
		if restoreErr := operations.restore(fd, state); restoreErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("cli: restore terminal mode: %w", restoreErr))
		}
	}()
	return run()
}
