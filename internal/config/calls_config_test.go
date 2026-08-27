package config

import (
	"testing"

	"github.com/compozy/compozy/internal/contracts"
)

func TestCallsConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the exact default call bounds", func(t *testing.T) {
		t.Parallel()

		config := DefaultCallsConfig()
		if config.MaxDepth != 3 || config.MaxBatch != 8 || config.MaxChildren != 5 ||
			config.MaxActivePerRoot != 32 || config.IdleTTL != "1h" || config.OperationTimeout != "30s" {
			t.Fatalf("DefaultCallsConfig() admission defaults = %#v", config)
		}
		if config.Results.DefaultBudget != "256KiB" || config.Results.MaxBudget != "4MiB" ||
			config.Results.Overflow != "store" {
			t.Fatalf("DefaultCallsConfig() results = %#v", config.Results)
		}
		if config.Messages.RateLimitPerMinute != 30 || config.Messages.DedupWindow != "30s" ||
			config.Messages.PendingCap != 50 || config.Messages.MaxBytes != "64KiB" {
			t.Fatalf("DefaultCallsConfig() messages = %#v", config.Messages)
		}
		if err := config.Validate(); err != nil {
			t.Fatalf("DefaultCallsConfig().Validate() error = %v", err)
		}
	})

	t.Run("Should reject invalid caps budgets and overflow with path context", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			mutate    func(*CallsConfig)
			wantError string
		}{
			{
				name: "Should reject a non-positive cap",
				mutate: func(config *CallsConfig) {
					config.MaxBatch = 0
				},
				wantError: "calls.max_batch must be positive: 0",
			},
			{
				name: "Should reject a non-positive operation timeout",
				mutate: func(config *CallsConfig) {
					config.OperationTimeout = "0s"
				},
				wantError: `calls.operation_timeout: must be a positive duration: "0s"`,
			},
			{
				name: "Should reject a default budget above max",
				mutate: func(config *CallsConfig) {
					config.Results.DefaultBudget = "5MiB"
				},
				wantError: `calls.results: default_budget "5MiB" exceeds max_budget "4MiB"`,
			},
			{
				name: "Should reject an unknown overflow mode",
				mutate: func(config *CallsConfig) {
					config.Results.Overflow = "truncate"
				},
				wantError: `calls.results.overflow must be "store" or "reject": "truncate"`,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				config := DefaultCallsConfig()
				test.mutate(&config)
				err := config.Validate()
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("CallsConfig.Validate() error = %v, want %q", err, test.wantError)
				}
			})
		}
	})

	t.Run("Should compose overlays per key without resetting sibling defaults", func(t *testing.T) {
		t.Parallel()

		config := DefaultCallsConfig()
		profileBatch := 4
		workspaceTTL := "2h"
		workspaceProfileOverflow := string(contracts.OverflowReject)
		overlays := []callsOverlay{
			{MaxBatch: &profileBatch},
			{IdleTTL: &workspaceTTL},
			{Results: callsResultsOverlay{Overflow: &workspaceProfileOverflow}},
		}
		for _, overlay := range overlays {
			overlay.Apply(&config)
		}
		if config.MaxBatch != 4 || config.IdleTTL != "2h" ||
			config.Results.Overflow != "reject" || config.MaxChildren != 5 {
			t.Fatalf("composed CallsConfig = %#v", config)
		}
	})
}

func TestCallsToolSurface(t *testing.T) {
	t.Parallel()

	expected := callsToolPathKinds()
	if len(expected) != 13 {
		t.Fatalf("calls tool paths = %d, want 13", len(expected))
	}
	for path, wantKind := range expected {
		t.Run("Should classify "+path, func(t *testing.T) {
			t.Parallel()
			segments, err := ParseDottedConfigPath(path)
			if err != nil {
				t.Fatalf("ParseDottedConfigPath(%q) error = %v", path, err)
			}
			policy, err := ClassifyToolConfigPath(segments)
			if err != nil {
				t.Fatalf("ClassifyToolConfigPath(%q) error = %v", path, err)
			}
			if policy.Denial != "" || policy.Kind != wantKind {
				t.Fatalf("ClassifyToolConfigPath(%q) = %#v, want kind %v", path, policy, wantKind)
			}
		})
	}

	t.Run("Should accept an integer call setting and deny an unknown path", func(t *testing.T) {
		t.Parallel()

		policy, err := ClassifyToolConfigPath([]string{"calls", "max_batch"})
		if err != nil {
			t.Fatalf("ClassifyToolConfigPath(max_batch) error = %v", err)
		}
		value, err := NormalizeToolConfigValue(policy.Kind, 4)
		if err != nil || value != 4 {
			t.Fatalf("NormalizeToolConfigValue(max_batch) = %#v, %v", value, err)
		}
		unknown, err := ClassifyToolConfigPath([]string{"calls", "unknown"})
		if err != nil {
			t.Fatalf("ClassifyToolConfigPath(unknown) error = %v", err)
		}
		if unknown.Denial != ConfigPathForbidden {
			t.Fatalf("unknown policy = %#v, want forbidden", unknown)
		}
	})
}

func TestParseByteSize(t *testing.T) {
	t.Parallel()

	t.Run("Should parse canonical binary sizes", func(t *testing.T) {
		t.Parallel()

		for raw, want := range map[string]int{"256KiB": 256 << 10, "4MiB": 4 << 20} {
			t.Run("Should parse "+raw, func(t *testing.T) {
				t.Parallel()
				got, err := ParseByteSize(raw)
				if err != nil || got != want {
					t.Fatalf("ParseByteSize(%q) = %d, %v; want %d", raw, got, err, want)
				}
			})
		}
	})

	t.Run("Should reject malformed sizes with an example", func(t *testing.T) {
		t.Parallel()

		_, err := ParseByteSize("256KB")
		want := `must be a positive byte size such as "256KiB": "256KB"`
		if err == nil || err.Error() != want {
			t.Fatalf("ParseByteSize(malformed) error = %v, want %q", err, want)
		}
	})

	t.Run("Should reject a negative byte size", func(t *testing.T) {
		t.Parallel()
		if _, err := ParseByteSize("-1KiB"); err == nil {
			t.Fatal("ParseByteSize(-1KiB) error = nil")
		}
	})
}
