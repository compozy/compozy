package registry

import (
	"errors"
	"fmt"
	"io"
)

func closeDownloadReader(reader io.ReadCloser, slug string) error {
	if reader == nil {
		return nil
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("registry: close download stream for %q: %w", slug, err)
	}
	return nil
}

func joinInstallerError(base error, extra error) error {
	if extra == nil {
		return base
	}
	if base == nil {
		return extra
	}
	return errors.Join(base, extra)
}

func (r *countingReader) Read(p []byte) (int, error) {
	if r == nil {
		return 0, errors.New("registry: counting reader is required")
	}
	n, err := r.reader.Read(p)
	r.total += int64(n)
	return n, err
}
