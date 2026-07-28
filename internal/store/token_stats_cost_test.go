package store

import (
	"math"
	"strings"
	"testing"
)

// Suite: token-stat cost rollup
// Invariant: public aggregates sum only compatible status, source, and currency rows.
// Boundary IN: durable per-agent token-stat rows.
// Boundary OUT: session/task HTTP, UDS, CLI, and web projections.
func TestAggregateTokenStatsCost(t *testing.T) {
	t.Parallel()

	t.Run("Should sum compatible estimated rows", func(t *testing.T) {
		t.Parallel()

		currency := "USD"
		first := 0.25
		second := 0.75
		result := AggregateTokenStatsCost([]TokenStats{
			{TotalCost: &first, CostCurrency: &currency, CostStatus: "estimated", CostSource: "catalog_config"},
			{TotalCost: &second, CostCurrency: &currency, CostStatus: "estimated", CostSource: "catalog_config"},
		})

		if result.TotalCost == nil || *result.TotalCost != 1 ||
			result.Currency == nil || *result.Currency != "USD" ||
			result.Status != "estimated" || result.Source != "catalog_config" {
			t.Fatalf("AggregateTokenStatsCost() = %#v, want estimated catalog_config USD 1", result)
		}
	})

	t.Run("Should fail closed when provenance conflicts while preserving no amount", func(t *testing.T) {
		t.Parallel()

		currency := "USD"
		actual := 0.25
		estimated := 0.75
		result := AggregateTokenStatsCost([]TokenStats{
			{TotalCost: &actual, CostCurrency: &currency, CostStatus: "actual", CostSource: "agent_reported"},
			{TotalCost: &estimated, CostCurrency: &currency, CostStatus: "estimated", CostSource: "catalog_config"},
		})

		if result.TotalCost != nil || result.Currency != nil ||
			result.Status != "unknown" || result.Source != "none" {
			t.Fatalf("AggregateTokenStatsCost() = %#v, want unknown/none without amount", result)
		}
	})

	t.Run("Should preserve included classification without fabricating currency", func(t *testing.T) {
		t.Parallel()

		result := AggregateTokenStatsCost([]TokenStats{
			{CostStatus: "included", CostSource: "none"},
			{CostStatus: "included", CostSource: "none"},
		})

		if result.TotalCost != nil || result.Currency != nil ||
			result.Status != "included" || result.Source != "none" {
			t.Fatalf("AggregateTokenStatsCost() = %#v, want included/none without amount", result)
		}
	})

	t.Run("Should fail closed when compatible finite rows overflow their sum", func(t *testing.T) {
		t.Parallel()

		currency := "USD"
		first := math.MaxFloat64
		second := math.MaxFloat64
		result := AggregateTokenStatsCost([]TokenStats{
			{TotalCost: &first, CostCurrency: &currency, CostStatus: "actual", CostSource: "agent_reported"},
			{TotalCost: &second, CostCurrency: &currency, CostStatus: "actual", CostSource: "agent_reported"},
		})

		if result.TotalCost != nil || result.Currency != nil ||
			result.Status != "unknown" || result.Source != "none" {
			t.Fatalf("AggregateTokenStatsCost() = %#v, want overflow to fail closed", result)
		}
	})
}

func TestTokenStatsUpdateValidatesCostShape(t *testing.T) {
	t.Parallel()

	amount := 1.0
	negativeAmount := -1.0
	currency := "USD"
	testCases := []struct {
		name   string
		update TokenStatsUpdate
		want   string
	}{
		{
			name: "Should reject actual cost without an agent reported amount",
			update: TokenStatsUpdate{
				SessionID: "sess-1", AgentName: "coder", CostStatus: "actual", CostSource: "agent_reported",
			},
			want: "requires agent_reported amount",
		},
		{
			name: "Should reject actual cost without currency",
			update: TokenStatsUpdate{
				SessionID: "sess-1", AgentName: "coder", CostAmount: &amount,
				CostStatus: "actual", CostSource: "agent_reported",
			},
			want: "amount and currency",
		},
		{
			name: "Should reject a negative monetary amount",
			update: TokenStatsUpdate{
				SessionID: "sess-1", AgentName: "coder", CostAmount: &negativeAmount, CostCurrency: &currency,
				CostStatus: "estimated", CostSource: "models_dev",
			},
			want: "finite and non-negative",
		},
		{
			name: "Should reject estimated cost without a catalog source",
			update: TokenStatsUpdate{
				SessionID: "sess-1", AgentName: "coder", CostAmount: &amount, CostCurrency: &currency,
				CostStatus: "estimated", CostSource: "agent_reported",
			},
			want: "requires a catalog source",
		},
		{
			name: "Should reject included cost with a monetary amount",
			update: TokenStatsUpdate{
				SessionID: "sess-1", AgentName: "coder", CostAmount: &amount,
				CostStatus: "included", CostSource: "none",
			},
			want: "cannot carry amount or currency",
		},
		{
			name: "Should accept estimated cost with complete catalog provenance",
			update: TokenStatsUpdate{
				SessionID: "sess-1", AgentName: "coder", CostAmount: &amount, CostCurrency: &currency,
				CostStatus: "estimated", CostSource: "models_dev",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.update.Validate()
			if testCase.want == "" && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
			if testCase.want != "" && (err == nil || !strings.Contains(err.Error(), testCase.want)) {
				t.Fatalf("Validate() error = %v, want containing %q", err, testCase.want)
			}
		})
	}
}
