package server

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStartupServer(t *testing.T, mutate func(*config.Config), output *bytes.Buffer) (*Server, *config.Config) {
	t.Helper()
	ctx := t.Context()
	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	if mutate != nil {
		mutate(cfg)
	}
	ctx = config.ContextWithManager(ctx, manager)
	logCfg := &logger.Config{
		Level:      logger.InfoLevel,
		Output:     output,
		JSON:       false,
		AddSource:  false,
		TimeFormat: time.RFC3339,
	}
	ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logCfg))
	srv, err := NewServer(ctx, t.TempDir(), "", "")
	require.NoError(t, err)
	return srv, cfg
}

var ansiStripper = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func cleanLog(input string) string {
	return ansiStripper.ReplaceAllString(input, "")
}

func TestServerStartup(t *testing.T) {
	t.Run("Should start with postgres driver", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		pool, cleanup := helpers.GetSharedPostgresDB(t)
		t.Cleanup(cleanup)
		require.NoError(t, helpers.EnsureTablesExistForTest(pool))
		srv, _ := newStartupServer(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverPostgres
			cfg.Database.ConnString = pool.Config().ConnString()
			cfg.Database.AutoMigrate = false
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "pgvector"}}
		}, buffer)
		provider, cleanupStore, err := srv.setupStore()
		require.NoError(t, err)
		t.Cleanup(cleanupStore)
		assert.Equal(t, driverPostgres, provider.Driver())
		logOutput := cleanLog(buffer.String())
		assert.Contains(t, logOutput, "driver=postgres")
	})

	t.Run("Should start with sqlite driver", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, _ := newStartupServer(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = filepath.Join(t.TempDir(), "compozy.db")
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "qdrant"}}
		}, buffer)
		provider, cleanupStore, err := srv.setupStore()
		require.NoError(t, err)
		t.Cleanup(cleanupStore)
		assert.Equal(t, driverSQLite, provider.Driver())
		logOutput := cleanLog(buffer.String())
		assert.Contains(t, logOutput, "driver=sqlite")
		assert.Contains(t, logOutput, "vector_db_required=true")
	})

	t.Run("Should fail with invalid driver", func(t *testing.T) {
		srv, _ := newStartupServer(t, func(cfg *config.Config) {
			cfg.Database.Driver = "mysql"
		}, bytes.NewBuffer(nil))
		provider, cleanupStore, err := srv.setupStore()
		require.Error(t, err)
		assert.Nil(t, provider)
		assert.Nil(t, cleanupStore)
		assert.Contains(t, err.Error(), "unsupported database driver")
	})

	t.Run("Should fail sqlite plus pgvector", func(t *testing.T) {
		srv, _ := newStartupServer(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = filepath.Join(t.TempDir(), "compozy.db")
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "pgvector"}}
		}, bytes.NewBuffer(nil))
		provider, cleanupStore, err := srv.setupStore()
		require.Error(t, err)
		assert.Nil(t, provider)
		assert.Nil(t, cleanupStore)
		assert.Contains(t, err.Error(), "pgvector provider is incompatible with SQLite driver")
	})

	t.Run("Should log driver information", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, _ := newStartupServer(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = ":memory:"
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "redis"}}
		}, buffer)
		provider, cleanupStore, err := srv.setupStore()
		require.NoError(t, err)
		t.Cleanup(cleanupStore)
		assert.Equal(t, driverSQLite, provider.Driver())
		logOutput := cleanLog(buffer.String())
		assert.Contains(t, logOutput, "mode=in-memory")
		assert.Contains(t, logOutput, "concurrency_limit")
	})
}
