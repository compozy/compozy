package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLM_MCP_Durations_ParseFromEnv(t *testing.T) {
	t.Run("Should parse readiness durations from env", func(t *testing.T) {
		t.Setenv("MCP_READINESS_TIMEOUT", "2s")
		t.Setenv("MCP_READINESS_POLL_INTERVAL", "150ms")
		ctx := t.Context()
		m := NewManager(ctx, NewService())
		_, err := m.Load(ctx, NewDefaultProvider(), NewEnvProvider())
		require.NoError(t, err)
		cfg := m.Get()
		require.NotNil(t, cfg)
		assert.Equal(t, 2*time.Second, cfg.LLM.MCPReadinessTimeout)
		assert.Equal(t, 150*time.Millisecond, cfg.LLM.MCPReadinessPollInterval)
		_ = m.Close(ctx)
	})
}

func TestConfig_Default(t *testing.T) {
	t.Run("Should return valid default configuration", func(t *testing.T) {
		// Act
		cfg := Default()

		// Assert
		require.NotNil(t, cfg)

		assert.Equal(t, ModeMemory, cfg.Mode)
		assert.Equal(t, ModeMemory, ResolveMode(cfg, ""))

		// Server defaults
		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
		assert.Equal(t, 5001, cfg.Server.Port)
		assert.True(t, cfg.Server.CORSEnabled)
		assert.Equal(t, 30*time.Second, cfg.Server.Timeout)

		// Database defaults
		assert.Equal(t, "localhost", cfg.Database.Host)
		assert.Equal(t, "5432", cfg.Database.Port)
		assert.Equal(t, "postgres", cfg.Database.User)
		assert.Equal(t, "compozy", cfg.Database.DBName)
		assert.Equal(t, "disable", cfg.Database.SSLMode)
		assert.Empty(t, cfg.Database.Driver)
		assert.Equal(t, ":memory:", cfg.Database.Path)

		// Temporal defaults
		assert.Empty(t, cfg.Temporal.Mode)
		assert.Equal(t, ModeMemory, cfg.EffectiveTemporalMode())
		assert.Equal(t, "localhost:7233", cfg.Temporal.HostPort)
		assert.Equal(t, "default", cfg.Temporal.Namespace)
		assert.Equal(t, "compozy-tasks", cfg.Temporal.TaskQueue)
		assert.Equal(t, ":memory:", cfg.Temporal.Standalone.DatabaseFile)
		assert.Equal(t, 7233, cfg.Temporal.Standalone.FrontendPort)
		assert.Equal(t, "127.0.0.1", cfg.Temporal.Standalone.BindIP)
		assert.Equal(t, cfg.Temporal.Namespace, cfg.Temporal.Standalone.Namespace)
		assert.Equal(t, "compozy-standalone", cfg.Temporal.Standalone.ClusterName)
		assert.True(t, cfg.Temporal.Standalone.EnableUI)
		assert.Equal(t, 8233, cfg.Temporal.Standalone.UIPort)
		assert.Equal(t, "warn", cfg.Temporal.Standalone.LogLevel)
		assert.Equal(t, 30*time.Second, cfg.Temporal.Standalone.StartTimeout)

		// Runtime defaults
		assert.Equal(t, "development", cfg.Runtime.Environment)
		assert.Equal(t, "info", cfg.Runtime.LogLevel)
		assert.Equal(t, 30*time.Second, cfg.Runtime.DispatcherHeartbeatInterval)
		assert.Equal(t, 90*time.Second, cfg.Runtime.DispatcherHeartbeatTTL)
		assert.Equal(t, 120*time.Second, cfg.Runtime.DispatcherStaleThreshold)
		assert.Equal(t, 4, cfg.Runtime.AsyncTokenCounterWorkers)
		assert.Equal(t, 100, cfg.Runtime.AsyncTokenCounterBufferSize)
		assert.True(t, cfg.Runtime.NativeTools.Enabled)
		assert.Equal(t, ".", cfg.Runtime.NativeTools.RootDir)
		assert.Equal(t, 30*time.Second, cfg.Runtime.NativeTools.Exec.Timeout)
		assert.Equal(t, int64(2<<20), cfg.Runtime.NativeTools.Exec.MaxStdoutBytes)
		assert.Equal(t, int64(1<<10), cfg.Runtime.NativeTools.Exec.MaxStderrBytes)
		assert.Empty(t, cfg.Runtime.NativeTools.Exec.Allowlist)
		assert.Equal(t, 5*time.Second, cfg.Runtime.NativeTools.Fetch.Timeout)
		assert.Equal(t, int64(2<<20), cfg.Runtime.NativeTools.Fetch.MaxBodyBytes)
		assert.Equal(t, 5, cfg.Runtime.NativeTools.Fetch.MaxRedirects)
		assert.Equal(
			t,
			[]string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
			cfg.Runtime.NativeTools.Fetch.AllowedMethods,
		)
		assert.True(t, cfg.Runtime.NativeTools.CallAgent.Enabled)
		assert.Equal(t, 60*time.Second, cfg.Runtime.NativeTools.CallAgent.DefaultTimeout)
		assert.True(t, cfg.Runtime.NativeTools.CallAgents.Enabled)
		assert.Equal(t, 60*time.Second, cfg.Runtime.NativeTools.CallAgents.DefaultTimeout)
		assert.Equal(t, DefaultCallAgentsMaxConcurrent, cfg.Runtime.NativeTools.CallAgents.MaxConcurrent)

		// Limits defaults
		assert.Equal(t, 20, cfg.Limits.MaxNestingDepth)
		assert.Equal(t, 100, cfg.Limits.MaxConfigFileNestingDepth)
		assert.Equal(t, 10485760, cfg.Limits.MaxStringLength) // 10MB
		assert.Equal(t, 10485760, cfg.Limits.MaxConfigFileSize)
		assert.Equal(t, 10240, cfg.Limits.MaxMessageContent)
		assert.Equal(t, 102400, cfg.Limits.MaxTotalContentSize)
		assert.Equal(t, 5, cfg.Limits.MaxTaskContextDepth)
		assert.Equal(t, 100, cfg.Limits.ParentUpdateBatchSize)

		// Memory defaults
		assert.Equal(t, "compozy:memory:", cfg.Memory.Prefix)
		assert.Equal(t, 24*time.Hour, cfg.Memory.TTL)
		assert.Equal(t, 10000, cfg.Memory.MaxEntries)

		// LLM usage metrics defaults
		expectedBuckets := []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1}
		assert.Equal(t, expectedBuckets, cfg.LLM.UsageMetrics.PersistBuckets)

		// MCP proxy defaults embed in memory mode with fixed port
		assert.Equal(t, ModeMemory, cfg.MCPProxy.Mode)
		assert.Equal(t, "127.0.0.1", cfg.MCPProxy.Host)
		assert.Equal(t, 6001, cfg.MCPProxy.Port)
		assert.Equal(t, "", cfg.MCPProxy.BaseURL)

		// CLI defaults
		assert.Equal(t, DefaultCLIActiveWindowDays, cfg.CLI.Users.ActiveWindowDays)

		// App mode removed in greenfield cleanup
	})
}

