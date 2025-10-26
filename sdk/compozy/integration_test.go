package compozy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	engineproject "github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	projectbuilder "github.com/compozy/compozy/sdk/project"
	workflowbuilder "github.com/compozy/compozy/sdk/workflow"
	"github.com/compozy/compozy/test/helpers"
)

func TestBuilderRegistersProjectAndWorkflow(t *testing.T) {
	t.Parallel()

	ctx, log := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)
	store := resources.NewMemoryResourceStore()

	instance, err := defaultBuilder(projectCfg, workflowCfg, store).
		Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, instance)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})

	resourceStore := instance.ResourceStore()
	require.NotNil(t, resourceStore)

	projectKey := resources.ResourceKey{
		Project: projectCfg.Name,
		Type:    resources.ResourceProject,
		ID:      projectCfg.Name,
	}
	value, _, err := resourceStore.Get(ctx, projectKey)
	require.NoError(t, err)
	require.IsType(t, &engineproject.Config{}, value)

	wfKey := resources.ResourceKey{
		Project: projectCfg.Name,
		Type:    resources.ResourceWorkflow,
		ID:      workflowCfg.ID,
	}
	value, _, err = resourceStore.Get(ctx, wfKey)
	require.NoError(t, err)
	require.IsType(t, &engineworkflow.Config{}, value)

	require.NotEmpty(t, log.entries)
}

func TestExecuteWorkflowReturnsOutputs(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)

	instance, err := defaultBuilder(projectCfg, workflowCfg, nil).Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})

	result, err := instance.ExecuteWorkflow(ctx, workflowCfg.ID, map[string]any{"user": "sam"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, workflowCfg.ID, result.WorkflowID)
	require.Equal(t, "hello", result.Output["message"])
}

func TestExecuteWorkflowUnknownID(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)

	instance, err := defaultBuilder(projectCfg, workflowCfg, nil).Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})

	_, err = instance.ExecuteWorkflow(ctx, "missing", nil)
	require.Error(t, err)
}

func TestLoadProjectIntoEngineValidationError(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	instance := &Compozy{store: resources.NewMemoryResourceStore()}

	err := instance.loadProjectIntoEngine(ctx, nil)
	require.Error(t, err)
}

func TestRegisterProjectValidationFailureIncludesName(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, _ := buildTestConfigs(t, ctx)
	projectCfg.Opts.SourceOfTruth = "invalid"
	instance := &Compozy{store: resources.NewMemoryResourceStore()}
	err := instance.RegisterProject(ctx, projectCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "project "+projectCfg.Name+" validation failed")
}

func TestRegisterWorkflowValidationFailureReturnsID(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)
	instance := &Compozy{store: resources.NewMemoryResourceStore()}
	require.NoError(t, instance.RegisterProject(ctx, projectCfg))
	if len(workflowCfg.Agents) > 0 {
		workflowCfg.Agents[0].ID = ""
	}
	err := instance.RegisterWorkflow(ctx, workflowCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "workflow "+workflowCfg.ID+" validation failed")
}

