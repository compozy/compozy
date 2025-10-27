package compozy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	engineproject "github.com/compozy/compozy/engine/project"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/sdk/internal/testutil"
	projectsdk "github.com/compozy/compozy/sdk/project"
)

func BenchmarkCompozyBuilderSimple(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	projectCfg, workflows := buildBenchmarkProject(b, setupCtx, 1)
	cwd := b.TempDir()
	createEmptyConfigFile(b, cwd)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New(projectCfg).
			WithWorkflows(workflows...).
			WithDatabase("postgres://bench@localhost:5432/compozy?sslmode=disable").
			WithTemporal("localhost:7233", "default").
			WithRedis("redis://localhost:6379").
			WithWorkingDirectory(cwd)
		return builder.Build(ctx)
	})
}

func BenchmarkCompozyBuilderMedium(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	projectCfg, workflows := buildBenchmarkProject(b, setupCtx, 3)
	cwd := b.TempDir()
	configFile := createEmptyConfigFile(b, cwd)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New(projectCfg).
			WithWorkflows(workflows...).
			WithDatabase("postgres://bench@localhost:5432/compozy?sslmode=disable").
			WithTemporal("localhost:7233", "bench-namespace").
			WithRedis("redis://localhost:6379/1").
			WithWorkingDirectory(cwd).
			WithConfigFile(configFile).
			WithServerHost("127.0.0.1").
			WithServerPort(8080)
		return builder.Build(ctx)
	})
}

func BenchmarkCompozyBuilderComplex(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	projectCfg, workflows := buildBenchmarkProject(b, setupCtx, 5)
	cwd := b.TempDir()
	configFile := createEmptyConfigFile(b, cwd)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New(projectCfg).
			WithWorkflows(workflows...).
			WithDatabase("postgres://bench@localhost:5432/compozy?sslmode=disable").
			WithTemporal("localhost:7233", "enterprise").
			WithRedis("redis://localhost:6380/2").
			WithWorkingDirectory(cwd).
			WithConfigFile(configFile).
			WithCORS(true, "https://app.example.com", "https://admin.example.com").
			WithAuth(true).
			WithLogLevel("debug").
			WithServerHost("0.0.0.0").
			WithServerPort(9090)
		return builder.Build(ctx)
	})
}

func BenchmarkCompozyBuilderParallel(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	projectCfg, workflows := buildBenchmarkProject(b, setupCtx, 2)
	cwd := b.TempDir()
	createEmptyConfigFile(b, cwd)
	testutil.RunParallelBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New(projectCfg).
			WithWorkflows(workflows...).
			WithDatabase("postgres://bench@localhost:5432/compozy?sslmode=disable").
			WithTemporal("localhost:7233", "parallel").
			WithRedis("redis://localhost:6379/3").
			WithWorkingDirectory(cwd)
		return builder.Build(ctx)
	})
}

func buildBenchmarkProject(
	b *testing.B,
	ctx context.Context,
	workflowCount int,
) (*engineproject.Config, []*engineworkflow.Config) {
	b.Helper()
	workflows := make([]*engineworkflow.Config, workflowCount)
	agents := make([]*agent.Config, 0, workflowCount)
	for i := 0; i < workflowCount; i++ {
		wf := testutil.NewTestWorkflow(testutil.BenchmarkID("workflow", i))
		workflows[i] = wf
		for idx := range wf.Agents {
			agentCfg := wf.Agents[idx]
			agents = append(agents, &agentCfg)
		}
	}
	projectBuilder := projectsdk.New(testutil.BenchmarkID("project", workflowCount+1)).
		WithVersion("1.0.0").
		WithDescription("Benchmark project").
		WithAuthor("Bench Bot", "bench@example.com", "Compozy")
	projectBuilder.AddModel(testutil.NewTestModel("openai", fmt.Sprintf("gpt-4o-mini-%d", workflowCount)))
	for _, wf := range workflows {
		projectBuilder.AddWorkflow(wf)
	}
	for _, agentCfg := range agents {
		projectBuilder.AddAgent(agentCfg)
	}
	projectCfg, err := projectBuilder.Build(ctx)
	if err != nil {
		b.Fatalf("failed to build project config: %v", err)
	}
	return projectCfg, workflows
}

func createEmptyConfigFile(b *testing.B, dir string) string {
	b.Helper()
	path := filepath.Join(dir, "compozy.yaml")
	if err := os.WriteFile(path, []byte("project: bench"), 0o600); err != nil {
		b.Fatalf("failed to write config file: %v", err)
	}
	return path
}
