package config

import (
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
}

func TestModeValidation(t *testing.T) {
	t.Run("Global mode validation", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.Mode = "standalone"
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected valid global mode, got: %v", err)
		}
		cfg.Mode = "distributed"
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected valid global mode, got: %v", err)
		}
		cfg.Mode = "invalid"
		if err := svc.Validate(cfg); err == nil {
			t.Fatalf("expected validation error for invalid global mode")
		}
	})

	t.Run("Component mode validation and inheritance", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		// Empty is allowed (inheritance)
		cfg.Redis.Mode = ""
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected nil error for empty redis.mode, got: %v", err)
		}
		// Allowed values
		cfg.Redis.Mode = "standalone"
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected valid redis.mode, got: %v", err)
		}
		cfg.Redis.Mode = "distributed"
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected valid redis.mode, got: %v", err)
		}
		// Invalid value
		cfg.Redis.Mode = "invalid"
		if err := svc.Validate(cfg); err == nil {
			t.Fatalf("expected validation error for invalid redis.mode")
		}
	})

	t.Run("Redis persistence configuration baseline", func(t *testing.T) {
		svc := NewService()
		cfg := Default()
		cfg.Redis.Mode = "standalone"
		cfg.Redis.Standalone.Persistence.Enabled = true
		cfg.Redis.Standalone.Persistence.SnapshotInterval = 0
		if err := svc.Validate(cfg); err != nil {
			t.Fatalf("expected persistence settings to validate, got: %v", err)
		}
	})
}
