package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"

	compozylogger "github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/procutil"
)

func spawnDetachedDaemonProcess(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	executable func() (string, error),
) (daemonProcess, error) {
	if ctx == nil {
		return nil, errors.New("cli: daemon process context is required")
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		return nil, err
	}

	binary, err := executable()
	if err != nil {
		return nil, fmt.Errorf("cli: resolve executable: %w", err)
	}

	child, err := procutil.SpawnDetachedLoggedProcess(ctx, procutil.DetachedLaunchRequest{
		Binary:  binary,
		Args:    []string{daemonDaemonKey, daemonStartKey, "--foreground", "--" + internalChildFlagName},
		Sandbox: compozylogger.WithMirrorToStderrEnv(os.Environ(), false),
		LogPath: daemonStdioLogPath(homePaths),
	})
	if err != nil {
		return nil, fmt.Errorf("cli: spawn detached daemon: %w", err)
	}

	return child, nil
}

// A lifetime stdio handle prevents structured log rotation on Windows.
func daemonStdioLogPath(homePaths compozyconfig.HomePaths) string {
	return filepath.Join(filepath.Dir(homePaths.LogFile), "daemon-stdio.log")
}
