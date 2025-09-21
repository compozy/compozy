package helpers

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	maxPortScanAttempts = 100
)

// IsPortAvailable checks if a port is available for binding using the provided context
func IsPortAvailable(ctx context.Context, host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

// FindAvailablePort finds the next available port starting from the given port
// It uses an exponential backoff strategy to efficiently find available ports
func FindAvailablePort(ctx context.Context, host string, startPort int) (int, error) {
	cfg := config.FromContext(ctx)

	if IsPortAvailable(ctx, host, startPort) {
		return startPort, nil
	}
	if waitForConfiguredPort(ctx, host, startPort, cfg.CLI.PortReleaseTimeout, cfg.CLI.PortReleasePollInterval) {
		return startPort, nil
	}
	log := logger.FromContext(ctx)
	log.Warn(
		"Configured port still unavailable after wait, scanning for alternative",
		"port",
		startPort,
		"wait",
		cfg.CLI.PortReleaseTimeout,
	)

	// Common alternative ports for development servers
	commonPorts := []int{5000, 5001, 5002, 5003, 5004, 5005, 5006, 5007, 5008, 5009, 5010}
	for _, port := range commonPorts {
		if port != startPort && IsPortAvailable(ctx, host, port) {
			return port, nil
		}
	}

	// If common ports are taken, scan incrementally from the start port
	// but skip already tried common ports
	triedPorts := make(map[int]bool)
	for _, p := range commonPorts {
		triedPorts[p] = true
	}
	triedPorts[startPort] = true

	for i := 1; i < maxPortScanAttempts; i++ {
		// Try ports in both directions from the start port
		portUp := startPort + i
		portDown := startPort - i

		// Check upward direction
		if portUp <= 65535 && !triedPorts[portUp] && IsPortAvailable(ctx, host, portUp) {
			return portUp, nil
		}

		// Check downward direction (but stay above privileged ports)
		if portDown >= 1024 && !triedPorts[portDown] && IsPortAvailable(ctx, host, portDown) {
			return portDown, nil
		}
	}

	return 0, fmt.Errorf("no available port found near %d after checking %d ports", startPort, maxPortScanAttempts)
}

func waitForConfiguredPort(ctx context.Context, host string, port int, timeout, pollInterval time.Duration) bool {
	log := logger.FromContext(ctx)
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	notified := false
	for {
		if IsPortAvailable(ctx, host, port) {
			if notified {
				log.Info("Configured port became available", "port", port)
			}
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		if !notified {
			log.Info("Waiting for configured port to become available", "port", port, "timeout", timeout)
			notified = true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
