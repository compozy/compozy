//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pty

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// unixPTYReader normalizes the EIO returned by Unix PTY masters after the
// slave closes into the stream EOF expected by terminal consumers.
type unixPTYReader struct {
	*os.File
}

func (r *unixPTYReader) Read(buffer []byte) (int, error) {
	read, err := r.File.Read(buffer)
	if errors.Is(err, syscall.EIO) {
		return read, io.EOF
	}
	return read, err
}
