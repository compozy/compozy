//go:build !darwin && !linux

package procutil

import (
	"context"
	"errors"
	"time"
)

type ProcessTermination struct {
	Outcome string
	Err     error
}

func TerminateScannedProcess(context.Context, ScannedProcess, time.Duration) ProcessTermination {
	return ProcessTermination{Outcome: "unproven", Err: errors.New("process scanning unsupported")}
}

func TerminateObservedProcess(ctx context.Context, p ScannedProcess, grace time.Duration) ProcessTermination {
	return TerminateScannedProcess(ctx, p, grace)
}
