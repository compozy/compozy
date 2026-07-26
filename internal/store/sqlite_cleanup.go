package store

import (
	"errors"
	"fmt"
)

type sqliteCloser interface {
	Close() error
}

func closeSQLiteAfterError(closer sqliteCloser, primaryErr error, action string) error {
	if closer == nil {
		return primaryErr
	}
	if closeErr := closer.Close(); closeErr != nil {
		return errors.Join(primaryErr, fmt.Errorf("store: close sqlite %s: %w", action, closeErr))
	}
	return primaryErr
}
