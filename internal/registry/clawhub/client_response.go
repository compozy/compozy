package clawhub

import (
	"errors"
	"fmt"
	"io"
)

func drainAndCloseResponseBody(body io.ReadCloser, context string) error {
	if body == nil {
		return nil
	}
	var cleanupErr error
	if _, err := io.Copy(io.Discard, io.LimitReader(body, maxErrorBodyBytes)); err != nil {
		cleanupErr = fmt.Errorf("clawhub: drain %s: %w", context, err)
	}
	if err := body.Close(); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("clawhub: close %s: %w", context, err))
	}
	return cleanupErr
}
