//go:build windows

package acp

import (
	"context"
	"log/slog"
)

type managedAgentTransport struct {
	socketDir string
}

func startManagedAgentTransport(
	context.Context,
	string,
	string,
	string,
	*slog.Logger,
) (*managedAgentTransport, string, error) {
	return nil, "", nil
}

func closeManagedAgentTransport(*managedAgentTransport) error {
	return nil
}
