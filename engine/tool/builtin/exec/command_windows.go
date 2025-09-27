//go:build windows

package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func newCommand(ctx context.Context, path string, args []string) (*exec.Cmd, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("command path must be absolute on windows: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("command not accessible: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("command path references a directory: %s", path)
	}
	return exec.CommandContext(ctx, path, args...), nil
}
