package github

import (
	"context"

	"errors"
	"fmt"
	"io"

	"net/http"

	"os"

	"strings"

	"time"

	"github.com/compozy/agh/internal/registry"
)

func joinErrors(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return errors.Join(filtered...)
}

func sleepContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current time.Duration, maxDelay time.Duration) time.Duration {
	if current <= 0 {
		return defaultInitialBackoff
	}
	if maxDelay <= 0 {
		maxDelay = defaultMaxBackoff
	}

	next := current * 2
	if next > maxDelay {
		return maxDelay
	}
	return next
}

func spoolDownloadResponse(body io.Reader, slug string, maxBytes int64) (_ io.ReadCloser, size int64, err error) {
	file, err := os.CreateTemp("", "agh-github-download-*")
	if err != nil {
		return nil, 0, fmt.Errorf("create temp download file for %q: %w", slug, err)
	}
	defer func() {
		if err != nil {
			if removeErr := os.Remove(file.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove failed temp download file %q: %w", file.Name(), removeErr))
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
				"%w: github download for %q exceeds compressed archive limit %d",
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

func closeIdleConnections(httpClient *http.Client) {
	if httpClient == nil {
		return
	}
	if httpClient.Transport == nil {
		if transport, ok := http.DefaultTransport.(interface{ CloseIdleConnections() }); ok {
			transport.CloseIdleConnections()
		}
		return
	}
	if transport, ok := httpClient.Transport.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func requestURLString(response *http.Response) string {
	if response == nil || response.Request == nil || response.Request.URL == nil {
		return ""
	}
	return response.Request.URL.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
