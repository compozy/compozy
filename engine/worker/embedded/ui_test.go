package embedded

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUIServer(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		BindIP:       "127.0.0.1",
		FrontendPort: 7600,
		Namespace:    "default",
		EnableUI:     true,
		UIPort:       randomUIPort(t),
	}

	ui := newUIServer(cfg)
	require.NotNil(t, ui)
	assert.Equal(t, cfg, ui.config)
	assert.Equal(t, cfg.EnableUI, ui.config.EnableUI)
}

func TestUIServerLifecycle(t *testing.T) {
	t.Parallel()

	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg := &Config{
		BindIP:       "127.0.0.1",
		FrontendPort: 7700,
		Namespace:    "default",
		EnableUI:     true,
		UIPort:       randomUIPort(t),
	}

	ui := newUIServer(cfg)
	require.NotNil(t, ui)

	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, ui.Start(startCtx))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ui.address+"/health", http.NoBody)
	require.NoError(t, err)
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stopCancel()
	require.NoError(t, ui.Stop(stopCtx))

	dialer := &net.Dialer{Timeout: 200 * time.Millisecond}
	_, err = dialer.DialContext(ctx, "tcp", ui.address)
	require.Error(t, err)
}

func TestUIServerDisabled(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		EnableUI: false,
		UIPort:   0,
	}

	assert.Nil(t, newUIServer(cfg))
}

func TestUIServerPortConflict(t *testing.T) {
	t.Parallel()

	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	port := randomUIPort(t)
	listener := reserveUIPort(t, port)
	defer func() { require.NoError(t, listener.Close()) }()

	cfg := &Config{
		BindIP:       "127.0.0.1",
		FrontendPort: 7800,
		Namespace:    "default",
		EnableUI:     true,
		UIPort:       port,
	}

	ui := newUIServer(cfg)
	require.NotNil(t, ui)

	err := ui.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(port))
}

func randomUIPort(t *testing.T) int {
	t.Helper()

	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)
	require.NoError(t, listener.Close())
	return addr.Port
}

func reserveUIPort(t *testing.T, port int) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := lc.Listen(t.Context(), "tcp", addr)
	require.NoError(t, err)
	return listener
}