func TestConfig_MemoryModeDefaultsToSQLiteDriver(t *testing.T) {
	t.Run("Should resolve sqlite driver when global mode memory", func(t *testing.T) {
		cfg := Default()
		require.NotNil(t, cfg)
		cfg.Mode = ModeMemory
		cfg.Database.Driver = ""
		cfg.Temporal.Mode = ""
		assert.Equal(t, databaseDriverSQLite, cfg.EffectiveDatabaseDriver())
		assert.Equal(t, ModeMemory, cfg.EffectiveTemporalMode())
	})
}

func TestConfig_ModeValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		mode           string
		wantErr        bool
		wantSubstrings []string
	}{
		{name: "empty inherits memory", mode: ""},
		{name: "memory valid", mode: ModeMemory},
		{name: "persistent valid", mode: ModePersistent},
		{name: "distributed valid", mode: ModeDistributed},
		{
			name:           "standalone invalid",
			mode:           "standalone",
			wantErr:        true,
			wantSubstrings: []string{"standalone", "has been replaced", ModeMemory, ModePersistent},
		},
		{name: "invalid value", mode: "invalid", wantErr: true, wantSubstrings: []string{"must be one of"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := NewService()
			cfg := Default()
			cfg.Mode = tc.mode
			err := svc.Validate(cfg)
			if tc.wantErr {
				require.Error(t, err)
				for _, sub := range tc.wantSubstrings {
					assert.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
			if tc.mode == "" {
				assert.Equal(t, ModeMemory, ResolveMode(cfg, ""))
			}
		})
	}
}

func TestDatabaseConfig(t *testing.T) {
	t.Run("Should default to postgres when driver empty", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "",
			Host:   "localhost",
			Port:   "5432",
			User:   "test",
			DBName: "compozy",
		}
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Equal(t, "postgres", cfg.Driver)
	})

	t.Run("Should accept postgres driver explicitly", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "postgres",
			Host:   "localhost",
			Port:   "5432",
			User:   "test",
			DBName: "compozy",
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("Should accept sqlite driver", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "sqlite",
			Path:   "data/test.db",
		}
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Equal(t, "data"+string(filepath.Separator)+"test.db", cfg.Path)
	})

	t.Run("Should reject invalid driver", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "mysql",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported database driver")
	})

	t.Run("Should require path for sqlite", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "sqlite",
			Path:   "",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires path")
	})

	t.Run("Should require connection params for postgres", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "postgres",
			Host:   "localhost",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires host, port, user, and name")
	})

	t.Run("Should validate sqlite path format", func(t *testing.T) {
		cfg := &DatabaseConfig{
			Driver: "sqlite",
			Path:   "../data/test.db",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot traverse directories")
	})
}

