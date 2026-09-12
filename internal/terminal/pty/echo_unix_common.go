//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package pty

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func (p *unixProc) InputVisible() (bool, error) {
	if err := p.io.begin(); err != nil {
		return false, err
	}
	defer p.io.end()
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	return p.inputVisibleLocked()
}

func (p *unixProc) inputVisibleLocked() (bool, error) {
	state, err := p.readTermios()
	if err != nil {
		return false, err
	}
	// Raw line editors render their own input regardless of foreground process group.
	return state.Lflag&unix.ECHO != 0 || state.Lflag&unix.ICANON == 0, nil
}

func (p *unixProc) InputEchoEnabled() (bool, error) {
	if err := p.io.begin(); err != nil {
		return false, err
	}
	defer p.io.end()
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	state, err := p.readTermios()
	if err != nil {
		return false, err
	}
	return state.Lflag&unix.ECHO != 0, nil
}

func (p *unixProc) WriteRedacted(input []byte) (RedactedWriteResult, error) {
	if err := p.io.begin(); err != nil {
		return RedactedWriteResult{}, err
	}
	defer p.io.end()
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	state, err := p.readTermios()
	if err != nil {
		return RedactedWriteResult{}, err
	}
	if state.Lflag&unix.ECHO != 0 {
		return RedactedWriteResult{}, ErrInputVisible
	}
	written, writeErr := writeAllRedactedBytes(input, p.write)
	return RedactedWriteResult{BytesDelivered: written}, writeErr
}

func (p *unixProc) readTermios() (*unix.Termios, error) {
	var state *unix.Termios
	if err := p.controlTerminal(func(fd int) error {
		var err error
		state, err = getTermios(fd)
		return err
	}); err != nil {
		return nil, fmt.Errorf("terminal pty: inspect echo: %w", err)
	}
	return state, nil
}

func (p *unixProc) restoreTermios(state *unix.Termios) error {
	return p.writeTermios(state, "restore")
}

func (p *unixProc) writeTermios(state *unix.Termios, action string) error {
	if err := p.controlTerminal(func(fd int) error { return setTermios(fd, state) }); err != nil {
		return fmt.Errorf("terminal pty: %s echo: %w", action, err)
	}
	return nil
}

func (p *unixProc) controlTerminal(operation func(int) error) error {
	if slave := p.device.Slave(); slave != nil {
		connection, err := slave.SyscallConn()
		if err == nil {
			var operationErr error
			controlErr := connection.Control(func(fd uintptr) { operationErr = operation(int(fd)) })
			if controlErr == nil && operationErr == nil {
				return nil
			}
		}
	}
	var operationErr error
	if err := p.device.Control(func(fd uintptr) { operationErr = operation(int(fd)) }); err != nil {
		return err
	}
	return operationErr
}
