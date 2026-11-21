//go:build !darwin && !linux

package procutil

import (
	"context"
	"errors"
)

func scanProcesses(context.Context, bool) ([]ScannedProcess, bool, error) {
	return []ScannedProcess{}, false, nil
}

func scanProcessDescendants(ctx context.Context, _, _ []int) ([]ScannedProcess, bool, error) {
	return scanProcesses(ctx, false)
}

func enrichProcesses(context.Context, []ScannedProcess) {}
func scanProcessIdentity(int) (ScannedProcess, error) {
	return ScannedProcess{}, errors.New("process scanning unsupported")
}
