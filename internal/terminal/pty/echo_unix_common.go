//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package pty

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func (p *unixProc) EchoEnabled() (bool, error) {
	state, err := p.readTermios()
	if err != nil {
		return false, err
	}
	return state.Lflag&unix.ECHO != 0, nil
}

func (p *unixProc) WriteRedacted(input []byte) (int, error) {
	state, err := p.setEcho(false)
	if err != nil {
		return 0, err
	}
	written, writeErr := p.Write(input)
	return written, errors.Join(writeErr, p.restoreTermios(state))
}

func (p *unixProc) readTermios() (*unix.Termios, error) {
	var state *unix.Termios
	var ioctlErr error
	if err := p.device.Control(func(fd uintptr) {
		state, ioctlErr = getTermios(int(fd))
	}); err != nil {
		return nil, fmt.Errorf("terminal pty: inspect echo: %w", err)
	}
	if ioctlErr != nil {
		return nil, fmt.Errorf("terminal pty: inspect echo: %w", ioctlErr)
	}
	return state, nil
}

func (p *unixProc) setEcho(enabled bool) (*unix.Termios, error) {
	prior, err := p.readTermios()
	if err != nil {
		return nil, err
	}
	next := *prior
	if enabled {
		next.Lflag |= unix.ECHO
	} else {
		next.Lflag &^= unix.ECHO
	}
	if err := p.writeTermios(&next, "change"); err != nil {
		return nil, err
	}
	return prior, nil
}

func (p *unixProc) restoreTermios(state *unix.Termios) error {
	return p.writeTermios(state, "restore")
}

func (p *unixProc) writeTermios(state *unix.Termios, action string) error {
	var ioctlErr error
	if err := p.device.Control(func(fd uintptr) {
		ioctlErr = setTermios(int(fd), state)
	}); err != nil {
		return fmt.Errorf("terminal pty: %s echo: %w", action, err)
	}
	if ioctlErr != nil {
		return fmt.Errorf("terminal pty: %s echo: %w", action, ioctlErr)
	}
	return nil
}
