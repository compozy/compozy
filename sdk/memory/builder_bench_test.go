package memory

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/sdk/internal/testutil"
)

func BenchmarkMemoryBuilderSimple(b *testing.B) {
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		return New(
			"memory-simple",
		).WithTokenCounter("openai", "gpt-4o-mini").
			WithMaxTokens(4096).
			WithPrivacy(PrivacySessionScope).
			Build(ctx)
	})
}

func BenchmarkMemoryBuilderMedium(b *testing.B) {
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		return New(
			"memory-medium",
		).WithTokenCounter("openai", "gpt-4o").
			WithMaxTokens(8192).
			WithFIFOFlush(200).
			WithPrivacy(PrivacyUserScope).
			WithPersistence(PersistenceRedis).
			WithExpiration(72 * time.Hour).
			Build(ctx)
	})
}

func BenchmarkMemoryBuilderComplex(b *testing.B) {
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		return New(
			"memory-complex",
		).WithTokenCounter("openai", "gpt-4o").
			WithMaxTokens(16384).
			WithSummarizationFlush("openai", "gpt-4o", 2048).
			WithDistributedLocking(true).
			WithPrivacy(PrivacyGlobalScope).
			WithPersistence(PersistenceRedis).
			WithExpiration(168 * time.Hour).
			Build(ctx)
	})
}

func BenchmarkMemoryBuilderParallel(b *testing.B) {
	testutil.RunParallelBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		return New(
			"memory-parallel",
		).WithTokenCounter("anthropic", "claude-3-haiku").
			WithMaxTokens(4096).
			WithPrivacy(PrivacySessionScope).
			Build(ctx)
	})
}
