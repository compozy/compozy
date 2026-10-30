//go:build !darwin && !linux

package procutil

import (
	"context"
	"errors"
)

func scanProcesses(context.Context, bool) ([]ScannedProcess, bool, error) {
	return []ScannedProcess{}, false, nil
}

func enrichProcesses(context.Context, []ScannedProcess) {}
func scanProcessIdentity(int) (ScannedProcess, error) {
	return ScannedProcess{}, errors.New("process scanning unsupported")
}
