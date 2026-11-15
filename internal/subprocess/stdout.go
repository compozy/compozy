package subprocess

import (
	"errors"
	"io"
	"os"
	"sync"
)

type stdoutReader struct {
	file  *os.File
	close func() error
}

func newStdoutReader(file *os.File) *stdoutReader {
	return &stdoutReader{file: file, close: sync.OnceValue(file.Close)}
}

func (r *stdoutReader) Read(p []byte) (int, error) {
	n, err := r.file.Read(p)
	if err != nil {
		if closeErr := r.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return n, err
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
