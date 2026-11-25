package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func loadRuntimeContext(deps commandDeps) (*runtimeContext, error) {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if deps.commandContext != nil {
		if commandCtx := deps.commandContext(); commandCtx != nil {
			ctx = commandCtx
		}
	}
	cfg, err := deps.loadConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Daemon.Socket) == "" {
		cfg.Daemon.Socket = homePaths.DaemonSocket
	}
	if deps.computerTarget != nil {
		if selection, selected := deps.computerTarget(); selected {
			target, err := bridgeClientTarget(ctx, deps, cfg.Daemon.Socket, selection)
			if err != nil {
				return nil, err
			}
			return &runtimeContext{HomePaths: homePaths, Config: cfg, Target: target}, nil
		}
	}
	return &runtimeContext{
		HomePaths: homePaths,
		Config:    cfg,
		Target:    LocalClientTarget(cfg.Daemon.Socket),
	}, nil
}

func clientFromDeps(deps commandDeps) (DaemonClient, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return nil, err
	}

	client, err := deps.newClient(runtime.Target)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// resolveLocalClientTarget keeps runtime lifecycle actions on the configured local socket.
func resolveLocalClientTarget(deps commandDeps, homePaths compozyconfig.HomePaths) (ClientTarget, error) {
	cfg, err := deps.loadConfig()
	if err != nil {
		return ClientTarget{}, err
	}
	return LocalClientTarget(cmp.Or(strings.TrimSpace(cfg.Daemon.Socket), homePaths.DaemonSocket)), nil
}

func currentWorkingDirectory(deps commandDeps) (string, error) {
	if deps.getwd == nil {
		return "", errors.New("cli: getwd dependency is required")
	}

	wd, err := deps.getwd()
	if err != nil {
		return "", fmt.Errorf("cli: resolve current working directory: %w", err)
	}
	wd = strings.TrimSpace(wd)
	if wd == "" {
		return "", errors.New("cli: current working directory is empty")
	}
	return wd, nil
}
