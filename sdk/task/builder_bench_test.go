package task

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/sdk/internal/testutil"
)

func BenchmarkTaskBuilders(b *testing.B) {
	cases := []struct {
		name  string
		build func(ctx context.Context) (any, error)
	}{
		{
			name: "basic",
			build: func(ctx context.Context) (any, error) {
				return NewBasic(
					"task-basic",
				).WithAgent("assistant").
					WithAction("reply").
					WithInput(map[string]string{"prompt": "{{ .workflow.input.prompt }}"}).
					WithOutput("summary={{ .task.output.summary }}").
					Build(ctx)
			},
		},
		{
			name: "aggregate",
			build: func(ctx context.Context) (any, error) {
				builder := NewAggregate("task-aggregate").WithStrategy("merge")
				for i := 0; i < 3; i++ {
					builder.AddTask(testutil.BenchmarkID("task", i))
				}
				return builder.Build(ctx)
			},
		},
		{
			name: "collection",
			build: func(ctx context.Context) (any, error) {
				return NewCollection(
					"task-collection",
				).WithCollection("{{ .input.items }}").
					WithTask("process-item").
					WithItemVar("item").
					Build(ctx)
			},
		},
		{
			name: "composite",
			build: func(ctx context.Context) (any, error) {
				return NewComposite(
					"task-composite",
				).WithWorkflow("child-workflow").
					WithInput(map[string]string{"source": "{{ .workflow.input }}"}).
					Build(ctx)
			},
		},
		{
			name: "memory",
			build: func(ctx context.Context) (any, error) {
				return NewMemoryTask(
					"task-memory",
				).WithOperation("append").
					WithMemory("session-store").
					WithContent("{{ .workflow.input.message }}").
					WithKeyTemplate("{{ .workflow.id }}:{{ .task.id }}").
					Build(ctx)
			},
		},
		{
			name: "parallel",
			build: func(ctx context.Context) (any, error) {
				builder := NewParallel("task-parallel").WithWaitAll(true)
				for i := 0; i < 4; i++ {
					builder.AddTask(testutil.BenchmarkID("child", i))
				}
				return builder.Build(ctx)
			},
		},
		{
			name: "router",
			build: func(ctx context.Context) (any, error) {
				builder := NewRouter("task-router").WithCondition("input.route")
				builder.AddRoute("sales", "task-sales")
				builder.AddRoute("support", "task-support")
				builder.WithDefault("task-default")
				return builder.Build(ctx)
			},
		},
		{
			name: "signal",
			build: func(ctx context.Context) (any, error) {
				payload := map[string]any{"customer": "{{ .workflow.input.customer }}", "amount": 42}
				return NewSignal(
					"task-signal",
				).Send("payment-signal", payload).
					OnSuccess("task-next").
					WithTimeout(15 * time.Second).
					Build(ctx)
			},
		},
		{
			name: "wait",
			build: func(ctx context.Context) (any, error) {
				return NewWait(
					"task-wait",
				).WithSignal("approval-signal").
					WithCondition("input.status == \"approved\"").
					WithTimeout(60 * time.Second).
					Build(ctx)
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(sb *testing.B) {
			sb.Helper()
			testutil.RunBuilderBenchmark(sb, tc.build)
		})
	}
	for _, tc := range cases {
		tc := tc
		b.Run(fmt.Sprintf("parallel/%s", tc.name), func(sb *testing.B) {
			sb.Helper()
			testutil.RunParallelBuilderBenchmark(sb, tc.build)
		})
	}
}
