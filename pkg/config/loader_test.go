package config

import (
	"strings"
	"testing"
	"time"
)

// TestValidateNativeToolTimeouts exercises negative, zero, and positive values
// for each runtime.native_tools.*.default_timeout field.
func TestValidateNativeToolTimeouts(t *testing.T) {
	t.Run("negative values produce errors", func(t *testing.T) {
		base := &Config{}
		base.Runtime.NativeTools = DefaultNativeToolsConfig()

		cases := []struct {
			name   string
			mutate func(cfg *Config)
		}{
			{"call_agent", func(cfg *Config) { cfg.Runtime.NativeTools.CallAgent.DefaultTimeout = -1 * time.Second }},
			{"call_agents", func(cfg *Config) { cfg.Runtime.NativeTools.CallAgents.DefaultTimeout = -1 * time.Second }},
			{"call_task", func(cfg *Config) { cfg.Runtime.NativeTools.CallTask.DefaultTimeout = -1 * time.Second }},
			{"call_tasks", func(cfg *Config) { cfg.Runtime.NativeTools.CallTasks.DefaultTimeout = -1 * time.Second }},
			{
				"call_workflow",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallWorkflow.DefaultTimeout = -1 * time.Second },
			},
			{
				"call_workflows",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallWorkflows.DefaultTimeout = -1 * time.Second },
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := *base // shallow copy ok; we mutate leaf durations only
				// copy nested NativeTools to avoid cross-test mutation
				cfg.Runtime.NativeTools = base.Runtime.NativeTools
				tc.mutate(&cfg)
				if err := validateNativeToolTimeouts(&cfg); err == nil {
					t.Fatalf("expected error for negative %s.default_timeout, got nil", tc.name)
				}
			})
		}
	})

	t.Run("zero values are allowed", func(t *testing.T) {
		base := &Config{}
		base.Runtime.NativeTools = DefaultNativeToolsConfig()

		cases := []struct {
			name   string
			mutate func(cfg *Config)
		}{
			{"call_agent", func(cfg *Config) { cfg.Runtime.NativeTools.CallAgent.DefaultTimeout = 0 }},
			{"call_agents", func(cfg *Config) { cfg.Runtime.NativeTools.CallAgents.DefaultTimeout = 0 }},
			{"call_task", func(cfg *Config) { cfg.Runtime.NativeTools.CallTask.DefaultTimeout = 0 }},
			{"call_tasks", func(cfg *Config) { cfg.Runtime.NativeTools.CallTasks.DefaultTimeout = 0 }},
			{"call_workflow", func(cfg *Config) { cfg.Runtime.NativeTools.CallWorkflow.DefaultTimeout = 0 }},
			{"call_workflows", func(cfg *Config) { cfg.Runtime.NativeTools.CallWorkflows.DefaultTimeout = 0 }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := *base
				cfg.Runtime.NativeTools = base.Runtime.NativeTools
				tc.mutate(&cfg)
				if err := validateNativeToolTimeouts(&cfg); err != nil {
					t.Fatalf("expected nil error for zero %s.default_timeout, got: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("positive values are allowed", func(t *testing.T) {
		base := &Config{}
		base.Runtime.NativeTools = DefaultNativeToolsConfig()

		cases := []struct {
			name   string
			mutate func(cfg *Config)
		}{
			{
				"call_agent",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallAgent.DefaultTimeout = 123 * time.Millisecond },
			},
			{
				"call_agents",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallAgents.DefaultTimeout = 456 * time.Millisecond },
			},
			{
				"call_task",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallTask.DefaultTimeout = 789 * time.Millisecond },
			},
			{
				"call_tasks",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallTasks.DefaultTimeout = 101 * time.Millisecond },
			},
			{
				"call_workflow",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallWorkflow.DefaultTimeout = 202 * time.Millisecond },
			},
			{
				"call_workflows",
				func(cfg *Config) { cfg.Runtime.NativeTools.CallWorkflows.DefaultTimeout = 303 * time.Millisecond },
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := *base
				cfg.Runtime.NativeTools = base.Runtime.NativeTools
				tc.mutate(&cfg)
				if err := validateNativeToolTimeouts(&cfg); err != nil {
					t.Fatalf("expected nil error for positive %s.default_timeout, got: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("MCPProxy mode validation", func(t *testing.T) {
		cases := []struct {
			name           string
			mode           string
			global         string
			port           int
			wantErr        bool
			wantSubstrings []string
		}{
			{
				name:   "inherit from global memory",
				mode:   "",
				global: ModeMemory,
				port:   6201,
			},
			{
				name:   "memory explicit",
				mode:   ModeMemory,
				global: ModeDistributed,
				port:   6202,
			},
			{
				name:   "persistent explicit",
				mode:   ModePersistent,
				global: ModeDistributed,
				port:   6203,
			},
			{
				name:   "distributed explicit",
				mode:   ModeDistributed,
				global: ModeMemory,
				port:   0,
			},
			{
				name:           "standalone rejected",
				mode:           deprecatedModeStandalone,
				global:         ModeDistributed,
				port:           6204,
				wantErr:        true,
				wantSubstrings: []string{"no longer supported", ModeMemory, ModePersistent, ModeDistributed},
			},
			{
				name:           "invalid value rejected",
				mode:           "invalid",
				global:         ModeDistributed,
				port:           6205,
				wantErr:        true,
				wantSubstrings: []string{"must be one of", "invalid"},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Default()
				cfg.Mode = tc.global
				cfg.MCPProxy.Mode = tc.mode
				cfg.MCPProxy.Port = tc.port
				err := validateMCPProxy(cfg)
				if tc.wantErr {
					if err == nil {
						t.Fatalf("expected validation error for mcp_proxy.mode %q", tc.mode)
					}
					for _, sub := range tc.wantSubstrings {
						if !strings.Contains(err.Error(), sub) {
							t.Fatalf("expected error to contain %q, got: %v", sub, err)
						}
					}
					return
				}
				if err != nil {
					t.Fatalf("expected nil error for mcp_proxy.mode %q, got: %v", tc.mode, err)
				}
			})
		}
	})
}

func TestValidateMCPProxy_PortRequirement(t *testing.T) {
	t.Run("embedded modes require explicit port", func(t *testing.T) {
		cases := []struct {
			name string
			mode string
		}{
			{name: "memory", mode: ModeMemory},
			{name: "persistent", mode: ModePersistent},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				cfg := Default()
				cfg.Mode = ModeDistributed
				cfg.MCPProxy.Mode = tc.mode
				cfg.MCPProxy.Port = 0
				err := validateMCPProxy(cfg)
				if err == nil {
					t.Fatalf("expected error when mcp_proxy.port is zero for mode %q", tc.mode)
				}
				for _, sub := range []string{"mcp_proxy.port", ModeMemory, ModePersistent} {
					if !strings.Contains(err.Error(), sub) {
						t.Fatalf("expected error to contain %q, got: %v", sub, err)
					}
				}
			})
		}
	})
}

