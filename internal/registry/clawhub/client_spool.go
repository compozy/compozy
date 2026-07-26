package clawhub

import (
	"errors"
	"fmt"
	"io"

	"os"

	"github.com/compozy/agh/internal/registry"
)

func spoolDownloadResponse(body io.Reader, slug string, maxBytes int64) (_ io.ReadCloser, size int64, err error) {
	file, err := os.CreateTemp("", "agh-clawhub-download-*")
	if err != nil {
		return nil, 0, fmt.Errorf("create temp download file for %q: %w", slug, err)
	}
	defer func() {
		if err != nil {
			if removeErr := os.Remove(file.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove temp download file %q: %w", file.Name(), removeErr))
			}
		}
	}()

	limit := normalizeArchiveSizeLimit(maxBytes)
	written, err := io.Copy(file, io.LimitReader(body, limit+1))
	if err != nil {
		closeErr := file.Close()
		return nil, 0, joinErrors(
			fmt.Errorf("write temp download file for %q: %w", slug, err),
			closeErr,
		)
	}
	if written > limit {
		closeErr := file.Close()
		return nil, written, joinErrors(
			fmt.Errorf(
				"%w: clawhub download for %q exceeds compressed archive limit %d",
				registry.ErrArchiveTooLargeCompressed,
				slug,
				limit,
			),
			closeErr,
		)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		closeErr := file.Close()
		return nil, 0, joinErrors(
			fmt.Errorf("rewind temp download file for %q: %w", slug, err),
			closeErr,
		)
	}

	return &tempFileReadCloser{File: file, path: file.Name()}, written, nil
}

func normalizeArchiveSizeLimit(limit int64) int64 {
	if limit > 0 {
		return limit
	}
	return registry.DefaultMaxArchiveSize
}

func wrapSearchCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("clawhub: close search response: %w", err)
}

func wrapCloseError(slug string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("clawhub: close download response for %q: %w", slug, err)
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return -1
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type tempFileReadCloser struct {
	*os.File
	path string
}

func (r *tempFileReadCloser) Close() error {
	if r == nil {
		return nil
	}
	closeErr := r.File.Close()
	removeErr := os.Remove(r.path)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return errors.Join(closeErr, fmt.Errorf("remove temp download file %q: %w", r.path, removeErr))
	}
	return closeErr
}
