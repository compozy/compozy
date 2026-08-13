package update

import (
	"errors"
	"fmt"
	"io"
)

const maxUpdateResponseDrainBytes int64 = 64 << 10

func drainAndCloseUpdateResponseBody(body io.ReadCloser) error {
	if body == nil {
		return nil
	}

	_, drainErr := io.Copy(io.Discard, io.LimitReader(body, maxUpdateResponseDrainBytes))
	if drainErr != nil {
		drainErr = fmt.Errorf("update: drain HTTP response body: %w", drainErr)
	}
	closeErr := body.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("update: close HTTP response body: %w", closeErr)
	}
	return errors.Join(drainErr, closeErr)
}

func writeDownloadTarget(targetPath string, target io.WriteCloser, source io.Reader, maxBytes int64) (err error) {
	closed := false
	closeTarget := func() error {
		if closed {
			return nil
		}
		closed = true
		if closeErr := target.Close(); closeErr != nil {
			return fmt.Errorf("update: close download target %q: %w", targetPath, closeErr)
		}
		return nil
	}
	defer func() {
		err = errors.Join(err, closeTarget())
	}()

	if maxBytes <= 0 {
		return fmt.Errorf("update: invalid download limit %d for %q", maxBytes, targetPath)
	}
	limited := &io.LimitedReader{R: source, N: maxBytes + 1}
	written, copyErr := io.Copy(target, limited)
	if copyErr != nil {
		return fmt.Errorf("update: write download %q: %w", targetPath, copyErr)
	}
	if written > maxBytes {
		return &downloadSizeError{Path: targetPath, Size: written, Limit: maxBytes}
	}
	return closeTarget()
}
