## markdown

## status: completed # Options: pending, in-progress, completed, excluded

<task_context>
<domain>sdk/*</domain>
<type>testing</type>
<scope>benchmarks</scope>
<complexity>low</complexity>
<dependencies>task_56,task_57</dependencies>
</task_context>

# Task 59.0: Benchmarks: Build(ctx) (S)

## Overview

Create benchmark tests for all SDK builders measuring Build(ctx) performance, memory allocation, and identifying optimization opportunities.

<critical>
- **ALWAYS READ** tasks/prd-sdk/07-testing-strategy.md (performance section)
- **MUST** use b.Context() for benchmark contexts (Go 1.25+)
- **TARGET:** Build(ctx) < 100ms (p99), memory < 10MB for typical workflow
- **MUST** run with -benchmem for memory profiling
</critical>

<requirements>
- Benchmark all major builders (project, workflow, agent, tasks)
- Measure Build(ctx) performance across complexity levels
- Track memory allocations
- Identify performance bottlenecks
- Create comparison baselines
- Document performance characteristics
</requirements>

## Subtasks

- [x] 59.1 Create benchmark infrastructure and helpers
- [x] 59.2 Benchmarks: Project builder (simple, medium, complex)
- [x] 59.3 Benchmarks: Workflow builder (1-50 tasks)
- [x] 59.4 Benchmarks: Agent builder with actions
- [x] 59.5 Benchmarks: Task builders (all 9 types)
- [x] 59.6 Benchmarks: Knowledge builders (base with sources)
- [x] 59.7 Benchmarks: Memory builder
- [x] 59.8 Benchmarks: Complete project builds
- [x] 59.9 Create performance regression detection
- [x] 59.10 Document benchmark results and baselines

## Implementation Details

**Based on:** tasks/prd-sdk/07-testing-strategy.md (performance testing)

### Benchmark Structure

```go
// sdk/workflow/builder_bench_test.go
package workflow

import (
    "testing"

    "github.com/compozy/compozy/sdk/internal/testutil"
)

func BenchmarkWorkflowBuilder_Simple(b *testing.B) {
    ctx := testutil.NewTestContext(b)  // Uses b.Context() in Go 1.25+
    agent := testutil.NewTestAgent("assistant")
    task := testutil.NewTestTask("t1")

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _, _ = New("bench-workflow").
            AddAgent(agent).
            AddTask(task).
            Build(ctx)
    }
}

func BenchmarkWorkflowBuilder_Complex(b *testing.B) {
    ctx := testutil.NewTestContext(b)

    // Build complex workflow: 10 agents, 50 tasks
    agents := make([]*agent.Config, 10)
    tasks := make([]*task.Config, 50)
    for i := 0; i < 10; i++ {
        agents[i] = testutil.NewTestAgent(fmt.Sprintf("agent-%d", i))
    }
    for i := 0; i < 50; i++ {
        tasks[i] = testutil.NewTestTask(fmt.Sprintf("task-%d", i))
    }

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        builder := New("complex-workflow")
        for _, a := range agents {
            builder.AddAgent(a)
        }
        for _, t := range tasks {
            builder.AddTask(t)
        }
        _, _ = builder.Build(ctx)
    }
}

func BenchmarkWorkflowBuilder_Parallel(b *testing.B) {
    ctx := testutil.NewTestContext(b)
    agent := testutil.NewTestAgent("assistant")
    task := testutil.NewTestTask("t1")

    b.ResetTimer()
    b.ReportAllocs()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, _ = New("bench-workflow").
                AddAgent(agent).
                AddTask(task).
                Build(ctx)
        }
    })
}
```

### Benchmark Categories

1. **Simple Benchmarks** - Minimal valid configs
2. **Medium Benchmarks** - Typical production configs
3. **Complex Benchmarks** - Large-scale configs
4. **Parallel Benchmarks** - Concurrent Build(ctx) calls

### Performance Targets

```yaml
targets:
  simple_workflow:
    time: < 1ms
    allocs: < 100
    memory: < 1MB

  medium_workflow:
    time: < 10ms
    allocs: < 1000
    memory: < 5MB

  complex_workflow:
    time: < 100ms
    allocs: < 10000
    memory: < 10MB

  complete_project:
    time: < 100ms
    allocs: < 5000
    memory: < 10MB
```

### Relevant Files

- All builder packages: sdk/*/builder_bench_test.go
- Benchmark helpers: sdk/internal/testutil/bench.go
- Performance targets: tasks/prd-sdk/07-testing-strategy.md

### Dependent Files

- Task 56.0 deliverable (testutil for benchmark helpers)
- Task 57.0 deliverable (builders to benchmark)

## Deliverables

- Benchmark files for all major builders:
  - `sdk/project/builder_bench_test.go`
  - `sdk/workflow/builder_bench_test.go`
  - `sdk/agent/builder_bench_test.go`
  - `sdk/task/builder_bench_test.go` (all types)
  - `sdk/knowledge/builder_bench_test.go`
  - `sdk/memory/builder_bench_test.go`
  - `sdk/compozy/builder_bench_test.go` (complete project)
- Benchmark helpers in `sdk/internal/testutil/bench.go`
- Performance baseline documentation in `sdk/docs/performance-benchmarks.md`
- CI integration for regression detection

## Tests

Benchmark execution:
- [x] Run `go test -bench=. -benchmem ./sdk/...`
- [x] All benchmarks complete without errors
- [x] Memory allocations are reasonable (< targets)
- [x] No memory leaks detected (run with -memprofile)
- [x] Performance meets or exceeds targets

Profiling:
- [x] CPU profile: `go test -bench=. -cpuprofile=cpu.prof ./sdk/...`
- [x] Memory profile: `go test -bench=. -memprofile=mem.prof ./sdk/...`
- [x] Analyze profiles with `go tool pprof`
- [x] Identify top 5 bottlenecks

Regression detection:
- [x] Baseline results documented
- [x] CI fails if performance regresses >20%
- [x] Benchmark comparison tool works

## Success Criteria

- All Build(ctx) operations complete in < 100ms (p99)
- Memory usage < 10MB for typical project
- No memory leaks detected in profiling
- Benchmarks establish performance baselines
- CI detects performance regressions automatically
- Performance documentation guides optimization efforts
- Parallel benchmarks show no lock contention
- Benchmark results reproducible across runs (< 10% variance)
