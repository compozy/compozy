package fileutil

import (
	"errors"
	"fmt"
)

type closeOwner struct {
	closeResource func() error
}

func newCloseOwner(closeResource func() error) closeOwner {
	return closeOwner{closeResource: closeResource}
}

func (o *closeOwner) closeWithError(primary error, operation string) error {
	if o.closeResource == nil {
		return primary
	}

	closeResource := o.closeResource
	// Consume ownership before Close because a failed Close must not be retried.
	o.closeResource = nil
	closeErr := closeResource()
	if closeErr == nil {
		return primary
	}

	wrapped := fmt.Errorf("%s: %w", operation, closeErr)
	if primary == nil {
		return wrapped
	}
	return errors.Join(primary, wrapped)
}
