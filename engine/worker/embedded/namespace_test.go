package embedded

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/metrics"
	persistenceSQL "go.temporal.io/server/common/persistence/sql"
	"go.temporal.io/server/common/persistence/sql/sqlplugin"
	"go.temporal.io/server/common/resolver"
)

func TestCreateNamespace(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := &Config{
		DatabaseFile: filepath.Join(tempDir, "temporal.db"),
		FrontendPort: 7400,
		BindIP:       "127.0.0.1",
		Namespace:    "standalone",
		ClusterName:  "cluster",
		EnableUI:     true,
		UIPort:       8300,
		LogLevel:     "info",
		StartTimeout: 15 * time.Second,
	}

	require.NoError(t, validateConfig(cfg))

	temporalCfg, err := buildTemporalConfig(cfg)
	require.NoError(t, err)

	require.NoError(t, createNamespace(t.Context(), temporalCfg, cfg))

	sqlCfg := temporalCfg.Persistence.DataStores[temporalCfg.Persistence.DefaultStore].SQL
	db, err := persistenceSQL.NewSQLDB(
		sqlplugin.DbKindUnknown,
		sqlCfg,
		resolver.NewNoopResolver(),
		log.NewNoopLogger(),
		metrics.NoopMetricsHandler,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	ctx := t.Context()
	rows, err := db.SelectFromNamespace(ctx, sqlplugin.NamespaceFilter{Name: &cfg.Namespace})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	require.NoError(t, createNamespace(t.Context(), temporalCfg, cfg))
}
