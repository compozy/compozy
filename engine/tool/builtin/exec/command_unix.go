//go:build !windows

package exec

import (
	"context"
	"os/exec"

	"golang.org/x/sys/execabs"
)

func newCommand(ctx context.Context, path string, args []string) (*exec.Cmd, error) {
	return execabs.CommandContext(ctx, path, args...), nil
}
