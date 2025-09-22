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
// Strategy: try requested port (with optional wait), then common alternatives, then incremental scan.
func FindAvailablePort(ctx context.Context, host string, startPort int) (int, error) {
	cfg := config.FromContext(ctx)
	if IsPortAvailable(ctx, host, startPort) {
		return startPort, nil
	}
	timeout, poll := resolvePortWaitSettings(cfg)
	if waitForConfiguredPort(ctx, host, startPort, timeout, poll) {
		return startPort, nil
	}
	log := logger.FromContext(ctx)
	log.Warn(
		"Configured port still unavailable after wait, scanning for alternative",
		"port",
		startPort,
		"wait",
		timeout,
	)
	commonPorts := []int{5000, 5001, 5002, 5003, 5004, 5005, 5006, 5007, 5008, 5009, 5010}
	triedPorts := make(map[int]bool, len(commonPorts)+1)
	triedPorts[startPort] = true
	if port, ok := tryCommonPorts(ctx, host, startPort, commonPorts, triedPorts); ok {
		return port, nil
	}
	if port, ok := scanNearbyPorts(ctx, host, startPort, triedPorts); ok {
		return port, nil
	}
	return 0, fmt.Errorf("no available port found near %d after checking %d ports", startPort, maxPortScanAttempts)
}

func resolvePortWaitSettings(cfg *config.Config) (time.Duration, time.Duration) {
	timeout := config.DefaultPortReleaseTimeout
	poll := config.DefaultPortReleasePollInterval
	if cfg == nil {
		return timeout, poll
	}
	if cfg.CLI.PortReleaseTimeout > 0 {
		timeout = cfg.CLI.PortReleaseTimeout
	}
	if cfg.CLI.PortReleasePollInterval > 0 {
		poll = cfg.CLI.PortReleasePollInterval
	}
	return timeout, poll
}

func tryCommonPorts(ctx context.Context, host string, startPort int, ports []int, tried map[int]bool) (int, bool) {
	for _, port := range ports {
		tried[port] = true
		if port == startPort {
			continue
		}
		if IsPortAvailable(ctx, host, port) {
			return port, true
		}
	}
	return 0, false
}

func scanNearbyPorts(ctx context.Context, host string, startPort int, tried map[int]bool) (int, bool) {
	for i := 1; i < maxPortScanAttempts; i++ {
		portUp := startPort + i
		if portUp <= 65535 && !tried[portUp] && IsPortAvailable(ctx, host, portUp) {
			return portUp, true
		}
		tried[portUp] = true
		portDown := startPort - i
		if portDown >= 1024 && !tried[portDown] && IsPortAvailable(ctx, host, portDown) {
			return portDown, true
		}
		tried[portDown] = true
	}
	return 0, false
}

func waitForConfiguredPort(ctx context.Context, host string, port int, timeout, pollInterval time.Duration) bool {
	log := logger.FromContext(ctx)
	if timeout <= 0 {
		timeout = config.DefaultPortReleaseTimeout
	}
	if pollInterval <= 0 {
		pollInterval = config.DefaultPortReleasePollInterval
	}
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
