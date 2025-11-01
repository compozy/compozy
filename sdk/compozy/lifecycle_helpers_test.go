package compozy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineRecordServerError(t *testing.T) {
	engine := &Engine{}
	engine.recordServerError(nil)
	assert.Nil(t, engine.serverFailure())
	expected := errors.New("server exploded")
	engine.recordServerError(expected)
	assert.Equal(t, expected, engine.serverFailure())
}

func TestEngineStartRequiresContext(t *testing.T) {
	engine := &Engine{}
	var nilCtx context.Context
	err := engine.Start(nilCtx)
	assert.Error(t, err)
}

func TestEngineStartUnsupportedMode(t *testing.T) {
	ctx := lifecycleTestContext(t)
	engine := &Engine{mode: Mode("legacy")}
	err := engine.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine mode")
	assert.Equal(t, err, engine.startErr)
}

func TestEngineStopReturnsCachedError(t *testing.T) {
	engine := &Engine{stopErr: errors.New("cached stop")}
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	err := engine.Stop(ctx)
	require.Error(t, err)
	assert.Equal(t, "cached stop", err.Error())
}

func TestEngineCleanupModeResourcesAggregatesErrors(t *testing.T) {
	engine := &Engine{}
	engine.modeCleanups = []modeCleanup{
		func(context.Context) error { return nil },
		func(context.Context) error { return errors.New("first failure") },
		func(context.Context) error { return errors.New("second failure") },
	}
	err := engine.cleanupModeResources(logger.ContextWithLogger(t.Context(), logger.NewForTests()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "first failure")
	assert.Contains(t, err.Error(), "second failure")
	assert.NoError(t, engine.cleanupModeResources(t.Context()))
}

func TestSanitizeHostForClientVariants(t *testing.T) {
	assert.Equal(t, loopbackHostname, sanitizeHostForClient(""))
	assert.Equal(t, loopbackHostname, sanitizeHostForClient("0.0.0.0"))
	assert.Equal(t, loopbackHostname, sanitizeHostForClient("::"))
	assert.Equal(t, "example.com", sanitizeHostForClient("example.com"))
}

func TestEngineCleanupStoreLogsOnCloseFailure(t *testing.T) {
	store := newResourceStoreStub()
	store.closeErr = errors.New("close failure")
	engine := &Engine{}
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	engine.cleanupStore(ctx, store)
	engine.cleanupStore(ctx, nil)
}

func TestEngineListenAllocatesPort(t *testing.T) {
	engine := &Engine{}
	listener, port, err := engine.listen(t.Context(), "", 0)
	require.NoError(t, err)
	require.NotZero(t, port)
	require.NotNil(t, listener)
	require.NoError(t, listener.Close())
}

func TestEngineListenFailsForOccupiedPort(t *testing.T) {
	listenCfg := net.ListenConfig{}
	ln, err := listenCfg.Listen(context.WithoutCancel(t.Context()), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().(*net.TCPAddr)
	engine := &Engine{}
	_, _, listenErr := engine.listen(t.Context(), "127.0.0.1", addr.Port)
	require.Error(t, listenErr)
	assert.Contains(t, listenErr.Error(), "listen on")
	require.NoError(t, ln.Close())
}

func TestEngineNewHTTPServerAppliesTimeouts(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg := appconfig.FromContext(lifecycleTestContext(t))
	require.NotNil(t, cfg)
	cfg.Server.Timeouts.HTTPRead = time.Second
	cfg.Server.Timeouts.HTTPWrite = 2 * time.Second
	cfg.Server.Timeouts.HTTPIdle = 3 * time.Second
	cfg.Server.Timeouts.HTTPReadHeader = 500 * time.Millisecond
	engine := &Engine{}
	server := engine.newHTTPServer(ctx, chi.NewRouter(), cfg, "127.0.0.1:0")
	assert.Equal(t, time.Second, server.ReadTimeout)
	assert.Equal(t, 2*time.Second, server.WriteTimeout)
	assert.Equal(t, 3*time.Second, server.IdleTimeout)
	assert.Equal(t, 500*time.Millisecond, server.ReadHeaderTimeout)
}

func TestEngineNewClientSanitizesHost(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	engine := &Engine{}
	client, baseURL, err := engine.newClient(ctx, "0.0.0.0", 8080)
	require.NoError(t, err)
	assert.Contains(t, baseURL, "127.0.0.1:8080")
	require.NotNil(t, client)
}

type failingListener struct{}

func (f *failingListener) Accept() (net.Conn, error) {
	return nil, listenerError{}
}

func (f *failingListener) Close() error {
	return nil
}

func (f *failingListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4zero, Port: 0}
}

type listenerError struct{}

func (listenerError) Error() string {
	return "accept failure"
}

func (listenerError) Timeout() bool {
	return false
}

func (listenerError) Temporary() bool {
	return false
}

func TestEngineLaunchServerRecordsFailures(t *testing.T) {
	engine := &Engine{}
	log := logger.NewForTests()
	ctx := logger.ContextWithLogger(t.Context(), log)
	server := &http.Server{}
	engine.launchServer(ctx, server, &failingListener{})
	engine.serverWG.Wait()
	err := engine.serverFailure()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http server failure")
}
