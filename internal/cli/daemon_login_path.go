package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

const (
	desktopLoginShellPathTimeout        = 5 * time.Second
	desktopLoginShellPathMaxOutputBytes = 64 * 1024
)

type desktopRuntimePathDependencies struct {
	executable     func() (string, error)
	ownedByDesktop func(compozyconfig.HomePaths, string) bool
	environment    func() []string
	resolve        func(context.Context, []string) (string, error)
	setenv         func(string, string) error
}

func refreshDesktopOwnedRuntimePathBeforeStart(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
) {
	pathDeps := desktopRuntimePathDependencies{
		executable:     deps.executable,
		ownedByDesktop: compozyupdate.RuntimeOwnedByDesktopApp,
		environment:    os.Environ,
		resolve: func(resolveCtx context.Context, environment []string) (string, error) {
			return procutil.ResolveLoginShellPath(resolveCtx, environment, procutil.LoginShellPathOptions{
				Timeout:        desktopLoginShellPathTimeout,
				MaxOutputBytes: desktopLoginShellPathMaxOutputBytes,
			})
		},
		setenv: os.Setenv,
	}
	if err := refreshDesktopOwnedRuntimePath(ctx, homePaths, pathDeps); err != nil {
		slog.WarnContext(ctx, "desktop runtime kept inherited PATH", "error", err)
	}
}

func refreshDesktopOwnedRuntimePath(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	deps desktopRuntimePathDependencies,
) error {
	executablePath, err := deps.executable()
	if err != nil {
		return fmt.Errorf("cli: resolve runtime executable for desktop PATH: %w", err)
	}
	if !deps.ownedByDesktop(homePaths, executablePath) {
		return nil
	}
	path, err := deps.resolve(ctx, deps.environment())
	if err != nil {
		return fmt.Errorf("cli: resolve desktop runtime login shell PATH: %w", err)
	}
	if err := deps.setenv("PATH", path); err != nil {
		return fmt.Errorf("cli: install desktop runtime login shell PATH: %w", err)
	}
	return nil
}
