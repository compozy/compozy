package server

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareServerWithConfig(
	t *testing.T,
	mutate func(*config.Config),
	output *bytes.Buffer,
) (*Server, *config.Config) {
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
		Level:      logger.DebugLevel,
		Output:     output,
		JSON:       false,
		AddSource:  false,
		TimeFormat: time.RFC3339,
	}
	ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logCfg))
	return &Server{ctx: ctx}, cfg
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(input string) string {
	return ansiRegexp.ReplaceAllString(input, "")
}

func TestValidateDatabaseConfig(t *testing.T) {
	t.Run("Should pass postgres with pgvector", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, cfg := prepareServerWithConfig(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverPostgres
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "pgvector"}}
		}, buffer)
		require.NoError(t, srv.validateDatabaseConfig(cfg))
		assert.Empty(t, stripANSI(buffer.String()))
	})

	t.Run("Should pass sqlite with qdrant", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, cfg := prepareServerWithConfig(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = "./test.db"
			cfg.Worker.MaxConcurrentWorkflowExecutionSize = 0
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "qdrant"}}
		}, buffer)
		require.NoError(t, srv.validateDatabaseConfig(cfg))
		assert.NotContains(t, stripANSI(buffer.String()), "concurrency limitations")
	})

	t.Run("Should fail sqlite with pgvector", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, cfg := prepareServerWithConfig(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = "./test.db"
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "pgvector"}}
		}, buffer)
		err := srv.validateDatabaseConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pgvector provider is incompatible with SQLite driver")
		assert.Contains(t, err.Error(), "docs/database/sqlite.md")
	})

	t.Run("Should warn sqlite without vector db", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, cfg := prepareServerWithConfig(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = "./test.db"
			cfg.Worker.MaxConcurrentWorkflowExecutionSize = 0
			cfg.Knowledge.VectorDBs = nil
		}, buffer)
		require.NoError(t, srv.validateDatabaseConfig(cfg))
		logOutput := stripANSI(buffer.String())
		assert.Contains(
			t,
			logOutput,
			"SQLite driver configured without vector database - knowledge features will not work",
		)
		assert.Contains(t, logOutput, "recommendation")
	})

	t.Run("Should warn sqlite with high concurrency", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		srv, cfg := prepareServerWithConfig(t, func(cfg *config.Config) {
			cfg.Database.Driver = driverSQLite
			cfg.Database.Path = "./test.db"
			cfg.Worker.MaxConcurrentWorkflowExecutionSize = 50
			cfg.Knowledge.VectorDBs = []config.VectorDBConfig{{Provider: "qdrant"}}
		}, buffer)
		require.NoError(t, srv.validateDatabaseConfig(cfg))
		logOutput := stripANSI(buffer.String())
		assert.Contains(t, logOutput, "SQLite has concurrency limitations")
		assert.Contains(t, logOutput, "recommended_max=10")
	})
}
