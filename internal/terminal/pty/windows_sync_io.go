//go:build windows

package pty

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type windowsSynchronousOperation func([]byte) (int, error)

type windowsSyncRequest struct {
	input  []byte
	result chan windowsSyncResult
}

type windowsSyncResult struct {
	n   int
	err error
}

type windowsSyncIO struct {
	operation windowsSynchronousOperation
	name      string
	requests  chan windowsSyncRequest
	stopCh    chan struct{}
	cancelCh  chan struct{}
	readyCh   chan struct{}
	doneCh    chan struct{}

	cancelOnce sync.Once
	stopOnce   sync.Once
	mu         sync.Mutex
	thread     windows.Handle
	startErr   error
	closeErr   error
	cancelErr  error
	cancelled  bool
	activeDone chan struct{}
}

var cancelSynchronousIOProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")

const windowsSyncIOCancelRetryInterval = time.Millisecond

func newWindowsSyncIO(name string, operation windowsSynchronousOperation) (*windowsSyncIO, error) {
	if operation == nil {
		return nil, errors.New("terminal pty: synchronous operation is required")
	}
	if err := cancelSynchronousIOProc.Find(); err != nil {
		return nil, fmt.Errorf("terminal pty: resolve CancelSynchronousIo: %w", err)
	}
	worker := &windowsSyncIO{
		operation: operation,
		name:      name,
		requests:  make(chan windowsSyncRequest),
		stopCh:    make(chan struct{}),
		cancelCh:  make(chan struct{}),
		readyCh:   make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	go worker.run()
	<-worker.readyCh
	worker.mu.Lock()
	startErr := worker.startErr
	worker.mu.Unlock()
	if startErr != nil {
		<-worker.doneCh
		return nil, fmt.Errorf("terminal pty: start %s worker: %w", name, startErr)
	}
	return worker, nil
}

func (w *windowsSyncIO) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	thread, err := duplicateWindowsCurrentThreadHandle()
	w.mu.Lock()
	w.thread = thread
	w.startErr = err
	w.mu.Unlock()
	close(w.readyCh)
	if err != nil {
		close(w.doneCh)
		return
	}
	defer func() {
		closeErr := closeWindowsSyncThreadHandle(thread, w.name)
		w.mu.Lock()
		w.thread = 0
		w.closeErr = closeErr
		w.mu.Unlock()
		close(w.doneCh)
	}()

	for {
		select {
		case request := <-w.requests:
			w.execute(request)
		case <-w.stopCh:
			return
		}
	}
}

func (w *windowsSyncIO) execute(request windowsSyncRequest) {
	w.mu.Lock()
	if w.cancelled {
		w.mu.Unlock()
		request.result <- windowsSyncResult{err: errIOClosed}
		return
	}
	activeDone := make(chan struct{})
	w.activeDone = activeDone
	operation := w.operation
	w.mu.Unlock()

	written, err := operation(request.input)
	w.mu.Lock()
	w.activeDone = nil
	w.mu.Unlock()
	close(activeDone)
	request.result <- windowsSyncResult{n: written, err: err}
}

func (w *windowsSyncIO) do(input []byte) (int, error) {
	if w == nil {
		return 0, errIOClosed
	}
	request := windowsSyncRequest{input: input, result: make(chan windowsSyncResult, 1)}
	select {
	case w.requests <- request:
	case <-w.cancelCh:
		return 0, errIOClosed
	case <-w.doneCh:
		return 0, errIOClosed
	}
	result := <-request.result
	return result.n, result.err
}

func (w *windowsSyncIO) cancel() error {
	if w == nil {
		return nil
	}
	w.cancelOnce.Do(func() {
		close(w.cancelCh)
		w.mu.Lock()
		w.cancelled = true
		thread := w.thread
		activeDone := w.activeDone
		w.mu.Unlock()
		if activeDone == nil {
			return
		}
		w.cancelErr = cancelWindowsSynchronousIOUntilDone(thread, w.name, activeDone)
	})
	return w.cancelErr
}

func cancelWindowsSynchronousIOUntilDone(
	thread windows.Handle,
	name string,
	activeDone <-chan struct{},
) error {
	var firstErr error
	retry := time.NewTicker(windowsSyncIOCancelRetryInterval)
	defer retry.Stop()
	for {
		if err := cancelWindowsSynchronousIO(thread, name); err != nil && firstErr == nil {
			firstErr = err
		}
		select {
		case <-activeDone:
			return firstErr
		case <-retry.C:
		}
	}
}

func cancelWindowsSynchronousIO(thread windows.Handle, name string) error {
	if thread == 0 {
		return fmt.Errorf("terminal pty: cancel %s: thread handle is closed", name)
	}
	result, _, callErr := cancelSynchronousIOProc.Call(uintptr(thread))
	if result != 0 {
		return nil
	}
	if callErr == nil {
		callErr = windows.ERROR_GEN_FAILURE
	}
	if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
		return nil
	}
	return fmt.Errorf("terminal pty: cancel %s synchronously: %w", name, callErr)
}

func (w *windowsSyncIO) stop() error {
	if w == nil {
		return nil
	}
	w.stopOnce.Do(func() {
		close(w.stopCh)
		<-w.doneCh
	})
	return w.closeErr
}

func duplicateWindowsCurrentThreadHandle() (windows.Handle, error) {
	process := windows.CurrentProcess()
	var duplicate windows.Handle
	if err := windows.DuplicateHandle(
		process,
		windows.CurrentThread(),
		process,
		&duplicate,
		0,
		false,
		windows.DUPLICATE_SAME_ACCESS,
	); err != nil {
		return 0, fmt.Errorf("duplicate current thread handle: %w", err)
	}
	return duplicate, nil
}

func closeWindowsSyncThreadHandle(handle windows.Handle, name string) error {
	if handle == 0 {
		return nil
	}
	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("terminal pty: close %s worker thread handle: %w", name, err)
	}
	return nil
}
