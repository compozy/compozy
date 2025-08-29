package helpers

import (
	"context"
	"fmt"
	"net"
)

const (
	maxPortScanAttempts = 100
)

// IsPortAvailable checks if a port is available for binding
func IsPortAvailable(host string, port int) bool {
	// Try to listen on the port with a short timeout
	addr := fmt.Sprintf("%s:%d", host, port)
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()
	return true
}

// FindAvailablePort finds the next available port starting from the given port
// It uses an exponential backoff strategy to efficiently find available ports
func FindAvailablePort(host string, startPort int) (int, error) {
	// First, try the requested port
	if IsPortAvailable(host, startPort) {
		return startPort, nil
	}

	// Common alternative ports for development servers
	commonPorts := []int{5000, 5001, 5002, 5003, 5004, 5005, 5006, 5007, 5008, 5009, 5010}
	for _, port := range commonPorts {
		if port != startPort && IsPortAvailable(host, port) {
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
		if portUp <= 65535 && !triedPorts[portUp] && IsPortAvailable(host, portUp) {
			return portUp, nil
		}

		// Check downward direction (but stay above privileged ports)
		if portDown >= 1024 && !triedPorts[portDown] && IsPortAvailable(host, portDown) {
			return portDown, nil
		}
	}

	return 0, fmt.Errorf("no available port found near %d after checking %d ports", startPort, maxPortScanAttempts)
}
