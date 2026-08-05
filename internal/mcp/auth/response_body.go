package auth

import (
	"errors"
	"fmt"
	"io"
)

const maxAuthResponseDrainBytes = int64(64 << 10)

func drainAndCloseResponseBody(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	var cleanupErr error
	if _, err := io.Copy(io.Discard, io.LimitReader(body, maxAuthResponseDrainBytes)); err != nil {
		cleanupErr = fmt.Errorf("mcp auth: drain HTTP response body: %w", err)
	}
	if err := body.Close(); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("mcp auth: close HTTP response body: %w", err))
	}
	return cleanupErr
}