func TestTemporalEmbeddedMode(t *testing.T) {
	t.Run("Should apply embedded defaults when mode set to memory", func(t *testing.T) {
		ctx := t.Context()
		manager := NewManager(ctx, NewService())
		overrides := map[string]any{
			"temporal-mode": ModeMemory,
		}
		cfg, err := manager.Load(ctx, NewDefaultProvider(), NewCLIProvider(overrides))
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, ModeMemory, cfg.Temporal.Mode)
		assert.Equal(t, ":memory:", cfg.Temporal.Standalone.DatabaseFile)
		assert.Equal(t, 7233, cfg.Temporal.Standalone.FrontendPort)
		assert.Equal(t, "127.0.0.1", cfg.Temporal.Standalone.BindIP)
		assert.Equal(t, cfg.Temporal.Namespace, cfg.Temporal.Standalone.Namespace)
		assert.Equal(t, "compozy-standalone", cfg.Temporal.Standalone.ClusterName)
		assert.True(t, cfg.Temporal.Standalone.EnableUI)
		assert.Equal(t, 8233, cfg.Temporal.Standalone.UIPort)
		assert.Equal(t, "warn", cfg.Temporal.Standalone.LogLevel)
		assert.Equal(t, 30*time.Second, cfg.Temporal.Standalone.StartTimeout)
		assert.Equal(t, "localhost:7233", cfg.Temporal.HostPort)
		assert.Equal(t, "default", cfg.Temporal.Namespace)
		_ = manager.Close(ctx)
	})

	t.Run("Should allow host port override in embedded mode", func(t *testing.T) {
		ctx := t.Context()
		manager := NewManager(ctx, NewService())
		overrides := map[string]any{
			"temporal-mode": ModePersistent,
			"temporal-host": "0.0.0.0:9000",
		}
		cfg, err := manager.Load(ctx, NewDefaultProvider(), NewCLIProvider(overrides))
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, ModePersistent, cfg.Temporal.Mode)
		assert.Equal(t, "0.0.0.0:9000", cfg.Temporal.HostPort)
		_ = manager.Close(ctx)
	})
}

