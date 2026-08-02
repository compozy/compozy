//go:build !windows

package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func newInheritedMCPAuthInput(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("manual input writer close error = %v", err)
		}
	})

	duplicateFD, err := unix.Dup(int(input.Fd()))
	if err != nil {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("manual input close after duplicate failure = %v", closeErr)
		}
		t.Fatalf("duplicate inherited manual input error = %v", err)
	}
	if err := input.Close(); err != nil {
		if closeErr := unix.Close(duplicateFD); closeErr != nil {
			t.Errorf("duplicate close after manual input close failure = %v", closeErr)
		}
		t.Fatalf("manual input close error = %v", err)
	}

	inherited := os.NewFile(uintptr(duplicateFD), "inherited-mcp-auth-input")
	if inherited == nil {
		if closeErr := unix.Close(duplicateFD); closeErr != nil {
			t.Errorf("invalid duplicate close error = %v", closeErr)
		}
		t.Fatal("os.NewFile() returned nil for inherited manual input")
	}
	t.Cleanup(func() {
		if err := inherited.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("inherited manual input close error = %v", err)
		}
	})
	if err := inherited.SetReadDeadline(time.Time{}); !errors.Is(err, os.ErrNoDeadline) {
		t.Fatalf("inherited input SetReadDeadline() error = %v, want os.ErrNoDeadline", err)
	}
	return inherited, writer
}

func assertMCPAuthInputBlocking(t *testing.T, file *os.File, _ *os.File) {
	t.Helper()

	rawConnection, err := file.SyscallConn()
	if err != nil {
		t.Fatalf("manual input SyscallConn() error = %v", err)
	}
	flags := 0
	var operationErr error
	if err := rawConnection.Control(func(fd uintptr) {
		flags, operationErr = unix.FcntlInt(fd, unix.F_GETFL, 0)
	}); err != nil {
		t.Fatalf("manual input descriptor control error = %v", err)
	}
	if operationErr != nil {
		t.Fatalf("manual input flags error = %v", operationErr)
	}
	if flags&unix.O_NONBLOCK != 0 {
		t.Fatal("borrowed manual input remained nonblocking after cancellable read")
	}
}

func TestPrepareCancellableMCPAuthInput(t *testing.T) {
	t.Parallel()

	t.Run("Should interrupt a pipe read with a deadline", func(t *testing.T) {
		t.Parallel()

		input, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := input.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				t.Errorf("input close error = %v", closeErr)
			}
		})
		t.Cleanup(func() {
			if closeErr := writer.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				t.Errorf("writer close error = %v", closeErr)
			}
		})

		ctx, cancel := context.WithCancel(t.Context())
		readStarted := make(chan struct{})
		readResult := make(chan error, 1)
		result := make(chan error, 1)
		go func() {
			_, readErr := readManualMCPAuthFileContext(ctx, input, func(reader io.Reader) (string, error) {
				close(readStarted)
				buffer := make([]byte, 1)
				_, inputErr := reader.Read(buffer)
				readResult <- inputErr
				return string(buffer), inputErr
			})
			result <- readErr
		}()
		<-readStarted
		cancel()

		if readErr := <-readResult; !errors.Is(readErr, os.ErrDeadlineExceeded) {
			t.Fatalf("cancellable pipe read error = %v, want os.ErrDeadlineExceeded", readErr)
		}
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("readManualMCPAuthFileContext() error = %v, want context cancellation", err)
		}
	})
}
