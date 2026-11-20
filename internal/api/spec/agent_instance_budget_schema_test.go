package spec

import "testing"

func TestAgentInstanceDailyBudgetUpdateSchema(t *testing.T) {
	t.Run("Should accept exactly one instance mutation with nonnegative optional caps", func(t *testing.T) {
		t.Parallel()
		document, err := Document()
		if err != nil {
			t.Fatal(err)
		}
		schema := document.Components.Schemas["AgentInstanceUpdateRequest"].Value
		for _, tc := range []struct {
			name  string
			body  map[string]any
			valid bool
		}{
			{name: "checks only", body: map[string]any{"checks_enabled": false}, valid: true},
			{name: "clear", body: map[string]any{"daily_budget": map[string]any{}}, valid: true},
			{name: "zero", body: map[string]any{"daily_budget": map[string]any{"tokens": float64(0), "usd": nil}}, valid: true},
			{name: "null counterpart", body: map[string]any{"checks_enabled": nil, "daily_budget": map[string]any{}}, valid: true},
			{name: "missing", body: map[string]any{}},
			{name: "both", body: map[string]any{"checks_enabled": true, "daily_budget": map[string]any{}}},
			{name: "only null", body: map[string]any{"daily_budget": nil}},
			{name: "negative", body: map[string]any{"daily_budget": map[string]any{"usd": float64(-1)}}},
			{name: "typo", body: map[string]any{"daily_budget": map[string]any{"token": float64(1)}}},
			{name: "misplaced cap", body: map[string]any{"daily_budget": map[string]any{}, "usd": float64(1.25)}},
		} {
			if err := schema.VisitJSON(tc.body); (err == nil) != tc.valid {
				t.Errorf("%s error=%v, want valid=%t", tc.name, err, tc.valid)
			}
		}
	})
}
