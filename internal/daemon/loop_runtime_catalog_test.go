package daemon

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func TestLoopRuntimeCatalogShouldRespectProviderAuthority(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize provider aliases through config authority", func(t *testing.T) {
		t.Parallel()

		catalog := loopRuntimeCatalog{}
		if got := catalog.CanonicalProvider("Z-AI"); got != "zai" {
			t.Fatalf("CanonicalProvider(Z-AI) = %q, want zai", got)
		}
	})

	t.Run("Should reject providers absent from the effective config", func(t *testing.T) {
		t.Parallel()

		catalog := loopRuntimeCatalog{config: &compozyconfig.Config{}}
		err := catalog.ValidateRuntime(t.Context(), looppkg.RuntimeSpec{Provider: "flarp"})
		assertLoopRuntimeCatalogValidation(t, err, "provider", "flarp", "unknown_provider")
	})

	t.Run("Should accept exact model ids for known providers", func(t *testing.T) {
		t.Parallel()

		tests := []looppkg.RuntimeSpec{
			{Provider: "openrouter", Model: "anthropic/claude-opus-4-7"},
			{Provider: "cursor", Model: "composer-2.5"},
		}
		catalog := loopRuntimeCatalog{config: &compozyconfig.Config{}}
		for _, runtime := range tests {
			if err := catalog.ValidateRuntime(t.Context(), runtime); err != nil {
				t.Fatalf("ValidateRuntime(%#v) error = %v", runtime, err)
			}
		}
	})
}

func assertLoopRuntimeCatalogValidation(
	t *testing.T,
	err error,
	field string,
	value string,
	reason string,
) {
	t.Helper()
	validation, ok := looppkg.AsRuntimeValidationError(err)
	if !ok || len(validation.Items) != 1 {
		t.Fatalf("runtime validation error = %v, want one item", err)
	}
	item := validation.Items[0]
	if item.Field != field || item.Value != value || item.Reason != reason {
		t.Fatalf("runtime validation item = %#v, want %s=%q/%s", item, field, value, reason)
	}
}
