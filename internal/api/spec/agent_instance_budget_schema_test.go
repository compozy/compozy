package spec

import (
	"strings"
	"testing"
)

func TestAgentInstanceDailyBudgetUpdateSchema(t *testing.T) {
	t.Run("Should accept exactly one instance mutation with nonnegative optional caps", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatal(err)
		}
		schema := document.Components.Schemas["AgentInstanceUpdateRequest"].Value
		for _, tc := range []struct {
			name      string
			body      map[string]any
			wantError string
		}{
			{name: "checks only", body: map[string]any{"checks_enabled": false}},
			{name: "clear", body: map[string]any{"daily_budget": map[string]any{}}},
			{name: "zero", body: map[string]any{"daily_budget": map[string]any{"tokens": float64(0), "usd": nil}}},
			{name: "null counterpart", body: map[string]any{"checks_enabled": nil, "daily_budget": map[string]any{}}},
			{name: "missing", body: map[string]any{}, wantError: `property "daily_budget" is missing`},
			{name: "both", body: map[string]any{"checks_enabled": true, "daily_budget": map[string]any{}}, wantError: `value is not one of the allowed values [null]`},
			{name: "only null", body: map[string]any{"daily_budget": nil}, wantError: "Value is not nullable"},
			{name: "negative", body: map[string]any{"daily_budget": map[string]any{"usd": float64(-1)}}, wantError: "number must be at least 0"},
			{name: "typo", body: map[string]any{"daily_budget": map[string]any{"token": float64(1)}}, wantError: `property "token" is unsupported`},
			{name: "misplaced cap", body: map[string]any{"daily_budget": map[string]any{}, "usd": 1.25}, wantError: `property "usd" is unsupported`},
			{name: "legacy check extensions", body: map[string]any{"checks_enabled": false, "hint": "legacy"}},
		} {
			err := schema.VisitJSON(tc.body)
			if tc.wantError == "" {
				if err != nil {
					t.Errorf("%s error = %v", tc.name, err)
				}
				continue
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("%s error = %v, want %q", tc.name, err, tc.wantError)
			}
		}
	})
}
