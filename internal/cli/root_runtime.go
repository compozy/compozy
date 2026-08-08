package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func loadRuntimeContext(deps commandDeps) (runtime *runtimeContext, returnErr error) {
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
	lock, err := acquireGatewayProfileRecoveryLock(ctx, homePaths, deps)
	if err != nil {
		return nil, err
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			runtime = nil
			returnErr = errors.Join(returnErr, fmt.Errorf("cli: release gateway profile lock: %w", releaseErr))
		}
	}()
	cfg, err := deps.loadConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Daemon.Socket) == "" {
		cfg.Daemon.Socket = homePaths.DaemonSocket
	}
	if active := strings.TrimSpace(cfg.Gateway.ActiveConnection); active != "" {
		if err := ensureGatewayProfileTransactionReady(homePaths.GatewayCredentialsDir, active); err != nil {
			return nil, err
		}
	}
	target, err := resolveConfiguredClientTarget(homePaths, &cfg, deps.readGatewayCredential)
	if err != nil {
		return nil, err
	}
	return &runtimeContext{
		HomePaths: homePaths,
		Config:    cfg,
		Target:    target,
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
	if deps.reportClientTarget != nil {
		if err := deps.reportClientTarget(runtime.Target); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func resolveConfiguredClientTarget(
	homePaths compozyconfig.HomePaths,
	cfg *compozyconfig.Config,
	readCredential func(string, string) (string, error),
) (ClientTarget, error) {
	if cfg == nil {
		return ClientTarget{}, errors.New("cli: configuration is required")
	}
	active := strings.TrimSpace(cfg.Gateway.ActiveConnection)
	if active == "" {
		socketPath := strings.TrimSpace(cfg.Daemon.Socket)
		if socketPath == "" {
			socketPath = homePaths.DaemonSocket
		}
		if socketPath == "" {
			return ClientTarget{}, errors.New("cli: daemon socket path is required")
		}
		return LocalClientTarget(socketPath), nil
	}

	for index := range cfg.Gateway.Connections {
		connection := cfg.Gateway.Connections[index]
		if connection.Name != active {
			continue
		}
		if connection.Scheme == clientSSHProtocol {
			return ClientTarget{}, fmt.Errorf(
				"cli: SSH profile %q requires `compozy connect ssh %s`",
				active,
				connection.Host,
			)
		}
		if readCredential == nil {
			return ClientTarget{}, errors.New("cli: gateway credential reader is required")
		}
		credential, err := readCredential(homePaths.GatewayCredentialsDir, active)
		if err != nil {
			return ClientTarget{}, fmt.Errorf("cli: load credential for profile %q: %w", active, err)
		}
		return GatewayClientTarget(active, connection.Host, connection.Port, credential)
	}
	return ClientTarget{}, fmt.Errorf("cli: active gateway profile %q was not found", active)
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
