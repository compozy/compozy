package compozy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	engineproject "github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/inline"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var temporalPortCounter int32 = 20000

type failingWatchStore struct {
	*resources.MemoryResourceStore
}

func (f *failingWatchStore) Watch(
	_ context.Context,
	_ string,
	_ resources.ResourceType,
) (<-chan resources.Event, error) {
	return nil, errors.New("watch failed")
}

func TestEngineLifecycle(t *testing.T) {
	t.Run("Should start and stop standalone engine with memory store", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		engine, err := New(
			ctx,
			WithWorkflow(&engineworkflow.Config{ID: "workflow-start"}),
			WithHost(loopbackHostname),
			WithPort(0),
		)
		require.NoError(t, err)
		require.NotNil(t, engine)

		err = engine.Start(ctx)
		require.NoError(t, err)
		assert.True(t, engine.IsStarted())

		store := engine.ResourceStore()
		require.IsType(t, &resources.MemoryResourceStore{}, store)
		assert.NotNil(t, engine.Server())
		assert.NotNil(t, engine.Router())
		assert.NotNil(t, engine.Config())
		assert.Equal(t, ModeStandalone, engine.Mode())

		err = engine.Start(ctx)
		require.ErrorIs(t, err, ErrAlreadyStarted)

		require.NoError(t, engine.Stop(ctx))
		assert.False(t, engine.IsStarted())
		assert.Nil(t, engine.Server())
		assert.Nil(t, engine.Router())

		require.NoError(t, engine.Stop(ctx))
		engine.Wait()

		memStore := store.(*resources.MemoryResourceStore)
		_, putErr := memStore.Put(ctx, resources.ResourceKey{
			Project: "test",
			Type:    resources.ResourceWorkflow,
			ID:      "after-stop",
		}, &engineworkflow.Config{ID: "after-stop"})
		require.Error(t, putErr)
		assert.ErrorContains(t, putErr, "store is closed")
	})

	t.Run("Should fail to start distributed mode without external configuration", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		cfg := appconfig.FromContext(ctx)
		require.NotNil(t, cfg)
		cfg.Mode = string(ModeDistributed)
		cfg.Redis.URL = ""
		cfg.Redis.Host = ""
		cfg.Redis.Port = ""
		engine, err := New(ctx, WithMode(ModeDistributed), WithWorkflow(&engineworkflow.Config{ID: "distributed"}))
		require.NoError(t, err)
		err = engine.Start(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "redis")
		assert.False(t, engine.IsStarted())
	})

	t.Run("Should start inline manager when project and tools provided", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		engine, err := New(
			ctx,
			WithProject(&engineproject.Config{Name: "inline-project"}),
			WithTool(&enginetool.Config{
				ID:   "inline-tool",
				Code: "export default () => 'ok';",
			}),
			WithWorkflow(&engineworkflow.Config{ID: "wf-inline"}),
			WithHost(loopbackHostname),
			WithPort(0),
		)
		require.NoError(t, err)
		require.NotNil(t, engine)
		require.NoError(t, engine.Start(ctx))
		require.NotNil(t, engine.inlineManager)
		require.NoError(t, engine.Stop(ctx))
		engine.Wait()
	})

	t.Run("Should fail to start with invalid host", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		engine, err := New(
			ctx,
			WithWorkflow(&engineworkflow.Config{ID: "invalid-host"}),
			WithHost("256.0.0.1"),
			WithPort(0),
		)
		require.NoError(t, err)
		err = engine.Start(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "listen")
	})
}

func TestEngineInitInlineManager(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg := appconfig.Default()
	cfg.CLI.CWD = t.TempDir()
	store := resources.NewMemoryResourceStore()
	defer func() {
		require.NoError(t, store.Close())
	}()
	toolCfg := &enginetool.Config{
		ID:   "inline-tool",
		Code: "export default () => 'inline';",
	}
	_, err := store.Put(ctx, resources.ResourceKey{
		Project: "inline-project",
		Type:    resources.ResourceTool,
		ID:      "inline-tool",
	}, toolCfg)
	require.NoError(t, err)
	engine := &Engine{
		ctx:     ctx,
		project: &engineproject.Config{Name: "inline-project"},
	}
	manager, err := engine.initInlineManager(ctx, cfg, store)
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer func() {
		require.NoError(t, manager.Close(ctx))
	}()
	entrypointPath := manager.EntrypointPath()
	require.NotEmpty(t, entrypointPath)
	require.Equal(t, entrypointPath, cfg.Runtime.EntrypointPath)
	data, readErr := os.ReadFile(entrypointPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "inline-tool")
}

