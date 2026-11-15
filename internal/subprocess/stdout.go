package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type stdoutReader struct {
	file  *os.File
	close func() error
	eof   atomic.Bool
}

func newStdoutReader(file *os.File) *stdoutReader {
	return &stdoutReader{file: file, close: sync.OnceValue(file.Close)}
}

func (r *stdoutReader) Read(p []byte) (int, error) {
	if r.eof.Load() {
		return 0, io.EOF
	}
	n, err := r.file.Read(p)
	if err != nil {
		if errors.Is(err, io.EOF) {
			r.eof.Store(true)
		}
		if closeErr := r.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return n, err
}

func (p *Process) waitForStdout(readerDone <-chan struct{}) error {
	timer := time.NewTimer(defaultProcessGroupWait)
	defer timer.Stop()
	select {
	case <-readerDone:
		return nil
	case <-timer.C:
		closeErr := p.stdout.Close()
		<-readerDone
		return errors.Join(
			fmt.Errorf("subprocess: stdout remained open after process exit: %w", context.DeadlineExceeded),
			closeErr,
		)
	}
}

func (r *stdoutReader) Close() error {
	return r.close()
}

func closeCommandPipes(pipes ...io.Closer) error {
	var errs []error
	for _, pipe := range pipes {
		if err := pipe.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
