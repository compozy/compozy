//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package pty

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func (p *unixProc) InputVisible() (bool, error) {
	state, err := p.readTermios()
	if err != nil {
		return false, err
	}
	if state.Lflag&unix.ECHO != 0 {
		return true, nil
	}
	if state.Lflag&unix.ICANON != 0 {
		return false, nil
	}
	foregroundGroup, err := p.foregroundProcessGroup()
	if err != nil {
		return false, err
	}
	return foregroundGroup == p.ProcessGroupID(), nil
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
	if err := p.controlTerminal(func(fd int) error {
		var err error
		state, err = getTermios(fd)
		return err
	}); err != nil {
		return nil, fmt.Errorf("terminal pty: inspect echo: %w", err)
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
	if err := p.controlTerminal(func(fd int) error { return setTermios(fd, state) }); err != nil {
		return fmt.Errorf("terminal pty: %s echo: %w", action, err)
	}
	return nil
}

func (p *unixProc) foregroundProcessGroup() (int, error) {
	foregroundGroup := 0
	if err := p.controlTerminal(func(fd int) error {
		var err error
		foregroundGroup, err = unix.IoctlGetInt(fd, unix.TIOCGPGRP)
		return err
	}); err != nil {
		return 0, fmt.Errorf("terminal pty: inspect foreground process group: %w", err)
	}
	return foregroundGroup, nil
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
