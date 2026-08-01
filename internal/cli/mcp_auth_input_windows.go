//go:build windows

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

func prepareCancellableMCPAuthInput(file *os.File) (*os.File, func() error, error) {
	rawConnection, err := file.SyscallConn()
	if err != nil {
		return nil, nil, fmt.Errorf("cli: access MCP authorization input handle: %w", err)
	}

	var duplicateHandle windows.Handle
	var duplicateErr error
	controlErr := rawConnection.Control(func(handle uintptr) {
		process := windows.CurrentProcess()
		duplicateErr = windows.DuplicateHandle(
			process,
			windows.Handle(handle),
			process,
			&duplicateHandle,
			0,
			false,
			windows.DUPLICATE_SAME_ACCESS,
		)
	})
	if err := errors.Join(
		wrapMCPAuthInputHandleError("access", controlErr),
		wrapMCPAuthInputHandleError("duplicate", duplicateErr),
	); err != nil {
		return nil, nil, err
	}

	duplicate := os.NewFile(uintptr(duplicateHandle), file.Name()+"-cancellable-source")
	if duplicate == nil {
		return nil, nil, errors.Join(
			errors.New("cli: create cancellable MCP authorization input source"),
			wrapMCPAuthInputHandleError("close duplicate", windows.CloseHandle(duplicateHandle)),
		)
	}
	bridgeReader, bridgeWriter, err := os.Pipe()
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("cli: create cancellable MCP authorization input bridge: %w", err),
			wrapMCPAuthInputHandleError("close duplicate", duplicate.Close()),
		)
	}

	copyResult := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(bridgeWriter, duplicate)
		copyResult <- errors.Join(copyErr, bridgeWriter.Close())
	}()

	cleanup := func() error {
		cancelErr := windows.CancelIoEx(duplicateHandle, nil)
		if errors.Is(cancelErr, windows.ERROR_NOT_FOUND) {
			cancelErr = nil
		}
		duplicateCloseErr := duplicate.Close()
		copyErr := <-copyResult
		if errors.Is(copyErr, windows.ERROR_OPERATION_ABORTED) || errors.Is(copyErr, os.ErrClosed) {
			copyErr = nil
		}
		return errors.Join(
			wrapMCPAuthInputHandleError("cancel duplicate", cancelErr),
			wrapMCPAuthInputHandleError("close duplicate", duplicateCloseErr),
			wrapMCPAuthInputHandleError("copy bridge", copyErr),
			wrapMCPAuthInputHandleError("close bridge", bridgeReader.Close()),
		)
	}
	return bridgeReader, cleanup, nil
}

func wrapMCPAuthInputHandleError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: %s MCP authorization input handle: %w", action, err)
}
