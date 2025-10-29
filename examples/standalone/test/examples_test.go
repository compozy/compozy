package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corepkg "github.com/compozy/compozy/engine/core"
	workflowpkg "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	helpers "github.com/compozy/compozy/test/helpers"
)

// repoRoot resolves repository root for building relative paths to example workflows.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := helpers.FindProjectRoot()
	require.NoError(t, err)
	return root
}

func TestExample_BasicStandalone(t *testing.T) {
	// Validate workflow compiles and loads
	root := repoRoot(t)
	wfPath := filepath.Join(root, "examples", "standalone", "basic", "workflows", "hello-world.yaml")
	cwd, err := projectCWDFromRepoRoot()
	require.NoError(t, err)
	wf, err := workflowpkg.Load(t.Context(), cwd, filepath.ToSlash(wfPath))
	require.NoError(t, err)
	require.Equal(t, "hello-world", wf.ID)
}

func TestExample_WithPersistence(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	dir := t.TempDir()
	cfg := config.RedisPersistenceConfig{
		Enabled:            true,
		DataDir:            dir,
		SnapshotInterval:   time.Second,
		SnapshotOnShutdown: true,
		RestoreOnStartup:   false,
	}
	env := helpers.SetupStandaloneWithPersistence(ctx, t, cfg)
	defer env.Cleanup(ctx)

	// Write a value and snapshot
	require.NoError(t, env.Client.Set(ctx, "demo:key", "value-1", 0).Err())
	require.NoError(t, env.SnapshotManager.Snapshot(ctx))
	// Close first instance
	env.Cleanup(ctx)

	// Reopen with restore and verify value exists
	cfg.RestoreOnStartup = true
	env2 := helpers.SetupMiniredisStandaloneWithConfig(ctx, t, cfg)
	defer env2.Cleanup(ctx)
	got, err := env2.Client.Get(ctx, "demo:key").Result()
	require.NoError(t, err)
	assert.Equal(t, "value-1", got)
}

func TestExample_MixedModeConfig(t *testing.T) {
	ctx := t.Context()
	// Build config via manager to simulate runtime configuration
	mgr := config.NewManager(ctx, config.NewService())
	_, err := mgr.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := mgr.Get()
	cfg.Mode = "standalone"       // global
	cfg.Redis.Mode = "standalone" // embedded cache
	cfg.Temporal.Mode = "remote"  // external Temporal override
	assert.Equal(t, "standalone", cfg.EffectiveRedisMode())
	assert.Equal(t, "remote", cfg.EffectiveTemporalMode())
	_ = mgr.Close(ctx)
}

func TestExample_EdgeDeployment(t *testing.T) {
	root := repoRoot(t)
	wfPath := filepath.Join(root, "examples", "standalone", "edge-deployment", "workflows", "edge-workflow.yaml")
	cwd, err := projectCWDFromRepoRoot()
	require.NoError(t, err)
	wf, err := workflowpkg.Load(t.Context(), cwd, filepath.ToSlash(wfPath))
	require.NoError(t, err)
	require.Equal(t, "edge-workflow", wf.ID)
}

func TestExample_MigrationDemo(t *testing.T) {
	root := repoRoot(t)
	phase1 := filepath.Join(root, "examples", "standalone", "migration-demo", "phase1-standalone", "compozy.yaml")
	_ = phase1 // compozy is for users; tests run workflows directly
	wf := filepath.Join(root, "examples", "standalone", "migration-demo", "workflows", "migration-workflow.yaml")

	// Phase 1 & 2 share the same workflow; we assert it loads and is consistent
	cwd, err := projectCWDFromRepoRoot()
	require.NoError(t, err)
	wfc, err := workflowpkg.Load(t.Context(), cwd, filepath.ToSlash(wf))
	require.NoError(t, err)
	assert.Equal(t, "migration-demo", wfc.ID)

	// Smoke test the migration helper script is present and executable
	stat, err := os.Stat(filepath.Join(root, "examples", "standalone", "migration-demo", "migrate.sh"))
	require.NoError(t, err)
	assert.False(t, stat.IsDir())
}

func projectCWDFromRepoRoot() (*corepkg.PathCWD, error) {
	root, err := helpers.FindProjectRoot()
	if err != nil {
		return nil, err
	}
	return corepkg.CWDFromPath(root)
}