func TestEngineInitInlineManager_NoProject(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg := appconfig.Default()
	store := resources.NewMemoryResourceStore()
	defer func() {
		require.NoError(t, store.Close())
	}()
	engine := &Engine{}
	manager, err := engine.initInlineManager(ctx, cfg, store)
	require.NoError(t, err)
	require.Nil(t, manager)
}

func TestEngineInitInlineManager_NilStore(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg := appconfig.Default()
	engine := &Engine{project: &engineproject.Config{Name: "proj"}}
	manager, err := engine.initInlineManager(ctx, cfg, nil)
	require.Error(t, err)
	require.Nil(t, manager)
}

func TestEngineInitInlineManager_WatchFailure(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	cfg := appconfig.Default()
	cfg.CLI.CWD = t.TempDir()
	store := &failingWatchStore{resources.NewMemoryResourceStore()}
	defer func() {
		require.NoError(t, store.Close())
	}()
	engine := &Engine{project: &engineproject.Config{Name: "proj"}}
	manager, err := engine.initInlineManager(ctx, cfg, store)
	require.Error(t, err)
	require.Nil(t, manager)
}

func TestEngineTakeInlineManager(t *testing.T) {
	engine := &Engine{}
	dummy := &inline.Manager{}
	engine.inlineManager = dummy
	taken := engine.takeInlineManager()
	require.Equal(t, dummy, taken)
	require.Nil(t, engine.inlineManager)
}

func TestCleanupHTTPState(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	listener, err := (&net.ListenConfig{}).Listen(context.WithoutCancel(ctx), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{}
	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	cleanupCtx, cancel := context.WithCancel(ctx)
	state := &httpState{
		server:   server,
		listener: listener,
		cancel:   cancel,
	}
	engine := &Engine{}
	engine.cleanupHTTPState(cleanupCtx, state)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not shut down")
	}
}

func TestAllocateTemporalFrontendPortWraps(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	atomic.StoreInt32(&temporalPortCounter, 59990)
	port := allocateTemporalFrontendPort(ctx, t)
	require.Greater(t, port, 20000)
	require.Less(t, port, 65000)
}

func lifecycleTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	service := appconfig.NewService()
	manager := appconfig.NewManager(ctx, service)
	_, err := manager.Load(ctx, appconfig.NewDefaultProvider())
	require.NoError(t, err)
	ctx = appconfig.ContextWithManager(ctx, manager)
	cfg := appconfig.FromContext(ctx)
	require.NotNil(t, cfg)
	port := allocateTemporalFrontendPort(ctx, t)
	cfg.Temporal.Standalone.FrontendPort = port
	cfg.Temporal.Standalone.UIPort = port + 1000
	cfg.Temporal.Standalone.DatabaseFile = filepath.Join(t.TempDir(), "temporal.db")
	return ctx
}

func allocateTemporalFrontendPort(ctx context.Context, t *testing.T) int {
	t.Helper()
	for attempts := 0; attempts < 2000; attempts++ {
		port := int(atomic.AddInt32(&temporalPortCounter, 1))
		if port > 60000 {
			atomic.StoreInt32(&temporalPortCounter, 20000)
			port = int(atomic.AddInt32(&temporalPortCounter, 1))
		}
		if reserveTCPPort(ctx, port) && reserveTCPPort(ctx, port+1000) {
			return port
		}
	}
	t.Fatalf("unable to allocate temporal port")
	return 0
}

func reserveTCPPort(ctx context.Context, port int) bool {
	if port <= 0 || port > 65000 {
		return false
	}
	ln, err := (&net.ListenConfig{}).Listen(context.WithoutCancel(ctx), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
