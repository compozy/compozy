package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

// BenchmarkProfile describes the scale of a benchmark scenario for SDK builders.
type BenchmarkProfile struct {
	Name      string
	Agents    int
	Tasks     int
	Workflows int
	Memories  int
	Knowledge int
}

// Predefined benchmark profiles used across builder benchmarks.
var (
	BenchmarkSimple  = BenchmarkProfile{Name: "simple", Agents: 1, Tasks: 1, Workflows: 1, Memories: 1, Knowledge: 1}
	BenchmarkMedium  = BenchmarkProfile{Name: "medium", Agents: 4, Tasks: 8, Workflows: 4, Memories: 2, Knowledge: 3}
	BenchmarkComplex = BenchmarkProfile{Name: "complex", Agents: 10, Tasks: 50, Workflows: 6, Memories: 4, Knowledge: 6}
)

var benchmarkSink atomic.Value

// RunBuilderBenchmark executes fn using b.N iterations while reporting allocations and capturing failures.
func RunBuilderBenchmark(b *testing.B, fn func(ctx context.Context) (any, error)) {
	b.Helper()
	ctx := NewBenchmarkContext(b)
	b.ReportAllocs()
	b.ResetTimer()
	var last any
	for i := 0; i < b.N; i++ {
		result, err := fn(ctx)
		if err != nil {
			b.Fatalf("build failed: %v", err)
		}
		if result != nil {
			last = result
		}
	}
	if last != nil {
		benchmarkSink.Store(last)
	}
}

// RunParallelBuilderBenchmark executes fn concurrently using RunParallel to surface contention issues.
func RunParallelBuilderBenchmark(b *testing.B, fn func(ctx context.Context) (any, error)) {
	b.Helper()
	ctx := NewBenchmarkContext(b)
	b.ReportAllocs()
	b.ResetTimer()
	var last atomic.Value
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := fn(ctx)
			if err != nil {
				b.Fatalf("build failed: %v", err)
			}
			if result != nil {
				last.Store(result)
			}
		}
	})
	if value := last.Load(); value != nil {
		benchmarkSink.Store(value)
	}
}

// BenchmarkID produces deterministic benchmark identifiers.
func BenchmarkID(prefix string, idx int) string {
	return fmt.Sprintf("%s-%03d", strings.TrimSpace(prefix), idx)
}
