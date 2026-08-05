package acp

import (
	"context"
	"log/slog"
	"strings"

	identityprotocol "github.com/compozy/compozy/internal/agentidentity/protocol"
	"github.com/compozy/compozy/internal/sandbox"
)

func prepareManagedAgentTransport(
	ctx context.Context,
	opts StartOpts,
	launcher sandbox.Launcher,
	logger *slog.Logger,
) (StartOpts, *managedAgentTransport, error) {
	next := withoutManagedAgentTransport(opts)
	if _, ok := launcher.(managedAgentTransportLauncher); !ok {
		return next, nil, nil
	}
	return bindManagedAgentTransport(ctx, next, logger)
}

func bindManagedAgentTransport(
	ctx context.Context,
	opts StartOpts,
	logger *slog.Logger,
) (StartOpts, *managedAgentTransport, error) {
	next := withoutManagedAgentTransport(opts)

	sessionID := startEnvValue(next.TerminalEnv, identityprotocol.EnvSessionID)
	agentName := startEnvValue(next.TerminalEnv, identityprotocol.EnvAgent)
	transport, socketPath, err := startManagedAgentTransport(
		ctx,
		strings.TrimSpace(next.DaemonSocket),
		sessionID,
		agentName,
		logger,
	)
	if err != nil {
		return StartOpts{}, nil, err
	}
	if transport == nil {
		return next, nil, nil
	}
	next.Env = setEnvValue(next.Env, identityprotocol.EnvTransportSocket, socketPath)
	next.TerminalEnv = setEnvValue(next.TerminalEnv, identityprotocol.EnvTransportSocket, socketPath)
	return next, transport, nil
}

func withoutManagedAgentTransport(opts StartOpts) StartOpts {
	next := opts
	next.Env = setEnvValue(next.Env, identityprotocol.EnvTransportSocket, "")
	next.TerminalEnv = setEnvValue(next.TerminalEnv, identityprotocol.EnvTransportSocket, "")
	return next
}

func startEnvValue(env []string, name string) string {
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(name)) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