func TestRegisterWorkflowDuplicateIDRejected(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)
	instance := &Compozy{store: resources.NewMemoryResourceStore()}
	require.NoError(t, instance.RegisterProject(ctx, projectCfg))
	require.NoError(t, instance.RegisterWorkflow(ctx, workflowCfg))
	err := instance.RegisterWorkflow(ctx, workflowCfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func TestMultipleWorkflowsRegisterInOrder(t *testing.T) {
	t.Parallel()

	ctx, log := newTestContext(t)
	projectDir := t.TempDir()
	wfOne := buildWorkflowConfig(t, ctx, "alpha", projectDir)
	wfTwo := buildWorkflowConfig(t, ctx, "beta", projectDir)
	projectCfg, err := projectbuilder.New("demo-order").
		WithDescription("Order validation project").
		AddWorkflow(wfOne).
		AddWorkflow(wfTwo).
		Build(ctx)
	require.NoError(t, err)
	require.NoError(t, projectCfg.SetCWD(projectDir))
	store := resources.NewMemoryResourceStore()
	builder := New(projectCfg).
		WithWorkflows(wfOne, wfTwo).
		WithDatabase("postgres://user:pass@localhost:5432/compozy?sslmode=disable").
		WithTemporal("localhost:7233", "default").
		WithRedis("redis://localhost:6379/0").
		WithResourceStore(store).
		WithWorkingDirectory(projectDir)
	instance, err := builder.Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})
	workflowOrder := make([]string, 0, 2)
	for _, entry := range log.entries {
		if entry.msg != "workflow registered" {
			continue
		}
		for i := 0; i < len(entry.args); i += 2 {
			key, ok := entry.args[i].(string)
			if !ok || i+1 >= len(entry.args) {
				continue
			}
			if key == "workflow" {
				if val, ok := entry.args[i+1].(string); ok {
					workflowOrder = append(workflowOrder, val)
				}
				break
			}
		}
	}
	require.Equal(t, []string{wfOne.ID, wfTwo.ID}, workflowOrder)
}

func TestHybridProjectSupportsYAML(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)
	store := resources.NewMemoryResourceStore()
	instance, err := defaultBuilder(projectCfg, workflowCfg, store).
		Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})
	projectCWD := ""
	if cwd := projectCfg.GetCWD(); cwd != nil {
		projectCWD = cwd.PathStr()
	}
	require.NotEmpty(t, projectCWD)
	yamlWorkflow := buildWorkflowConfig(t, ctx, "yaml-flow", projectCWD)
	require.NoError(t, yamlWorkflow.IndexToResourceStore(ctx, projectCfg.Name, store))
	_, _, err = store.Get(
		ctx,
		resources.ResourceKey{Project: projectCfg.Name, Type: resources.ResourceWorkflow, ID: workflowCfg.ID},
	)
	require.NoError(t, err)
	_, _, err = store.Get(
		ctx,
		resources.ResourceKey{Project: projectCfg.Name, Type: resources.ResourceWorkflow, ID: yamlWorkflow.ID},
	)
	require.NoError(t, err)
}

func TestBuilderRequiresProject(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	_, workflowCfg := buildTestConfigs(t, ctx)

	_, err := New(nil).
		WithWorkflows(workflowCfg).
		WithDatabase("postgres://localhost/db").
		WithTemporal("localhost:7233", "default").
		WithRedis("redis://localhost:6379").
		Build(ctx)
	require.Error(t, err)
}

func TestBuilderRequiresWorkflows(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, _ := buildTestConfigs(t, ctx)
	_, err := New(projectCfg).
		WithDatabase("postgres://user:pass@localhost:5432/compozy?sslmode=disable").
		WithTemporal("localhost:7233", "default").
		WithRedis("redis://localhost:6379/0").
		Build(ctx)
	require.Error(t, err)
	buildErr, ok := err.(*sdkerrors.BuildError)
	require.True(t, ok)
	require.Contains(t, buildErr.Error(), "at least one workflow must be provided")
}

func TestBuilderAggregatesInfrastructureErrors(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)

	_, err := New(projectCfg).
		WithWorkflows(workflowCfg).
		Build(ctx)
	require.Error(t, err)
	buildErr, ok := err.(*sdkerrors.BuildError)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(buildErr.Errors), 3)
}

func TestLifecycleStartStopWait(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)

	instance, err := defaultBuilder(projectCfg, workflowCfg, nil).
		WithServerHost("127.0.0.1").
		WithServerPort(0).
		Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})

	require.NoError(t, instance.Start())

	stopCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	require.NoError(t, instance.Stop(stopCtx))
	require.NoError(t, instance.Wait())
}

func TestConfigAccessors(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContext(t)
	projectCfg, workflowCfg := buildTestConfigs(t, ctx)

	host := "127.0.0.1"
	instance, err := defaultBuilder(projectCfg, workflowCfg, nil).
		WithServerHost(host).
		WithServerPort(12345).
		WithLogLevel("debug").
		Build(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = instance.Stop(stopCtx)
	})

	require.NotNil(t, instance.Server())
	require.NotNil(t, instance.Config())
	require.Equal(t, host, instance.Config().Server.Host)
	require.Equal(t, 12345, instance.Config().Server.Port)
}

