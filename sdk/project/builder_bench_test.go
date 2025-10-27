package project

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/sdk/internal/testutil"
	"github.com/compozy/compozy/sdk/schedule"
)

func BenchmarkProjectBuilderSimple(b *testing.B) {
	workflows, agents := buildProjectWorkflows(b, 1)
	models := buildProjectModels(1)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("project-simple").WithVersion("1.0.0").WithDescription("Simple benchmark project").WithAuthor("Bench Bot", "bench@example.com", "Compozy")
		for _, model := range models {
			builder.AddModel(model)
		}
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, wf := range workflows {
			builder.AddWorkflow(wf)
		}
		return builder.Build(ctx)
	})
}

func BenchmarkProjectBuilderMedium(b *testing.B) {
	workflows, agents := buildProjectWorkflows(b, 4)
	models := buildProjectModels(2)
	setupCtx := testutil.NewBenchmarkContext(b)
	schedules := buildProjectSchedules(b, setupCtx, workflows, 2)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("project-medium").WithVersion("1.2.0").WithDescription("Medium benchmark project").WithAuthor("Bench Bot", "bench@example.com", "Compozy")
		for _, model := range models {
			builder.AddModel(model)
		}
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, wf := range workflows {
			builder.AddWorkflow(wf)
		}
		for _, scheduleCfg := range schedules {
			builder.AddSchedule(scheduleCfg)
		}
		return builder.Build(ctx)
	})
}

func BenchmarkProjectBuilderComplex(b *testing.B) {
	workflows, agents := buildProjectWorkflows(b, 6)
	models := buildProjectModels(3)
	setupCtx := testutil.NewBenchmarkContext(b)
	schedules := buildProjectSchedules(b, setupCtx, workflows, 4)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("project-complex").WithVersion("2.0.0").WithDescription("Complex benchmark project").WithAuthor("Bench Bot", "bench@example.com", "Compozy")
		for _, model := range models {
			builder.AddModel(model)
		}
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, wf := range workflows {
			builder.AddWorkflow(wf)
		}
		for _, scheduleCfg := range schedules {
			builder.AddSchedule(scheduleCfg)
		}
		return builder.Build(ctx)
	})
}

func BenchmarkProjectBuilderParallel(b *testing.B) {
	workflows, agents := buildProjectWorkflows(b, 2)
	models := buildProjectModels(1)
	testutil.RunParallelBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("project-parallel").WithVersion("1.1.0").WithDescription("Parallel benchmark project").WithAuthor("Bench Bot", "bench@example.com", "Compozy")
		for _, model := range models {
			builder.AddModel(model)
		}
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, wf := range workflows {
			builder.AddWorkflow(wf)
		}
		return builder.Build(ctx)
	})
}

func buildProjectModels(count int) []*core.ProviderConfig {
	models := make([]*core.ProviderConfig, count)
	for i := 0; i < count; i++ {
		models[i] = testutil.NewTestModel("openai", fmt.Sprintf("gpt-4o-mini-%d", i))
	}
	return models
}

func buildProjectWorkflows(b *testing.B, count int) ([]*engineworkflow.Config, []*agent.Config) {
	b.Helper()
	workflows := make([]*engineworkflow.Config, count)
	agents := make([]*agent.Config, count)
	for i := 0; i < count; i++ {
		wf := testutil.NewTestWorkflow(testutil.BenchmarkID("workflow", i))
		workflows[i] = wf
		agentCfg := wf.Agents[0]
		agents[i] = &agentCfg
	}
	return workflows, agents
}

func buildProjectSchedules(b *testing.B, ctx context.Context, workflows []*engineworkflow.Config, count int) []*engineschedule.Config {
	b.Helper()
	schedules := make([]*engineschedule.Config, count)
	for i := 0; i < count; i++ {
		workflowID := workflows[i%len(workflows)].ID
		builder := schedule.New(testutil.BenchmarkID("schedule", i)).WithWorkflow(workflowID).WithCron("0 * * * *").WithTimezone("UTC").WithDescription("hourly run")
		if i%2 == 0 {
			builder = builder.WithRetry(3, time.Minute)
		}
		cfg, err := builder.Build(ctx)
		if err != nil {
			b.Fatalf("failed to build schedule fixture %d: %v", i, err)
		}
		schedules[i] = cfg
	}
	return schedules
}
