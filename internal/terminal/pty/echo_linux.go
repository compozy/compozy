//go:build linux

package pty

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func (p *unixProc) EchoEnabled() (bool, error) {
	var enabled bool
	var ioctlErr error
	if err := p.device.Control(func(fd uintptr) {
		state, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
		if err != nil {
			ioctlErr = err
			return
		}
		enabled = state.Lflag&unix.ECHO != 0
	}); err != nil {
		return false, fmt.Errorf("terminal pty: inspect echo: %w", err)
	}
	if ioctlErr != nil {
		return false, fmt.Errorf("terminal pty: inspect echo: %w", ioctlErr)
	}
	return enabled, nil
}

func (p *unixProc) WriteRedacted(input []byte) (int, error) {
	state, err := p.setEcho(false)
	if err != nil {
		return 0, err
	}
	written, writeErr := p.Write(input)
	restoreErr := p.restoreTermios(state)
	return written, errors.Join(writeErr, restoreErr)
}

func (p *unixProc) setEcho(enabled bool) (*unix.Termios, error) {
	var prior *unix.Termios
	var ioctlErr error
	if err := p.device.Control(func(fd uintptr) {
		state, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
		if err != nil {
			ioctlErr = err
			return
		}
		prior = state
		next := *state
		if enabled {
			next.Lflag |= unix.ECHO
		} else {
			next.Lflag &^= unix.ECHO
		}
		ioctlErr = unix.IoctlSetTermios(int(fd), unix.TCSETS, &next)
	}); err != nil {
		return nil, fmt.Errorf("terminal pty: change echo: %w", err)
	}
	if ioctlErr != nil {
		return nil, fmt.Errorf("terminal pty: change echo: %w", ioctlErr)
	}
	return prior, nil
}

func (p *unixProc) restoreTermios(state *unix.Termios) error {
	var ioctlErr error
	if err := p.device.Control(func(fd uintptr) {
		ioctlErr = unix.IoctlSetTermios(int(fd), unix.TCSETS, state)
	}); err != nil {
		return fmt.Errorf("terminal pty: restore echo: %w", err)
	}
	if ioctlErr != nil {
		return fmt.Errorf("terminal pty: restore echo: %w", ioctlErr)
	}
	return nil
}
