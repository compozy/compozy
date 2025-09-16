package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLM_MCP_Durations_ParseFromEnv(t *testing.T) {
	t.Run("Should parse readiness durations from env", func(t *testing.T) {
		t.Setenv("MCP_READINESS_TIMEOUT", "2s")
		t.Setenv("MCP_READINESS_POLL_INTERVAL", "150ms")
		ctx := context.Background()
		m := NewManager(NewService())
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

		// Temporal defaults
		assert.Equal(t, "localhost:7233", cfg.Temporal.HostPort)
		assert.Equal(t, "default", cfg.Temporal.Namespace)
		assert.Equal(t, "compozy-tasks", cfg.Temporal.TaskQueue)

		// Runtime defaults
		assert.Equal(t, "development", cfg.Runtime.Environment)
		assert.Equal(t, "info", cfg.Runtime.LogLevel)
		assert.Equal(t, 30*time.Second, cfg.Runtime.DispatcherHeartbeatInterval)
		assert.Equal(t, 90*time.Second, cfg.Runtime.DispatcherHeartbeatTTL)
		assert.Equal(t, 120*time.Second, cfg.Runtime.DispatcherStaleThreshold)
		assert.Equal(t, 4, cfg.Runtime.AsyncTokenCounterWorkers)
		assert.Equal(t, 100, cfg.Runtime.AsyncTokenCounterBufferSize)

		// Limits defaults
		assert.Equal(t, 20, cfg.Limits.MaxNestingDepth)
		assert.Equal(t, 10485760, cfg.Limits.MaxStringLength) // 10MB
		assert.Equal(t, 10240, cfg.Limits.MaxMessageContent)
		assert.Equal(t, 102400, cfg.Limits.MaxTotalContentSize)
		assert.Equal(t, 5, cfg.Limits.MaxTaskContextDepth)
		assert.Equal(t, 100, cfg.Limits.ParentUpdateBatchSize)

		// Memory defaults
		assert.Equal(t, "compozy:memory:", cfg.Memory.Prefix)
		assert.Equal(t, 24*time.Hour, cfg.Memory.TTL)
		assert.Equal(t, 10000, cfg.Memory.MaxEntries)

		// MCP proxy defaults preserve classic external port
		assert.Equal(t, mcpProxyModeStandalone, cfg.MCPProxy.Mode)
		assert.Equal(t, "127.0.0.1", cfg.MCPProxy.Host)
		assert.Equal(t, 6001, cfg.MCPProxy.Port)
		assert.Equal(t, "", cfg.MCPProxy.BaseURL)

		// App mode removed in greenfield cleanup
	})
}

func TestConfig_Validation(t *testing.T) {
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

	t.Run("Should require non-ephemeral MCP proxy port when standalone", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.MCPProxy.Mode = mcpProxyModeStandalone
		cfg.MCPProxy.Port = 0
		err := svc.Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mcp_proxy.port must be non-zero in standalone mode")
	})

	t.Run("Should allow standalone MCP proxy when port provided", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.MCPProxy.Mode = mcpProxyModeStandalone
		cfg.MCPProxy.Port = 6200
		err := svc.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Should default MCP proxy to standalone with fixed port", func(t *testing.T) {
		cfg := Default()
		assert.Equal(t, mcpProxyModeStandalone, cfg.MCPProxy.Mode)
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