func buildTestConfigs(t *testing.T, ctx context.Context) (*engineproject.Config, *engineworkflow.Config) {
	t.Helper()

	agentCfg := &agent.Config{
		ID:           "assistant",
		Instructions: "Respond with a static greeting.",
		Model:        agent.Model{Ref: "test-model"},
	}
	taskCfg := &task.Config{
		BaseConfig: task.BaseConfig{
			ID:    "say-hello",
			Type:  task.TaskTypeBasic,
			Agent: agentCfg,
		},
		BasicTask: task.BasicTask{
			Prompt: "Say hello to the user.",
		},
	}
	wfBuilder := workflowbuilder.New("welcome").
		WithDescription("Sample workflow").
		AddAgent(agentCfg).
		AddTask(taskCfg).
		WithOutputs(map[string]string{"message": "hello"})
	workflowCfg, err := wfBuilder.Build(ctx)
	require.NoError(t, err)

	projectCfg, err := projectbuilder.New("demo-project").
		WithDescription("Demo project for SDK integration").
		AddWorkflow(workflowCfg).
		Build(ctx)
	require.NoError(t, err)
	projectDir := t.TempDir()
	require.NoError(t, projectCfg.SetCWD(projectDir))
	require.NoError(t, workflowCfg.SetCWD(projectDir))

	return projectCfg, workflowCfg
}

func buildWorkflowConfig(t *testing.T, ctx context.Context, id string, projectDir string) *engineworkflow.Config {
	t.Helper()

	agentCfg := &agent.Config{
		ID:           id + "-agent",
		Instructions: "Dynamic workflow agent.",
		Model:        agent.Model{Ref: "test-model"},
	}
	taskCfg := &task.Config{
		BaseConfig: task.BaseConfig{
			ID:    id + "-task",
			Type:  task.TaskTypeBasic,
			Agent: agentCfg,
		},
		BasicTask: task.BasicTask{Prompt: "Say " + id},
	}
	builder := workflowbuilder.New(id).
		WithDescription("Workflow " + id).
		AddAgent(agentCfg).
		AddTask(taskCfg).
		WithOutputs(map[string]string{"message": id})
	wfCfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NoError(t, wfCfg.SetCWD(projectDir))
	return wfCfg
}

func defaultBuilder(
	projectCfg *engineproject.Config,
	workflowCfg *engineworkflow.Config,
	store resources.ResourceStore,
) *Builder {
	builder := New(projectCfg).
		WithWorkflows(workflowCfg).
		WithDatabase("postgres://user:pass@localhost:5432/compozy?sslmode=disable").
		WithTemporal("localhost:7233", "default").
		WithRedis("redis://localhost:6379/0")

	if projectCfg.GetCWD() != nil {
		builder = builder.WithWorkingDirectory(projectCfg.GetCWD().PathStr())
	}
	if store != nil {
		builder = builder.WithResourceStore(store)
	}
	return builder
}

func newTestContext(t *testing.T) (context.Context, *recordingLogger) {
	t.Helper()
	baseCtx := helpers.NewTestContext(t)
	log := &recordingLogger{}
	ctx := logger.ContextWithLogger(baseCtx, log)
	return ctx, log
}

type logEntry struct {
	msg  string
	args []any
}

type recordingLogger struct {
	entries []logEntry
}

func (l *recordingLogger) Debug(string, ...any)      {}
func (l *recordingLogger) Warn(string, ...any)       {}
func (l *recordingLogger) Error(string, ...any)      {}
func (l *recordingLogger) With(...any) logger.Logger { return l }
func (l *recordingLogger) Info(msg string, args ...any) {
	copied := append([]any(nil), args...)
	l.entries = append(l.entries, logEntry{msg: msg, args: copied})
}
