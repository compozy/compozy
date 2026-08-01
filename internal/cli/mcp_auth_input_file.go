package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

func readManualMCPAuthFileContext(
	ctx context.Context,
	file *os.File,
	readInput func(io.Reader) (string, error),
) (string, error) {
	if err := file.SetReadDeadline(time.Time{}); err != nil {
		if !errors.Is(err, os.ErrNoDeadline) {
			return "", fmt.Errorf("cli: prepare MCP authorization input deadline: %w", err)
		}
		info, statErr := file.Stat()
		if statErr != nil {
			return "", errors.Join(
				fmt.Errorf("cli: inspect MCP authorization input: %w", statErr),
				fmt.Errorf("cli: prepare MCP authorization input deadline: %w", err),
			)
		}
		if info.Mode().IsRegular() {
			value, readErr := readInput(file)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", errors.Join(mcpAuthorizationTimeoutError(ctxErr), readErr)
			}
			return value, readErr
		}
		cancellableFile, cleanup, prepareErr := prepareCancellableMCPAuthInput(file)
		if prepareErr != nil {
			return "", errors.Join(
				fmt.Errorf("cli: prepare cancellable MCP authorization input: %w", prepareErr),
				fmt.Errorf("cli: prepare MCP authorization input deadline: %w", err),
			)
		}
		value, readErr := readManualMCPAuthDeadlineFileContext(ctx, cancellableFile, readInput)
		cleanupErr := cleanup()
		return value, errors.Join(readErr, wrapMCPAuthInputDeadlineError("cleanup", cleanupErr))
	}
	return readManualMCPAuthDeadlineFileContext(ctx, file, readInput)
}

func readManualMCPAuthDeadlineFileContext(
	ctx context.Context,
	file *os.File,
	readInput func(io.Reader) (string, error),
) (string, error) {
	readComplete := make(chan struct{})
	interruptResult := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			interruptResult <- file.SetReadDeadline(time.Now())
		case <-readComplete:
			interruptResult <- nil
		}
	}()

	value, readErr := readInput(file)
	close(readComplete)
	interruptErr := wrapMCPAuthInputDeadlineError("interrupt", <-interruptResult)
	resetErr := wrapMCPAuthInputDeadlineError("reset", file.SetReadDeadline(time.Time{}))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", errors.Join(
			mcpAuthorizationTimeoutError(ctxErr),
			unexpectedMCPAuthInputReadError(readErr),
			interruptErr,
			resetErr,
		)
	}
	if errors.Is(readErr, os.ErrDeadlineExceeded) {
		return "", errors.Join(
			mcpAuthorizationTimeoutError(context.DeadlineExceeded),
			interruptErr,
			resetErr,
		)
	}
	return value, errors.Join(readErr, interruptErr, resetErr)
}

func unexpectedMCPAuthInputReadError(err error) error {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return nil
	}
	return err
}

func wrapMCPAuthInputDeadlineError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: %s MCP authorization input deadline: %w", action, err)
}
