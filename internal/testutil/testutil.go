// Package testutil provides shared test helpers for internal packages.
package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const defaultTimeout = 45 * time.Second

var (
	tcpPortCounter       atomic.Uint32
	tcpPortReservationMu sync.Mutex
	tcpPortReservations  = make(map[int]struct{})
)

// Context returns a context canceled during test cleanup.
func Context(t testing.TB) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	t.Cleanup(cancel)
	return ctx
}

// EqualStringSlices reports whether two string slices have equal contents.
func EqualStringSlices(left, right []string) bool {
	return slices.Equal(left, right)
}

// FreeTCPPort returns an available localhost TCP port chosen from a
// per-process pseudo-random walk through a high, non-default port range. The
// listener is closed before returning, so callers still need to bind quickly,
// but this avoids repeated reuse of the same ephemeral port under parallel
// package execution.
func FreeTCPPort(t testing.TB) int {
	t.Helper()

	const (
		minPort = 20000
		// Keep test allocations below the default Linux and macOS ephemeral ranges.
		portSpan    = 10000
		maxAttempts = portSpan
	)

	pid := os.Getpid()
	if pid < 0 {
		t.Fatalf("os.Getpid() = %d, want non-negative pid", pid)
	}

	listenerContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	listenConfig := net.ListenConfig{}
	seed := int64(pid)*131 + int64(tcpPortCounter.Add(1))
	start := int(seed % int64(portSpan))
	for attempt := range maxAttempts {
		port := minPort + ((start + attempt) % portSpan)
		ln, err := listenConfig.Listen(listenerContext, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		if !reserveTCPPort(port) {
			if err := ln.Close(); err != nil {
				t.Fatalf("net.Listener.Close(%d) error = %v", port, err)
			}
			continue
		}
		if err := ln.Close(); err != nil {
			releaseTCPPort(port)
			t.Fatalf("net.Listener.Close(%d) error = %v", port, err)
		}
		t.Cleanup(func() {
			releaseTCPPort(port)
		})
		return port
	}

	t.Fatal("FreeTCPPort() exhausted candidate range")
	return 0
}

func reserveTCPPort(port int) bool {
	tcpPortReservationMu.Lock()
	defer tcpPortReservationMu.Unlock()

	if _, ok := tcpPortReservations[port]; ok {
		return false
	}
	tcpPortReservations[port] = struct{}{}
	return true
}

func releaseTCPPort(port int) {
	tcpPortReservationMu.Lock()
	defer tcpPortReservationMu.Unlock()

	delete(tcpPortReservations, port)
}
