//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func prepareCancellableMCPAuthInput(file *os.File) (*os.File, func() error, error) {
	rawConnection, err := file.SyscallConn()
	if err != nil {
		return nil, nil, fmt.Errorf("cli: access MCP authorization input descriptor: %w", err)
	}

	originalFlags := 0
	duplicateFD := -1
	var operationErr error
	controlErr := rawConnection.Control(func(fd uintptr) {
		originalFlags, operationErr = unix.FcntlInt(fd, unix.F_GETFL, 0)
		if operationErr != nil {
			return
		}
		duplicateFD, operationErr = unix.FcntlInt(fd, unix.F_DUPFD_CLOEXEC, 0)
		if operationErr != nil {
			return
		}
		// A duplicated descriptor shares file-status flags with the original, so cleanup must restore them.
		operationErr = unix.SetNonblock(duplicateFD, true)
	})
	if err := errors.Join(
		wrapMCPAuthInputDescriptorError("inspect", controlErr),
		wrapMCPAuthInputDescriptorError("prepare", operationErr),
	); err != nil {
		return nil, nil, errors.Join(err, closeMCPAuthInputDescriptor(duplicateFD))
	}

	duplicate := os.NewFile(uintptr(duplicateFD), file.Name()+"-cancellable")
	if duplicate == nil {
		return nil, nil, errors.Join(
			errors.New("cli: create cancellable MCP authorization input file"),
			restoreMCPAuthInputFlags(rawConnection, originalFlags),
			closeMCPAuthInputDescriptor(duplicateFD),
		)
	}
	if err := duplicate.SetReadDeadline(time.Time{}); err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("cli: initialize cancellable MCP authorization input deadline: %w", err),
			wrapMCPAuthInputDescriptorError("close duplicate", duplicate.Close()),
			restoreMCPAuthInputFlags(rawConnection, originalFlags),
		)
	}

	cleanup := func() error {
		return errors.Join(
			wrapMCPAuthInputDescriptorError("close duplicate", duplicate.Close()),
			restoreMCPAuthInputFlags(rawConnection, originalFlags),
		)
	}
	return duplicate, cleanup, nil
}

func restoreMCPAuthInputFlags(rawConnection syscall.RawConn, flags int) error {
	var operationErr error
	controlErr := rawConnection.Control(func(fd uintptr) {
		_, operationErr = unix.FcntlInt(fd, unix.F_SETFL, flags)
	})
	return errors.Join(
		wrapMCPAuthInputDescriptorError("access for restore", controlErr),
		wrapMCPAuthInputDescriptorError("restore flags", operationErr),
	)
}

func closeMCPAuthInputDescriptor(fd int) error {
	if fd < 0 {
		return nil
	}
	return wrapMCPAuthInputDescriptorError("close duplicate", unix.Close(fd))
}

func wrapMCPAuthInputDescriptorError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: %s MCP authorization input descriptor: %w", action, err)
}
