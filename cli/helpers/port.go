package helpers

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// IsPortAvailable checks if a port is available for binding using the provided context
func IsPortAvailable(ctx context.Context, host string, port int) bool {
	addr := formatAddress(host, port)
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

// EnsurePortAvailable verifies that the requested port can be bound.
// Returns an error when the port is already in use to force the caller to resolve the conflict.
func EnsurePortAvailable(ctx context.Context, host string, port int) error {
	if IsPortAvailable(ctx, host, port) {
		return nil
	}
	return fmt.Errorf("port %d is not available on host %s", port, host)
}

func formatAddress(host string, port int) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return fmt.Sprintf("[%s]:%d", host, port)
	}
	return fmt.Sprintf("%s:%d", host, port)
}