func TestLLMConfig_StructuredOutputRetryPrecedence(t *testing.T) {
	t.Setenv("LLM_STRUCTURED_OUTPUT_RETRIES", "3")
	ctx := t.Context()
	manager := NewManager(ctx, NewService())
	cliOverrides := map[string]any{
		"llm-structured-output-retries": 5,
	}
	config, err := manager.Load(ctx, NewDefaultProvider(), NewEnvProvider(), NewCLIProvider(cliOverrides))
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, 5, config.LLM.StructuredOutputRetryAttempts)
	_ = manager.Close(ctx)
}

func TestDefaultNativeToolsConfig(t *testing.T) {
	t.Run("Should return default native tools config", func(t *testing.T) {
		config := DefaultNativeToolsConfig()
		assert.True(t, config.Enabled)
		assert.Equal(t, ".", config.RootDir)
		assert.Nil(t, config.AdditionalRoots)
		assert.Equal(t, 30*time.Second, config.Exec.Timeout)
		assert.Equal(t, int64(2<<20), config.Exec.MaxStdoutBytes)
		assert.Equal(t, int64(1<<10), config.Exec.MaxStderrBytes)
		assert.Nil(t, config.Exec.Allowlist)
		assert.Equal(t, 5*time.Second, config.Fetch.Timeout)
		assert.Equal(t, int64(2<<20), config.Fetch.MaxBodyBytes)
		assert.Equal(t, 5, config.Fetch.MaxRedirects)
		assert.Equal(t, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}, config.Fetch.AllowedMethods)
		assert.True(t, config.CallAgent.Enabled)
		assert.Equal(t, 60*time.Second, config.CallAgent.DefaultTimeout)
		assert.True(t, config.CallAgents.Enabled)
		assert.Equal(t, 60*time.Second, config.CallAgents.DefaultTimeout)
		assert.Equal(t, DefaultCallAgentsMaxConcurrent, config.CallAgents.MaxConcurrent)
	})
}

