package contract

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestRoleStatusJSONContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve inherit nulls and omit session role timeout", func(t *testing.T) {
		t.Parallel()

		encoded, err := json.Marshal(RoleStatus{
			Role:           "auto_title",
			Enabled:        true,
			ResolutionMode: RoleResolutionModeInherit,
			FallbackChain:  []RoleFallbackStatus{},
			Provenance:     map[string]string{"enabled": "default", "fallback_chain": "default"},
			Diagnostics:    []RoleDiagnostic{},
		})
		if err != nil {
			t.Fatalf("json.Marshal(RoleStatus) error = %v", err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatalf("json.Unmarshal(RoleStatus) error = %v", err)
		}
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		want := []string{
			"agent",
			"diagnostics",
			"enabled",
			"fallback_chain",
			"model",
			"provenance",
			"provider",
			"reasoning_effort",
			"resolution_mode",
			"role",
		}
		if !reflect.DeepEqual(keys, want) {
			t.Fatalf("RoleStatus JSON fields = %#v, want %#v", keys, want)
		}
		for _, field := range []string{"agent", "provider", "model", "reasoning_effort"} {
			if string(fields[field]) != "null" {
				t.Fatalf("RoleStatus.%s JSON = %s, want null", field, fields[field])
			}
		}
		if string(fields["fallback_chain"]) != "[]" || string(fields["diagnostics"]) != "[]" {
			t.Fatalf("RoleStatus arrays = %s/%s, want []/[]", fields["fallback_chain"], fields["diagnostics"])
		}
	})

	t.Run("Should include timeout for the in-process role", func(t *testing.T) {
		t.Parallel()

		timeout := "250ms"
		encoded, err := json.Marshal(RoleStatus{
			Role:           "memory_controller",
			Enabled:        true,
			ResolutionMode: RoleResolutionModeInherit,
			Timeout:        &timeout,
			FallbackChain:  []RoleFallbackStatus{},
			Provenance:     map[string]string{"timeout": "default"},
			Diagnostics:    []RoleDiagnostic{},
		})
		if err != nil {
			t.Fatalf("json.Marshal(RoleStatus) error = %v", err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatalf("json.Unmarshal(RoleStatus) error = %v", err)
		}
		if got := string(fields["timeout"]); got != `"250ms"` {
			t.Fatalf("RoleStatus.timeout JSON = %s, want %q", got, timeout)
		}
	})
}
