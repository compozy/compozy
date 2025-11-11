package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		globalMode     string
		componentMode  string
		useNilConfig   bool
		expectedResult string
	}{
		{
			name:           "Component mode overrides global",
			globalMode:     ModeDistributed,
			componentMode:  ModeMemory,
			expectedResult: ModeMemory,
		},
		{
			name:           "Component mode overrides with persistent",
			globalMode:     ModeMemory,
			componentMode:  ModePersistent,
			expectedResult: ModePersistent,
		},
		{
			name:           "Component mode overrides with distributed",
			globalMode:     ModePersistent,
			componentMode:  ModeDistributed,
			expectedResult: ModeDistributed,
		},
		{
			name:           "Fallback to global persistent",
			globalMode:     ModePersistent,
			expectedResult: ModePersistent,
		},
		{
			name:           "Fallback to global distributed",
			globalMode:     ModeDistributed,
			expectedResult: ModeDistributed,
		},
		{
			name:           "Default to memory when unset",
			expectedResult: ModeMemory,
		},
		{
			name:           "Nil config defaults to memory",
			useNilConfig:   true,
			expectedResult: ModeMemory,
		},
		{
			name:           "Nil config respects component override",
			componentMode:  ModeDistributed,
			useNilConfig:   true,
			expectedResult: ModeDistributed,
		},
		{
			name:           "Component mode remote",
			globalMode:     ModeDistributed,
			componentMode:  ModeRemoteTemporal,
			expectedResult: ModeRemoteTemporal,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cfg *Config
			if !tc.useNilConfig {
				cfg = &Config{Mode: tc.globalMode}
			}
			result := ResolveMode(cfg, tc.componentMode)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestEffectiveRedisMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		globalMode    string
		componentMode string
		want          string
	}{
		{
			name:          "Component override to memory",
			globalMode:    ModeDistributed,
			componentMode: ModeMemory,
			want:          ModeMemory,
		},
		{
			name:       "Inherit persistent mode",
			globalMode: ModePersistent,
			want:       ModePersistent,
		},
		{
			name:       "Inherit distributed mode",
			globalMode: ModeDistributed,
			want:       ModeDistributed,
		},
		{
			name: "Default to memory when global empty",
			want: ModeMemory,
		},
		{
			name:          "Component override to distributed",
			globalMode:    ModeMemory,
			componentMode: ModeDistributed,
			want:          ModeDistributed,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{Mode: tc.globalMode, Redis: RedisConfig{Mode: tc.componentMode}}
			assert.Equal(t, tc.want, cfg.EffectiveRedisMode())
		})
	}
}

func TestEffectiveTemporalMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "Default config uses memory",
			cfg:  &Config{},
			want: ModeMemory,
		},
		{
			name: "Distributed maps to remote",
			cfg: &Config{
				Mode: ModeDistributed,
			},
			want: ModeRemoteTemporal,
		},
		{
			name: "Persistent inherits embedded",
			cfg: &Config{
				Mode: ModePersistent,
			},
			want: ModePersistent,
		},
		{
			name: "Memory inherits embedded",
			cfg: &Config{
				Mode: ModeMemory,
			},
			want: ModeMemory,
		},
		{
			name: "Component override to memory",
			cfg: &Config{
				Mode:     ModeDistributed,
				Temporal: TemporalConfig{Mode: ModeMemory},
			},
			want: ModeMemory,
		},
		{
			name: "Component override to distributed",
			cfg: &Config{
				Mode:     ModePersistent,
				Temporal: TemporalConfig{Mode: ModeDistributed},
			},
			want: ModeRemoteTemporal,
		},
		{
			name: "Component override to persistent",
			cfg: &Config{
				Mode:     ModeDistributed,
				Temporal: TemporalConfig{Mode: ModePersistent},
			},
			want: ModePersistent,
		},
		{
			name: "Component override to remote",
			cfg: &Config{
				Mode:     ModeMemory,
				Temporal: TemporalConfig{Mode: ModeRemoteTemporal},
			},
			want: ModeRemoteTemporal,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.cfg.EffectiveTemporalMode())
		})
	}
}

func TestEffectiveMCPProxyMode_Resolution(t *testing.T) {
	t.Parallel()
	t.Run("inherit global when component empty", func(t *testing.T) {
		cfg := &Config{Mode: ModeMemory}
		assert.Equal(t, ModeMemory, cfg.EffectiveMCPProxyMode())
	})

	t.Run("prefer component over global", func(t *testing.T) {
		cfg := &Config{Mode: ModeMemory, MCPProxy: MCPProxyConfig{Mode: ""}}
		assert.Equal(t, ModeMemory, cfg.EffectiveMCPProxyMode())
		cfg.MCPProxy.Mode = ModeDistributed
		assert.Equal(t, ModeDistributed, cfg.EffectiveMCPProxyMode())
	})
}

func TestEffectiveDatabaseDriver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		cfg        *Config
		wantDriver string
	}{
		{
			name:       "Nil config defaults to sqlite",
			cfg:        nil,
			wantDriver: databaseDriverSQLite,
		},
		{
			name:       "Memory mode defaults to sqlite",
			cfg:        &Config{Mode: ModeMemory},
			wantDriver: databaseDriverSQLite,
		},
		{
			name:       "Persistent mode defaults to sqlite",
			cfg:        &Config{Mode: ModePersistent},
			wantDriver: databaseDriverSQLite,
		},
		{
			name:       "Distributed mode defaults to postgres",
			cfg:        &Config{Mode: ModeDistributed},
			wantDriver: databaseDriverPostgres,
		},
		{
			name:       "Explicit postgres override respected",
			cfg:        &Config{Mode: ModeMemory, Database: DatabaseConfig{Driver: "postgres"}},
			wantDriver: databaseDriverPostgres,
		},
		{
			name:       "Explicit sqlite override respected",
			cfg:        &Config{Database: DatabaseConfig{Driver: "sqlite"}},
			wantDriver: databaseDriverSQLite,
		},
		{
			name:       "Empty mode defaults to sqlite",
			cfg:        &Config{Mode: ""},
			wantDriver: databaseDriverSQLite,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.cfg == nil {
				var cfg *Config
				assert.Equal(t, tc.wantDriver, cfg.EffectiveDatabaseDriver())
				return
			}
			assert.Equal(t, tc.wantDriver, tc.cfg.EffectiveDatabaseDriver())
		})
	}
}