func TestConfig_Validation(t *testing.T) {
	t.Run("Should validate temporal mode", func(t *testing.T) {
		testCases := []struct {
			name           string
			mode           string
			wantErr        bool
			wantSubstrings []string
		}{
			{name: "remote", mode: ModeRemoteTemporal},
			{name: "memory", mode: ModeMemory},
			{name: "persistent", mode: ModePersistent},
			{name: "empty inherits", mode: ""},
			{
				name:           "standalone invalid",
				mode:           "standalone",
				wantErr:        true,
				wantSubstrings: []string{"standalone", "has been removed", ModeMemory, ModePersistent},
			},
			{name: "invalid", mode: "invalid", wantErr: true, wantSubstrings: []string{"must be one of"}},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Default()
				cfg.Temporal.Mode = tc.mode
				svc := NewService()
				err := svc.Validate(cfg)
				if tc.wantErr {
					require.Error(t, err)
					for _, sub := range tc.wantSubstrings {
						assert.Contains(t, err.Error(), sub)
					}
					return
				}
				require.NoError(t, err)
				assert.NotEmpty(t, cfg.Temporal.Mode)
				if tc.mode == "" {
					assert.Equal(t, ModeMemory, cfg.EffectiveTemporalMode())
					assert.Equal(t, ModeMemory, cfg.Temporal.Mode)
				} else {
					assert.Equal(t, tc.mode, cfg.Temporal.Mode)
				}
			})
		}
	})

	t.Run("Should validate embedded configuration when mode embedded", func(t *testing.T) {
		testCases := []struct {
			name    string
			mutate  func(*TemporalConfig)
			wantErr string
		}{
			{
				name: "missing database file",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.DatabaseFile = ""
				},
				wantErr: "database_file",
			},
			{
				name: "frontend port out of range",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.FrontendPort = 0
				},
				wantErr: "frontend_port",
			},
			{
				name: "bind ip invalid",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.BindIP = "not-an-ip"
				},
				wantErr: "bind_ip",
			},
			{
				name: "missing namespace",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.Namespace = ""
				},
				wantErr: "namespace",
			},
			{
				name: "missing cluster name",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.ClusterName = ""
				},
				wantErr: "cluster_name",
			},
			{
				name: "ui port invalid when ui enabled",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.UIPort = 0
				},
				wantErr: "ui_port",
			},
			{
				name: "invalid log level",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.LogLevel = "trace"
				},
				wantErr: "log_level",
			},
			{
				name: "non positive start timeout",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.StartTimeout = 0
				},
				wantErr: "start_timeout",
			},
			{
				name: "ui disabled allows zero port",
				mutate: func(cfg *TemporalConfig) {
					cfg.Standalone.EnableUI = false
					cfg.Standalone.UIPort = 0
				},
				wantErr: "",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Default()
				cfg.Temporal.Mode = ModeMemory
				tc.mutate(&cfg.Temporal)
				svc := NewService()
				err := svc.Validate(cfg)
				if tc.wantErr == "" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.wantErr)
				}
			})
		}
	})
	t.Run("Should validate server port range", func(t *testing.T) {
		tests := []struct {
			name    string
			port    int
			wantErr bool
		}{
			{"valid port", 5001, false},
			{"minimum port", 1, false},
			{"maximum port", 65535, false},
			{"port too low", 0, true},
			{"port too high", 65536, true},
			{"negative port", -1, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				cfg.Server.Port = tt.port

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate runtime environment", func(t *testing.T) {
		tests := []struct {
			name    string
			env     string
			wantErr bool
		}{
			{"development", "development", false},
			{"staging", "staging", false},
			{"production", "production", false},
			{"invalid", "testing", true},
			{"empty", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				cfg.Runtime.Environment = tt.env

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate log levels", func(t *testing.T) {
		tests := []struct {
			name     string
			logLevel string
			wantErr  bool
		}{
			{"debug", "debug", false},
			{"info", "info", false},
			{"warn", "warn", false},
			{"error", "error", false},
			{"invalid", "verbose", true},
			{"empty", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				cfg.Runtime.LogLevel = tt.logLevel

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate call agents boundaries", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Config)
			wantErr bool
		}{
			{
				name: "allows sequential fallback with zero max concurrent",
				modify: func(cfg *Config) {
					cfg.Runtime.NativeTools.CallAgents.MaxConcurrent = 0
				},
				wantErr: false,
			},
			{
				name: "allows zero timeout to defer to per-request deadlines",
				modify: func(cfg *Config) {
					cfg.Runtime.NativeTools.CallAgents.DefaultTimeout = 0
				},
				wantErr: false,
			},
			{
				name: "rejects negative max concurrent",
				modify: func(cfg *Config) {
					cfg.Runtime.NativeTools.CallAgents.MaxConcurrent = -1
				},
				wantErr: true,
			},
			{
				name: "rejects negative default timeout",
				modify: func(cfg *Config) {
					cfg.Runtime.NativeTools.CallAgents.DefaultTimeout = -1 * time.Second
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				tt.modify(cfg)

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate task execution timeouts", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Config)
			wantErr bool
		}{
			{
				name:    "valid defaults",
				modify:  func(_ *Config) {},
				wantErr: false,
			},
			{
				name: "default must be positive",
				modify: func(c *Config) {
					c.Runtime.TaskExecutionTimeoutDefault = 0
				},
				wantErr: true,
			},
			{
				name: "max must be positive",
				modify: func(c *Config) {
					c.Runtime.TaskExecutionTimeoutMax = 0
				},
				wantErr: true,
			},
			{
				name: "default cannot exceed max",
				modify: func(c *Config) {
					c.Runtime.TaskExecutionTimeoutDefault = 10 * time.Minute
					c.Runtime.TaskExecutionTimeoutMax = 5 * time.Minute
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				tt.modify(cfg)
				svc := NewService()
				err := svc.Validate(cfg)
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate MCP proxy mode values", func(t *testing.T) {
		tests := []struct {
			name           string
			mode           string
			wantErr        bool
			wantSubstrings []string
		}{
			{
				name:    "inherits global mode",
				mode:    "",
				wantErr: false,
			},
			{
				name:    "memory mode allowed",
				mode:    ModeMemory,
				wantErr: false,
			},
			{
				name:    "persistent mode allowed",
				mode:    ModePersistent,
				wantErr: false,
			},
			{
				name:    "distributed mode allowed",
				mode:    ModeDistributed,
				wantErr: false,
			},
			{
				name:           "standalone mode rejected",
				mode:           deprecatedModeStandalone,
				wantErr:        true,
				wantSubstrings: []string{"mcp_proxy.mode", deprecatedModeStandalone, "no longer supported"},
			},
			{
				name:           "unknown mode rejected",
				mode:           "invalid",
				wantErr:        true,
				wantSubstrings: []string{"mcp_proxy.mode", "must be one of"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				svc := NewService()
				cfg := Default()
				cfg.MCPProxy.Mode = tc.mode
				err := svc.Validate(cfg)
				if tc.wantErr {
					require.Error(t, err)
					for _, sub := range tc.wantSubstrings {
						assert.Contains(t, err.Error(), sub)
					}
					return
				}
				assert.NoError(t, err)
			})
		}
	})

	t.Run("Should require non-ephemeral MCP proxy port in embedded modes", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.MCPProxy.Mode = ModeMemory
		cfg.MCPProxy.Port = 0
		err := svc.Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mcp_proxy.port must be non-zero when mode is \"memory\" or \"persistent\"")
	})

	t.Run("Should allow embedded MCP proxy when port provided", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.MCPProxy.Mode = ModePersistent
		cfg.MCPProxy.Port = 6200
		err := svc.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Should default MCP proxy to embedded mode with fixed port", func(t *testing.T) {
		cfg := Default()
		assert.Equal(t, ModeMemory, cfg.MCPProxy.Mode)
		assert.Equal(t, 6001, cfg.MCPProxy.Port)
	})

	t.Run("Should validate limits are positive", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Config)
			wantErr bool
		}{
			{
				"valid limits",
				func(_ *Config) {},
				false,
			},
			{
				"zero nesting depth",
				func(c *Config) { c.Limits.MaxNestingDepth = 0 },
				true,
			},
			{
				"negative string length",
				func(c *Config) { c.Limits.MaxStringLength = -1 },
				true,
			},
			{
				"zero message content",
				func(c *Config) { c.Limits.MaxMessageContent = 0 },
				true,
			},
			{
				"zero workers",
				func(c *Config) { c.Runtime.AsyncTokenCounterWorkers = 0 },
				true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				tt.modify(cfg)

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate dispatcher timing constraints", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Config)
			wantErr bool
			errMsg  string
		}{
			{
				"valid timing",
				func(_ *Config) {},
				false,
				"",
			},
			{
				"TTL less than interval",
				func(c *Config) {
					c.Runtime.DispatcherHeartbeatInterval = 60 * time.Second
					c.Runtime.DispatcherHeartbeatTTL = 30 * time.Second
				},
				true,
				"dispatcher heartbeat TTL must be greater than heartbeat interval",
			},
			{
				"stale threshold less than TTL",
				func(c *Config) {
					c.Runtime.DispatcherHeartbeatTTL = 90 * time.Second
					c.Runtime.DispatcherStaleThreshold = 60 * time.Second
				},
				true,
				"dispatcher stale threshold must be greater than heartbeat TTL",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				tt.modify(cfg)

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
					if tt.errMsg != "" {
						assert.Contains(t, err.Error(), tt.errMsg)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate database configuration", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Config)
			wantErr bool
		}{
			{
				"valid with connection string",
				func(c *Config) {
					c.Database.ConnString = "postgres://user:pass@localhost/db"
				},
				false,
			},
			{
				"valid with individual components",
				func(c *Config) {
					c.Database.ConnString = ""
					c.Database.Host = "localhost"
					c.Database.Port = "5432"
					c.Database.User = "postgres"
					c.Database.DBName = "compozy"
				},
				false,
			},
			{
				"missing host",
				func(c *Config) {
					c.Database.ConnString = ""
					c.Database.Host = ""
				},
				true,
			},
			{
				"missing port",
				func(c *Config) {
					c.Database.ConnString = ""
					c.Database.Port = ""
				},
				true,
			},
			{
				"missing user",
				func(c *Config) {
					c.Database.ConnString = ""
					c.Database.User = ""
				},
				true,
			},
			{
				"missing dbname",
				func(c *Config) {
					c.Database.ConnString = ""
					c.Database.DBName = ""
				},
				true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				cfg.Mode = ModeDistributed
				tt.modify(cfg)

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Should validate temporal configuration", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Config)
			wantErr bool
		}{
			{
				"valid temporal config",
				func(_ *Config) {},
				false,
			},
			{
				"missing host port",
				func(c *Config) {
					c.Temporal.HostPort = ""
				},
				true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				cfg.Mode = ModeDistributed
				tt.modify(cfg)

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestCacheConfig_Defaults(t *testing.T) {
	t.Run("Should have correct default values", func(t *testing.T) {
		cacheConfig := Default().Cache

		// Test cache-specific defaults
		assert.True(t, cacheConfig.Enabled, "cache should be enabled by default")
		assert.Equal(t, 24*time.Hour, cacheConfig.TTL, "cache TTL should default to 24h")
		assert.Equal(t, "compozy:cache:", cacheConfig.Prefix, "cache prefix should have correct default")
		assert.Equal(t, int64(1048576), cacheConfig.MaxItemSize, "max item size should be 1MB")
		assert.True(t, cacheConfig.CompressionEnabled, "compression should be enabled by default")
		assert.Equal(t, int64(1024), cacheConfig.CompressionThreshold, "compression threshold should be 1KB")
		assert.Equal(t, "lru", cacheConfig.EvictionPolicy, "eviction policy should default to lru")
		assert.Equal(t, 5*time.Minute, cacheConfig.StatsInterval, "stats interval should default to 5m")
		assert.Equal(t, 100, cacheConfig.KeyScanCount, "key scan count should default to 100")
	})
}

func TestCacheConfig_Separation(t *testing.T) {
	t.Run("Should be separate from Redis configuration", func(t *testing.T) {
		cfg := Default()
		cacheConfig := Default().Cache

		// Verify that CacheConfig doesn't have Redis connection properties
		// This is implicitly tested by the struct definition having only cache-specific fields

		// Verify Redis config exists separately
		assert.Equal(t, "localhost", cfg.Redis.Host, "Redis should have separate host config")
		assert.Equal(t, "6379", cfg.Redis.Port, "Redis should have separate port config")

		// Verify cache config has its own properties accessed through Default().Cache
		assert.NotEmpty(t, cacheConfig.Prefix, "Cache should have its own prefix")
		assert.NotZero(t, cacheConfig.TTL, "Cache should have its own TTL")
	})
}

func TestRedisPortValidation(t *testing.T) {
	t.Run("Should validate Redis port configuration", func(t *testing.T) {
		tests := []struct {
			name     string
			port     string
			wantErr  bool
			errorMsg string
		}{
			{
				"valid port string",
				"6379",
				false,
				"",
			},
			{
				"valid min port",
				"1",
				false,
				"",
			},
			{
				"valid max port",
				"65535",
				false,
				"",
			},
			{
				"empty port (uses default)",
				"",
				false,
				"",
			},
			{
				"invalid port - zero",
				"0",
				true,
				"Redis port must be between 1 and 65535",
			},
			{
				"invalid port - too high",
				"65536",
				true,
				"Redis port must be between 1 and 65535",
			},
			{
				"invalid port - non-numeric",
				"abc",
				true,
				"Redis port must be a valid integer",
			},
			{
				"invalid port - negative",
				"-1",
				true,
				"Redis port must be between 1 and 65535",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := Default()
				cfg.Redis.Port = tt.port

				svc := NewService()
				err := svc.Validate(cfg)

				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "validation failed")
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
