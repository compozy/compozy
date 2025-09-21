package helpers

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindAvailablePort(t *testing.T) {
	t.Run("Should reuse configured port after short wait", func(t *testing.T) {
		originalTimeout := portReleaseTimeout
		originalInterval := portReleasePollInterval
		portReleaseTimeout = 100 * time.Millisecond
		portReleasePollInterval = 10 * time.Millisecond
		t.Cleanup(func() {
			portReleaseTimeout = originalTimeout
			portReleasePollInterval = originalInterval
		})
		listenCtx, listenCancel := context.WithCancel(context.Background())
		defer listenCancel()
		lc := net.ListenConfig{}
		listener, err := lc.Listen(listenCtx, "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().(*net.TCPAddr)
		host := "127.0.0.1"
		port := addr.Port
		ctx := logger.ContextWithLogger(context.Background(), logger.NewLogger(logger.TestConfig()))
		done := make(chan struct{})
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = listener.Close()
			close(done)
		}()
		start := time.Now()
		resolvedPort, resolveErr := FindAvailablePort(ctx, host, port)
		require.NoError(t, resolveErr)
		<-done
		assert.Equal(t, port, resolvedPort)
		assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)
	})

	t.Run("Should fall back to new port when configured port stays busy", func(t *testing.T) {
		originalTimeout := portReleaseTimeout
		originalInterval := portReleasePollInterval
		portReleaseTimeout = 80 * time.Millisecond
		portReleasePollInterval = 10 * time.Millisecond
		t.Cleanup(func() {
			portReleaseTimeout = originalTimeout
			portReleasePollInterval = originalInterval
		})
		listenCtx, listenCancel := context.WithCancel(context.Background())
		defer listenCancel()
		lc := net.ListenConfig{}
		listener, err := lc.Listen(listenCtx, "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().(*net.TCPAddr)
		host := "127.0.0.1"
		port := addr.Port
		ctx := logger.ContextWithLogger(context.Background(), logger.NewLogger(logger.TestConfig()))
		done := make(chan struct{})
		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = listener.Close()
			close(done)
		}()
		resolvedPort, resolveErr := FindAvailablePort(ctx, host, port)
		require.NoError(t, resolveErr)
		<-done
		assert.NotEqual(t, port, resolvedPort)
	})
}
