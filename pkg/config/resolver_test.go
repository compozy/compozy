package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMode_ExplicitComponentMode(t *testing.T) {
	t.Run("Should return component mode when explicitly set", func(t *testing.T) {
		cfg := &Config{
			Mode:  "standalone",
			Redis: RedisConfig{Mode: "distributed"},
		}
		result := cfg.EffectiveRedisMode()
		assert.Equal(t, "distributed", result)
	})
}

func TestResolveMode_InheritAndDefault(t *testing.T) {
	t.Run("Should inherit from global mode", func(t *testing.T) {
		cfg := &Config{
			Mode:  "standalone",
			Redis: RedisConfig{Mode: ""},
		}
		result := cfg.EffectiveRedisMode()
		assert.Equal(t, "standalone", result)
	})

	t.Run("Should default to distributed", func(t *testing.T) {
		cfg := &Config{
			Mode:  "",
			Redis: RedisConfig{Mode: ""},
		}
		result := cfg.EffectiveRedisMode()
		assert.Equal(t, "distributed", result)
	})
}

func TestEffectiveTemporalMode_Normalization(t *testing.T) {
	t.Run("Should normalize distributed to remote for Temporal", func(t *testing.T) {
		cfg := &Config{Mode: "distributed"}
		result := cfg.EffectiveTemporalMode()
		assert.Equal(t, "remote", result)
	})

	t.Run("Should pass through standalone for Temporal", func(t *testing.T) {
		cfg := &Config{Mode: "standalone"}
		result := cfg.EffectiveTemporalMode()
		assert.Equal(t, "standalone", result)
	})

	t.Run("Should fallback to global mode when component unset", func(t *testing.T) {
		cfg := &Config{Mode: ModeStandalone}
		result := cfg.EffectiveTemporalMode()
		assert.Equal(t, ModeStandalone, result)
	})
}

func TestEffectiveMCPProxyMode_Resolution(t *testing.T) {
	t.Run("inherit global when component empty", func(t *testing.T) {
		cfg := &Config{Mode: "standalone"}
		got := cfg.EffectiveMCPProxyMode()
		assert.Equal(t, "standalone", got)
	})

	t.Run("prefer component over global", func(t *testing.T) {
		cfg := &Config{Mode: "standalone", MCPProxy: MCPProxyConfig{Mode: ""}}
		assert.Equal(t, "standalone", cfg.EffectiveMCPProxyMode())
		cfg.MCPProxy.Mode = "distributed"
		assert.Equal(t, "distributed", cfg.EffectiveMCPProxyMode())
	})
}

func TestEffectiveDatabaseDriver(t *testing.T) {
	t.Run("Should default to sqlite when global mode standalone", func(t *testing.T) {
		cfg := &Config{Mode: ModeStandalone}
		assert.Equal(t, databaseDriverSQLite, cfg.EffectiveDatabaseDriver())
	})

	t.Run("Should default to postgres when mode distributed", func(t *testing.T) {
		cfg := &Config{}
		assert.Equal(t, databaseDriverPostgres, cfg.EffectiveDatabaseDriver())
	})

	t.Run("Should respect explicit postgres override", func(t *testing.T) {
		cfg := &Config{Mode: ModeStandalone, Database: DatabaseConfig{Driver: "postgres"}}
		assert.Equal(t, databaseDriverPostgres, cfg.EffectiveDatabaseDriver())
	})

	t.Run("Should respect explicit sqlite override", func(t *testing.T) {
		cfg := &Config{Database: DatabaseConfig{Driver: "sqlite"}}
		assert.Equal(t, databaseDriverSQLite, cfg.EffectiveDatabaseDriver())
	})
}
