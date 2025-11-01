package inline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/inline"
	testhelpers "github.com/compozy/compozy/test/helpers"
)

func TestManagerSyncWritesModules(t *testing.T) {
	t.Parallel()
	ctx := testhelpers.NewTestContext(t)
	tmp := t.TempDir()
	store := resources.NewMemoryResourceStore()
	manager, err := inline.NewManager(ctx, inline.Options{
		ProjectRoot: tmp,
		ProjectName: "proj-inline",
		Store:       store,
	})
	require.NoError(t, err)
	toolCfg := &tool.Config{
		ID:             "weather-brief",
		Implementation: tool.ImplementationRuntime,
		Code:           "export default () => 'sunny';",
	}
	_, err = store.Put(ctx, resources.ResourceKey{
		Project: "proj-inline",
		Type:    resources.ResourceTool,
		ID:      "weather-brief",
	}, toolCfg)
	require.NoError(t, err)
	require.NoError(t, manager.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, manager.Close(ctx))
	})
	modulePath, ok := manager.ModulePath("weather-brief")
	require.True(t, ok)
	data, err := os.ReadFile(modulePath)
	require.NoError(t, err)
	require.Equal(t, "export default () => 'sunny';\n", string(data))
	entry, err := os.ReadFile(manager.EntrypointPath())
	require.NoError(t, err)
	require.Contains(t, string(entry), `"weather-brief"`)
	require.Contains(t, string(entry), "inline0")
}

func TestManagerSyncIsConcurrencySafe(t *testing.T) {
	t.Parallel()
	ctx := testhelpers.NewTestContext(t)
	tmp := t.TempDir()
	store := resources.NewMemoryResourceStore()
	manager, err := inline.NewManager(ctx, inline.Options{
		ProjectRoot: tmp,
		ProjectName: "proj-concurrent",
		Store:       store,
	})
	require.NoError(t, err)
	toolCfg := &tool.Config{
		ID:             "metrics-report",
		Implementation: tool.ImplementationRuntime,
		Code:           "export default () => ({ status: 'ok' });",
	}
	_, err = store.Put(ctx, resources.ResourceKey{
		Project: "proj-concurrent",
		Type:    resources.ResourceTool,
		ID:      "metrics-report",
	}, toolCfg)
	require.NoError(t, err)
	require.NoError(t, manager.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, manager.Close(ctx))
	})
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			errCh <- manager.Sync(ctx)
		}()
	}
	for i := 0; i < 8; i++ {
		require.NoError(t, <-errCh)
	}
	modulePath, ok := manager.ModulePath("metrics-report")
	require.True(t, ok)
	_, err = os.Stat(modulePath)
	require.NoError(t, err)
}

func TestManagerWatcherRegeneratesOnUpdates(t *testing.T) {
	t.Parallel()
	ctx := testhelpers.NewTestContext(t)
	tmp := t.TempDir()
	store := resources.NewMemoryResourceStore()
	manager, err := inline.NewManager(ctx, inline.Options{
		ProjectRoot:    tmp,
		ProjectName:    "proj-watch",
		Store:          store,
		UserEntrypoint: filepath.Join("src", "entrypoint.ts"),
	})
	require.NoError(t, err)
	initial := &tool.Config{
		ID:             "alpha",
		Implementation: tool.ImplementationRuntime,
		Code:           "export default () => 'v1';",
	}
	_, err = store.Put(ctx, resources.ResourceKey{
		Project: "proj-watch",
		Type:    resources.ResourceTool,
		ID:      "alpha",
	}, initial)
	require.NoError(t, err)
	require.NoError(t, manager.Start(ctx))
	t.Cleanup(func() {
		require.NoError(t, manager.Close(ctx))
	})
	initialPath, ok := manager.ModulePath("alpha")
	require.True(t, ok)
	data, err := os.ReadFile(initialPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "'v1'")
	updated := &tool.Config{
		ID:             "alpha",
		Implementation: tool.ImplementationRuntime,
		Code:           "export default () => 'v2';",
	}
	_, err = store.Put(ctx, resources.ResourceKey{
		Project: "proj-watch",
		Type:    resources.ResourceTool,
		ID:      "alpha",
	}, updated)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		modulePath, ok := manager.ModulePath("alpha")
		if !ok {
			return false
		}
		content, readErr := os.ReadFile(modulePath)
		if readErr != nil {
			return false
		}
		return strings.Contains(string(content), "'v2'")
	}, 2*time.Second, 50*time.Millisecond)
	entryContent, err := os.ReadFile(manager.EntrypointPath())
	require.NoError(t, err)
	require.Contains(t, string(entryContent), "alpha")
	require.Contains(t, string(entryContent), "inlineExports")
}
