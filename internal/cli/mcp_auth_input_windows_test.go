//go:build windows

package cli

import (
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func newInheritedMCPAuthInput(t *testing.T) *os.File {
	t.Helper()

	var readHandle windows.Handle
	var writeHandle windows.Handle
	if err := windows.CreatePipe(&readHandle, &writeHandle, nil, 0); err != nil {
		t.Fatalf("create inherited manual input pipe error = %v", err)
	}
	input := os.NewFile(uintptr(readHandle), "inherited-mcp-auth-input")
	writer := os.NewFile(uintptr(writeHandle), "inherited-mcp-auth-writer")
	if input == nil || writer == nil {
		if input != nil {
			if err := input.Close(); err != nil {
				t.Errorf("manual input close after file creation failure = %v", err)
			}
		} else if err := windows.CloseHandle(readHandle); err != nil {
			t.Errorf("manual input handle close after file creation failure = %v", err)
		}
		if writer != nil {
			if err := writer.Close(); err != nil {
				t.Errorf("manual input writer close after file creation failure = %v", err)
			}
		} else if err := windows.CloseHandle(writeHandle); err != nil {
			t.Errorf("manual input writer handle close after file creation failure = %v", err)
		}
		t.Fatal("os.NewFile() returned nil for inherited manual input")
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("inherited manual input close error = %v", err)
		}
	})
	t.Cleanup(func() {
		if err := writer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("manual input writer close error = %v", err)
		}
	})
	assertMCPAuthInputBlocking(t, input)
	return input
}

func assertMCPAuthInputBlocking(t *testing.T, file *os.File) {
	t.Helper()

	if err := file.SetReadDeadline(time.Time{}); !errors.Is(err, os.ErrNoDeadline) {
		t.Fatalf("inherited input SetReadDeadline() error = %v, want os.ErrNoDeadline", err)
	}
}
