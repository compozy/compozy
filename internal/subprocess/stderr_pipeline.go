package subprocess

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
)

const maxPendingStderrBytes = 1024 * 1024

const oversizedStderrRecord = "[stderr record exceeded 1048576 bytes; discarded]\n"

type stderrPipeline struct {
	mu                        sync.Mutex
	pending                   []byte
	capture                   io.Writer
	sink                      io.Writer
	transform                 func(string) string
	discardingOversizedRecord bool
}

func newStderrPipeline(capture, sink io.Writer, transform func(string) string) *stderrPipeline {
	if transform == nil {
		transform = func(value string) string { return value }
	}
	return &stderrPipeline{capture: capture, sink: sink, transform: transform}
}

func (w *stderrPipeline) Write(p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	written := len(p)
	var resultErr error
	for len(p) > 0 {
		if w.discardingOversizedRecord {
			newline := bytes.IndexByte(p, '\n')
			if newline < 0 {
				return written, resultErr
			}
			w.discardingOversizedRecord = false
			p = p[newline+1:]
			continue
		}

		remainingCapacity := maxPendingStderrBytes - len(w.pending)
		newline := bytes.IndexByte(p, '\n')
		if newline >= 0 && newline < remainingCapacity {
			end := newline + 1
			w.pending = append(w.pending, p[:end]...)
			if err := w.emitLocked(w.pending); err != nil {
				resultErr = errors.Join(resultErr, err)
			}
			w.pending = w.pending[:0]
			p = p[end:]
			continue
		}
		if len(p) < remainingCapacity {
			w.pending = append(w.pending, p...)
			break
		}

		w.pending = w.pending[:0]
		w.discardingOversizedRecord = true
		if err := w.emitLocked([]byte(oversizedStderrRecord)); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}
	return written, resultErr
}

func (w *stderrPipeline) Flush() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.discardingOversizedRecord = false
	if len(w.pending) == 0 {
		return nil
	}
	err := w.emitLocked(w.pending)
	w.pending = nil
	return err
}

func (w *stderrPipeline) emitLocked(raw []byte) error {
	redacted := []byte(w.transform(string(raw)))
	var resultErr error
	if w.capture != nil {
		if _, err := w.capture.Write(redacted); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("capture stderr: %w", err))
		}
	}
	if w.sink != nil {
		if _, err := w.sink.Write(redacted); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("forward stderr: %w", err))
		}
	}
	return resultErr
}
