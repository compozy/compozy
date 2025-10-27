package embedded

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/server/common/primitives"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	valid := Config{
		DatabaseFile: ":memory:",
		FrontendPort: 7233,
		BindIP:       "127.0.0.1",
		Namespace:    "default",
		ClusterName:  "cluster",
		EnableUI:     true,
		UIPort:       8233,
		LogLevel:     "info",
		StartTimeout: time.Second,
	}

	cases := []struct {
		name    string
		mutate  func(cfg *Config)
		wantErr string
	}{
		{
			name:   "valid configuration passes",
			mutate: func(_ *Config) {},
		},
		{
			name: "invalid frontend port",
			mutate: func(cfg *Config) {
				cfg.FrontendPort = -1
			},
			wantErr: "frontend_port",
		},
		{
			name: "service port overflow",
			mutate: func(cfg *Config) {
				cfg.FrontendPort = 65534
			},
			wantErr: "out-of-range",
		},
		{
			name: "invalid ui port",
			mutate: func(cfg *Config) {
				cfg.UIPort = 0
			},
			wantErr: "ui_port",
		},
		{
			name: "invalid bind ip",
			mutate: func(cfg *Config) {
				cfg.BindIP = "not-an-ip"
			},
			wantErr: "invalid bind IP",
		},
		{
			name: "invalid log level",
			mutate: func(cfg *Config) {
				cfg.LogLevel = "verbose"
			},
			wantErr: "invalid log level",
		},
		{
			name: "invalid database path",
			mutate: func(cfg *Config) {
				cfg.DatabaseFile = filepath.Join(string(filepath.Separator), "does", "not", "exist", "temporal.db")
			},
			wantErr: "database directory",
		},
		{
			name: "missing namespace",
			mutate: func(cfg *Config) {
				cfg.Namespace = ""
			},
			wantErr: "namespace is required",
		},
		{
			name: "missing cluster name",
			mutate: func(cfg *Config) {
				cfg.ClusterName = ""
			},
			wantErr: "cluster name is required",
		},
		{
			name: "invalid start timeout",
			mutate: func(cfg *Config) {
				cfg.StartTimeout = 0
			},
			wantErr: "start timeout",
		},
		{
			name: "ui disabled allows zero port",
			mutate: func(cfg *Config) {
				cfg.EnableUI = false
				cfg.UIPort = 0
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			err := validateConfig(&cfg)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	applyDefaults(cfg)

	assert.Equal(t, defaultDatabaseFile, cfg.DatabaseFile)
	assert.Equal(t, defaultFrontendPort, cfg.FrontendPort)
	assert.Equal(t, defaultBindIP, cfg.BindIP)
	assert.Equal(t, defaultNamespace, cfg.Namespace)
	assert.Equal(t, defaultClusterName, cfg.ClusterName)
	assert.Equal(t, defaultUIPort, cfg.UIPort)
	assert.Equal(t, defaultLogLevel, cfg.LogLevel)
	assert.Equal(t, defaultStartTimeout, cfg.StartTimeout)
	assert.True(t, cfg.EnableUI)
}

func TestBuildSQLiteConnectAttrs(t *testing.T) {
	t.Parallel()

	memoryAttrs := buildSQLiteConnectAttrs(&Config{DatabaseFile: ":memory:"})
	require.Equal(t, map[string]string{
		"mode":  "memory",
		"cache": "shared",
	}, memoryAttrs)

	fileAttrs := buildSQLiteConnectAttrs(&Config{DatabaseFile: "temporal.db"})
	require.Equal(t, map[string]string{
		"cache":        "private",
		"journal_mode": "wal",
		"synchronous":  "2",
		"setup":        "true",
	}, fileAttrs)
}

func TestBuildStaticHosts(t *testing.T) {
	t.Parallel()

	cfg := &Config{BindIP: "127.0.0.1", FrontendPort: 7233}
	hosts := buildStaticHosts(cfg)

	assert.Equal(t, "127.0.0.1:7233", hosts[primitives.FrontendService].Self)
	assert.Equal(t, []string{"127.0.0.1:7233"}, hosts[primitives.FrontendService].All)
	assert.Equal(t, "127.0.0.1:7234", hosts[primitives.HistoryService].Self)
	assert.Equal(t, "127.0.0.1:7235", hosts[primitives.MatchingService].Self)
	assert.Equal(t, "127.0.0.1:7236", hosts[primitives.WorkerService].Self)
}
