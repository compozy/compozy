package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

type manualMCPAuthReadResult struct {
	value string
	err   error
}

func readManualMCPAuthFileContext(
	ctx context.Context,
	file *os.File,
	readInput func() (string, error),
) (value string, resultErr error) {
	readDeadline := time.Time{}
	if deadline, ok := ctx.Deadline(); ok {
		readDeadline = deadline
	}
	if err := file.SetReadDeadline(readDeadline); err != nil {
		return readManualMCPAuthFileWithoutDeadline(ctx, readInput, err)
	}
	defer func() {
		if err := file.SetReadDeadline(time.Time{}); err != nil && !errors.Is(err, os.ErrClosed) {
			resetErr := fmt.Errorf("cli: clear MCP authorization input deadline: %w", err)
			resultErr = errors.Join(resultErr, resetErr)
		}
	}()

	results := make(chan manualMCPAuthReadResult, 1)
	go func() {
		readValue, err := readInput()
		results <- manualMCPAuthReadResult{value: readValue, err: err}
	}()
	select {
	case result := <-results:
		return manualMCPAuthReadResultValue(ctx, result)
	case <-ctx.Done():
		return "", interruptManualMCPAuthFileRead(ctx, file, results)
	}
}

func readManualMCPAuthFileWithoutDeadline(
	ctx context.Context,
	readInput func() (string, error),
	deadlineErr error,
) (string, error) {
	if !errors.Is(deadlineErr, os.ErrNoDeadline) {
		return "", fmt.Errorf("cli: set MCP authorization input deadline: %w", deadlineErr)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", mcpAuthorizationTimeoutError(ctxErr)
	}
	// Interrupting input without deadline support would require closing caller-owned input.
	value, err := readInput()
	if err != nil {
		return "", err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", mcpAuthorizationTimeoutError(ctxErr)
	}
	return value, nil
}

func manualMCPAuthReadResultValue(
	ctx context.Context,
	result manualMCPAuthReadResult,
) (string, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", mcpAuthorizationTimeoutError(ctxErr)
	}
	if errors.Is(result.err, os.ErrDeadlineExceeded) {
		return "", mcpAuthorizationTimeoutError(context.DeadlineExceeded)
	}
	return result.value, result.err
}

func interruptManualMCPAuthFileRead(
	ctx context.Context,
	file *os.File,
	results <-chan manualMCPAuthReadResult,
) error {
	timeoutErr := mcpAuthorizationTimeoutError(ctx.Err())
	if err := file.SetReadDeadline(time.Now()); err != nil {
		return errors.Join(
			timeoutErr,
			fmt.Errorf("cli: interrupt MCP authorization input: %w", err),
		)
	}
	result := <-results
	if result.err != nil && !errors.Is(result.err, os.ErrDeadlineExceeded) {
		return errors.Join(timeoutErr, result.err)
	}
	return timeoutErr
}
