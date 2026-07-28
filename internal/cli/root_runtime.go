package cli

import (
	"errors"
	"fmt"

	"strings"
)

func loadRuntimeContext(deps commandDeps) (*runtimeContext, error) {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return nil, err
	}
	cfg, err := deps.loadConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Daemon.Socket) == "" {
		cfg.Daemon.Socket = homePaths.DaemonSocket
	}
	return &runtimeContext{
		HomePaths: homePaths,
		Config:    cfg,
	}, nil
}

func clientFromDeps(deps commandDeps) (DaemonClient, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return nil, err
	}

	socketPath := strings.TrimSpace(runtime.Config.Daemon.Socket)
	if socketPath == "" {
		socketPath = runtime.HomePaths.DaemonSocket
	}
	if socketPath == "" {
		return nil, errors.New("cli: daemon socket path is required")
	}

	client, err := deps.newClient(socketPath)
	if err != nil {
		return nil, err
	}
	return client, nil
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
