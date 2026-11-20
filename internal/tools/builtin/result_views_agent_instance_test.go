package builtin

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestAgentInstanceCompactDailyBudgetSchema(t *testing.T) {
	t.Run("Should advertise both optional spend fields and their nested shapes", func(t *testing.T) {
		t.Parallel()
		compact := decodeRawNativeObjectSchema(t, "agent instance compact record",
			json.RawMessage(agentInstanceCompactRecordSchema))
		for _, field := range []string{"daily_budget", "daily_budget_usage"} {
			if slices.Contains(compact.Required, field) {
				t.Fatalf("optional budget field %s must not be required", field)
			}
			if _, ok := compact.Properties[field]; !ok {
				t.Fatalf("compact result schema does not declare %s", field)
			}
		}
		limits := decodeRawNativeObjectSchema(t, "daily budget", compact.Properties["daily_budget"])
		for _, field := range []string{"tokens", "usd"} {
			if _, ok := limits.Properties[field]; !ok || slices.Contains(limits.Required, field) {
				t.Fatalf("budget limit %s must be optional", field)
			}
		}
		usage := decodeRawNativeObjectSchema(t, "daily budget usage", compact.Properties["daily_budget_usage"])
		for _, field := range []string{"day", "tokens", "known_usd", "has_unpriced_usage"} {
			if _, ok := usage.Properties[field]; !ok || !slices.Contains(usage.Required, field) {
				t.Fatalf("budget usage %s must be present and required", field)
			}
		}
	})
}
