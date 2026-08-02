//go:build windows

package cli

import (
	"errors"
	"io"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func newInheritedMCPAuthInput(t *testing.T) (*os.File, *os.File) {
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
	assertMCPAuthInputBlocking(t, input, writer)
	return input, writer
}

func assertMCPAuthInputBlocking(t *testing.T, file *os.File, writer *os.File) {
	t.Helper()

	const expected = "r"
	if _, err := writer.WriteString(expected); err != nil {
		t.Fatalf("write borrowed manual input error = %v", err)
	}
	buffer := make([]byte, len(expected))
	if _, err := io.ReadFull(file, buffer); err != nil {
		t.Fatalf("read borrowed manual input error = %v", err)
	}
	if got := string(buffer); got != expected {
		t.Fatalf("borrowed manual input = %q, want %q", got, expected)
	}
}
