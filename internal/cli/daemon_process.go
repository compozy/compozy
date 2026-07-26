package cli

import (
	"context"

	"fmt"
	"os"

	aghconfig "github.com/compozy/agh/internal/config"

	aghlogger "github.com/compozy/agh/internal/logger"
	"github.com/compozy/agh/internal/procutil"
)

func spawnDetachedDaemonProcess(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	executable func() (string, error),
) (daemonProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		return nil, err
	}

	binary, err := executable()
	if err != nil {
		return nil, fmt.Errorf("cli: resolve executable: %w", err)
	}

	child, err := procutil.SpawnDetachedLoggedProcess(ctx, procutil.DetachedLaunchRequest{
		Binary:  binary,
		Args:    []string{daemonDaemonKey, daemonStartKey, "--foreground", "--" + internalChildFlagName},
		Sandbox: aghlogger.WithMirrorToStderrEnv(os.Environ(), false),
		LogPath: homePaths.LogFile,
	})
	if err != nil {
		return nil, fmt.Errorf("cli: spawn detached daemon: %w", err)
	}

	return child, nil
}
