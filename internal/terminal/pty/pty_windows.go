//go:build windows

package pty

import (
	"context"
	"os"
)

func startInteractive(context.Context, ProcSpec) (Proc, error) {
	return nil, ErrInteractiveUnavailable
}

func processSignal(*os.ProcessState) string { return "" }
