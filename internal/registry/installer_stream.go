package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// installerDownloadStream owns a downloaded archive after Download succeeds.
// Its underlying reader must unblock a concurrent Read when Close is called.
type installerDownloadStream struct {
	reader io.ReadCloser

	closeMu sync.Mutex
	closed  bool
}

func newInstallerDownloadStream(reader io.ReadCloser) *installerDownloadStream {
	return &installerDownloadStream{reader: reader}
}

func (s *installerDownloadStream) Read(buffer []byte) (int, error) {
	if s == nil || s.reader == nil {
		return 0, errors.New("registry: download stream is required")
	}
	return s.reader.Read(buffer)
}

func (s *installerDownloadStream) Close() error {
	if s == nil || s.reader == nil {
		return nil
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.reader.Close()
}

type installerCopyResult struct {
	written int64
	err     error
}

func copyInstallStream(
	ctx context.Context,
	destination io.Writer,
	reader io.Reader,
	interrupter io.Closer,
) (int64, error) {
	result := make(chan installerCopyResult, 1)
	go func() {
		written, err := io.Copy(destination, reader)
		result <- installerCopyResult{written: written, err: err}
	}()

	select {
	case copyResult := <-result:
		if copyResult.err != nil {
			return copyResult.written, fmt.Errorf("registry: copy archive stream: %w", copyResult.err)
		}
		return copyResult.written, nil
	case <-ctx.Done():
		closeErr := interrupter.Close()
		copyResult := <-result
		if closeErr != nil {
			return copyResult.written, errors.Join(
				ctx.Err(),
				fmt.Errorf("registry: interrupt archive stream: %w", closeErr),
			)
		}
		return copyResult.written, ctx.Err()
	}
}

func readInstallStreamByte(ctx context.Context, reader io.Reader, interrupter io.Closer) (int, error) {
	var buffer [1]byte
	result := make(chan installerCopyResult, 1)
	go func() {
		read, err := reader.Read(buffer[:])
		result <- installerCopyResult{written: int64(read), err: err}
	}()

	select {
	case readResult := <-result:
		return int(readResult.written), readResult.err
	case <-ctx.Done():
		closeErr := interrupter.Close()
		<-result
		if closeErr != nil {
			return 0, errors.Join(
				ctx.Err(),
				fmt.Errorf("registry: interrupt archive stream: %w", closeErr),
			)
		}
		return 0, ctx.Err()
	}
}
