package modelcatalog

import (
	"math"
	"testing"
)

// Suite: merged catalog cost estimation
// Invariant: silent usage is priced exactly once with truthful rate provenance.
// Boundary IN: pure usage and merged-model pricing.
// Boundary OUT: observer precedence and durable token-stat aggregation.
func TestEstimateCost(t *testing.T) {
	t.Parallel()

	t.Run("Should price all five buckets with independent compatible rates", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		outputRate := 2.0
		cacheReadRate := 0.5
		cacheWriteRate := 3.0
		reasoningRate := 4.0
		input := int64(1_000_000)
		output := int64(1_000_000)
		cacheRead := int64(1_000_000)
		cacheWrite := int64(1_000_000)
		reasoning := int64(1_000_000)

		result, ok := EstimateCost(&Model{
			CostInputPerMillion:      &inputRate,
			CostOutputPerMillion:     &outputRate,
			CostCacheReadPerMillion:  &cacheReadRate,
			CostCacheWritePerMillion: &cacheWriteRate,
			CostReasoningPerMillion:  &reasoningRate,
			CostInputSource:          SourceKindConfig,
			CostOutputSource:         SourceKindConfig,
			CostCacheReadSource:      SourceKindConfig,
			CostCacheWriteSource:     SourceKindConfig,
			CostReasoningSource:      SourceKindConfig,
		}, TokenBuckets{
			Input:      &input,
			Output:     &output,
			CacheRead:  &cacheRead,
			CacheWrite: &cacheWrite,
			Reasoning:  &reasoning,
		})
		if !ok {
			t.Fatal("EstimateCost() ok = false, want true")
		}
		if math.Abs(result.Amount-10.5) > 1e-9 {
			t.Fatalf("EstimateCost().Amount = %.12f, want 10.5", result.Amount)
		}
		if result.Currency != "USD" || result.Status != CostStatusEstimated ||
			result.Source != CostSourceCatalogConfig {
			t.Fatalf("EstimateCost() = %#v, want USD estimated catalog_config", result)
		}
	})

	t.Run("Should require an explicit reasoning rate for nonzero reasoning usage", func(t *testing.T) {
		t.Parallel()

		outputRate := 4.0
		reasoning := int64(1)
		_, ok := EstimateCost(&Model{
			CostOutputPerMillion: &outputRate,
			CostOutputSource:     SourceKindConfig,
		}, TokenBuckets{Reasoning: &reasoning})
		if ok {
			t.Fatal("EstimateCost() ok = true, want false without a reasoning-specific rate")
		}
	})

	t.Run("Should return unknown when a used bucket has no rate", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		output := int64(1)
		_, ok := EstimateCost(&Model{
			CostInputPerMillion: &inputRate,
			CostInputSource:     SourceKindConfig,
		}, TokenBuckets{Output: &output})
		if ok {
			t.Fatal("EstimateCost() ok = true, want false")
		}
	})

	t.Run("Should return zero cost for zero usage when a rate row exists", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		zero := int64(0)
		result, ok := EstimateCost(&Model{
			CostInputPerMillion: &inputRate,
			CostInputSource:     SourceKindBuiltin,
		}, TokenBuckets{Input: &zero})
		if !ok {
			t.Fatal("EstimateCost() ok = false, want true")
		}
		if result.Amount != 0 || result.Source != CostSourceBuiltin {
			t.Fatalf("EstimateCost() = %#v, want zero builtin estimate", result)
		}
	})

	t.Run("Should reject incompatible rate provenance", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		outputRate := 2.0
		input := int64(1)
		output := int64(1)
		_, ok := EstimateCost(&Model{
			CostInputPerMillion:  &inputRate,
			CostOutputPerMillion: &outputRate,
			CostInputSource:      SourceKindConfig,
			CostOutputSource:     SourceKindModelsDev,
		}, TokenBuckets{Input: &input, Output: &output})
		if ok {
			t.Fatal("EstimateCost() ok = true, want false")
		}
	})

	t.Run("Should ignore unused input rate provenance for output-only usage", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		outputRate := 2.0
		output := int64(1_000_000)
		result, ok := EstimateCost(&Model{
			CostInputPerMillion:  &inputRate,
			CostOutputPerMillion: &outputRate,
			CostInputSource:      SourceKindConfig,
			CostOutputSource:     SourceKindModelsDev,
		}, TokenBuckets{Output: &output})
		if !ok {
			t.Fatal("EstimateCost() ok = false, want true")
		}
		if result.Amount != 2 || result.Source != CostSourceModelsDev {
			t.Fatalf("EstimateCost() = %#v, want output-only models_dev estimate of 2", result)
		}
	})

	t.Run("Should ignore unused output rate provenance for input-only usage", func(t *testing.T) {
		t.Parallel()

		inputRate := 1.0
		outputRate := 2.0
		input := int64(1_000_000)
		result, ok := EstimateCost(&Model{
			CostInputPerMillion:  &inputRate,
			CostOutputPerMillion: &outputRate,
			CostInputSource:      SourceKindBuiltin,
			CostOutputSource:     SourceKindConfig,
		}, TokenBuckets{Input: &input})
		if !ok {
			t.Fatal("EstimateCost() ok = false, want true")
		}
		if result.Amount != 1 || result.Source != CostSourceBuiltin {
			t.Fatalf("EstimateCost() = %#v, want input-only builtin estimate of 1", result)
		}
	})

	t.Run("Should reject a non-finite computed amount", func(t *testing.T) {
		t.Parallel()

		rate := math.MaxFloat64
		tokens := int64(math.MaxInt64)
		_, ok := EstimateCost(&Model{
			CostInputPerMillion: &rate,
			CostInputSource:     SourceKindConfig,
		}, TokenBuckets{Input: &tokens})
		if ok {
			t.Fatal("EstimateCost() ok = true, want false for overflow")
		}
	})
}
