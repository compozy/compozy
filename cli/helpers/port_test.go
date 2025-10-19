package helpers

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnsurePortAvailable(t *testing.T) {
	t.Run("Should allow binding when port is available", func(t *testing.T) {
		listener, port := occupyPort(t)
		require.NoError(t, listener.Close())
		waitForPortRelease(t, port)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		require.NoError(t, EnsurePortAvailable(ctx, "127.0.0.1", port))
	})
	t.Run("Should return error when port is already bound", func(t *testing.T) {
		listener, port := occupyPort(t)
		defer listener.Close()
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		err := EnsurePortAvailable(ctx, "127.0.0.1", port)
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("%d", port))
	})
}

func occupyPort(t *testing.T) (net.Listener, int) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return listener, addr.Port
}

func TestFormatAddress(t *testing.T) {
	t.Run("Should bracket IPv6 host", func(t *testing.T) {
		require.Equal(t, "[::1]:5000", formatAddress("::1", 5000))
	})
	t.Run("Should leave IPv4 host unchanged", func(t *testing.T) {
		require.Equal(t, "127.0.0.1:5000", formatAddress("127.0.0.1", 5000))
	})
}

func waitForPortRelease(t *testing.T, port int) {
	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		return EnsurePortAvailable(ctx, "127.0.0.1", port) == nil
	}, 500*time.Millisecond, 25*time.Millisecond, "port %d did not become available in time", port)
}