func TestModeValidation(t *testing.T) {
	t.Run("Global mode validation", func(t *testing.T) {
		cases := []struct {
			name           string
			mode           string
			wantErr        bool
			wantSubstrings []string
		}{
			{name: "empty inherits default", mode: ""},
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
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := NewService()
				cfg := Default()
				cfg.Mode = tc.mode
				err := svc.Validate(cfg)
				if tc.wantErr {
					if err == nil {
						t.Fatalf("expected validation error for global mode %q", tc.mode)
					}
					for _, sub := range tc.wantSubstrings {
						if !strings.Contains(err.Error(), sub) {
							t.Fatalf("expected error to contain %q, got: %v", sub, err)
						}
					}
					return
				}
				if err != nil {
					t.Fatalf("expected valid global mode %q, got: %v", tc.mode, err)
				}
			})
		}
	})

	t.Run("Component mode validation and inheritance", func(t *testing.T) {
		cases := []struct {
			name           string
			mode           string
			wantErr        bool
			wantSubstrings []string
		}{
			{name: "inherit from global", mode: ""},
			{name: "memory valid", mode: ModeMemory},
			{name: "persistent valid", mode: ModePersistent},
			{name: "distributed valid", mode: ModeDistributed},
			{
				name:           "standalone invalid",
				mode:           "standalone",
				wantErr:        true,
				wantSubstrings: []string{"standalone", "no longer supported", ModeMemory, ModePersistent},
			},
			{name: "invalid value", mode: "invalid", wantErr: true, wantSubstrings: []string{"must be one of"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := NewService()
				cfg := Default()
				cfg.Redis.Mode = tc.mode
				err := svc.Validate(cfg)
				if tc.wantErr {
					if err == nil {
						t.Fatalf("expected validation error for redis.mode %q", tc.mode)
					}
					for _, sub := range tc.wantSubstrings {
						if !strings.Contains(err.Error(), sub) {
							t.Fatalf("expected error to contain %q, got: %v", sub, err)
						}
					}
					return
				}
				if err != nil {
					t.Fatalf("expected valid redis.mode %q, got: %v", tc.mode, err)
				}
			})
		}
	})

	t.Run("Redis persistence configuration baseline", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.Redis.Mode = ModePersistent
		cfg.Redis.Standalone.Persistence.Enabled = true
		cfg.Redis.Standalone.Persistence.DataDir = "/tmp/compozy-test"
		cfg.Redis.Standalone.Persistence.SnapshotInterval = time.Minute
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected persistence settings to validate, got: %v", err)
		}
	})

	t.Run("Should provide helpful error for invalid snapshot interval", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.Redis.Mode = ModePersistent
		cfg.Redis.Standalone.Persistence.Enabled = true
		cfg.Redis.Standalone.Persistence.DataDir = "/tmp/dir"
		cfg.Redis.Standalone.Persistence.SnapshotInterval = 0
		if err := svc.Validate(cfg); err == nil {
			t.Fatalf("expected error for zero snapshot interval")
		}
	})

	t.Run("Should allow missing Redis address in distributed mode (server skips client)", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.Mode = ModeDistributed
		cfg.Redis.Mode = ModeDistributed
		cfg.Redis.URL = ""
		cfg.Redis.Host = ""
		cfg.Redis.Port = ""
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
	})
}
