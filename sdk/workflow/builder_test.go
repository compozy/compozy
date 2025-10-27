package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

type recordingLogger struct {
	debugMsgs []string
}

func (l *recordingLogger) Debug(msg string, _ ...any) {
	l.debugMsgs = append(l.debugMsgs, msg)
}

func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}
func (l *recordingLogger) Error(string, ...any) {}
func (l *recordingLogger) With(...any) logger.Logger {
	return l
}

func sampleAgent(id string) *agent.Config {
	return &agent.Config{
		ID:           id,
		Instructions: "do something useful",
		Model:        agent.Model{Ref: "openai-gpt-4o-mini"},
	}
}

func sampleTask(id string) *task.Config {
	return &task.Config{
		BaseConfig: task.BaseConfig{
			ID:   id,
			Type: task.TaskTypeBasic,
		},
	}
}

func TestNewCreatesValidBuilder(t *testing.T) {
	t.Parallel()

	builder := New("  example-workflow  ")
	require.NotNil(t, builder)
	require.Equal(t, "example-workflow", builder.config.ID)
	require.NotNil(t, builder.config)
	require.Len(t, builder.config.Agents, 0)
	require.Len(t, builder.config.Tasks, 0)
}

func TestWithDescriptionStoresValue(t *testing.T) {
	t.Parallel()

	builder := New("wf").WithDescription("  sample workflow  ")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "sample workflow", cfg.Description)
}

func TestWithDescriptionEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("wf").WithDescription("")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "description cannot be empty")
}

func TestAddAgentAccumulatesAgents(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddAgent(sampleAgent("agent-a"))
	builder.AddAgent(sampleAgent("agent-b"))

	require.Len(t, builder.config.Agents, 2)
	require.Equal(t, "agent-a", builder.config.Agents[0].ID)
	require.Equal(t, "agent-b", builder.config.Agents[1].ID)
}

func TestAddAgentNilProducesError(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddAgent(nil)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "agent cannot be nil")
}

func TestAddTaskAccumulatesTasks(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))
	builder.AddTask(sampleTask("task-2"))

	require.Len(t, builder.config.Tasks, 2)
	require.Equal(t, "task-1", builder.config.Tasks[0].ID)
	require.Equal(t, "task-2", builder.config.Tasks[1].ID)
}

func TestAddTaskNilProducesError(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(nil)
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "task cannot be nil")
}

func TestWithInputSetsSchema(t *testing.T) {
	t.Parallel()

	s := schema.Schema{"type": "object"}
	builder := New("wf").WithInput(&s)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.Opts.InputSchema)
	require.Equal(t, &s, cfg.Opts.InputSchema)
}

func TestWithOutputsSetsValues(t *testing.T) {
	t.Parallel()

	outputs := map[string]string{
		"result": "{{ .tasks.finish.output }}",
	}
	builder := New("wf").WithOutputs(outputs)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg.Outputs)
	require.Equal(t, "{{ .tasks.finish.output }}", (*cfg.Outputs)["result"])
	_, ok := (*cfg.Outputs)["result"]
	require.True(t, ok)
}

func TestWithOutputsEmptyKeyAddsError(t *testing.T) {
	t.Parallel()

	outputs := map[string]string{
		" ": "value",
	}
	builder := New("wf").WithOutputs(outputs)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "output key cannot be empty")
}

func TestBuildValidConfigurationSucceeds(t *testing.T) {
	t.Parallel()

	builder := New("wf").
		WithDescription("workflow").
		AddAgent(sampleAgent("agent-a")).
		AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotSame(t, builder.config, cfg)
	require.Equal(t, "wf", cfg.ID)
	require.Len(t, cfg.Tasks, 1)
	require.Len(t, cfg.Agents, 1)
}

func TestBuildWithEmptyIDFails(t *testing.T) {
	t.Parallel()

	builder := New("   ")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "workflow id is invalid")
}

func TestBuildWithoutTasksFails(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "at least one task must be registered")
}

func TestBuildWithDuplicateTaskIDsFails(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "duplicate task ids found")
}

func TestBuildAggregatesMultipleErrors(t *testing.T) {
	t.Parallel()

	builder := New(" ").
		WithDescription("").
		AddAgent(nil).
		AddTask(nil)
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.GreaterOrEqual(t, len(buildErr.Errors), 3)
}

func TestBuildUsesLoggerFromContext(t *testing.T) {
	t.Parallel()

	recLogger := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), recLogger)

	builder := New("wf").
		AddTask(sampleTask("task-1"))

	_, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, recLogger.debugMsgs)
	require.Contains(t, recLogger.debugMsgs[0], "building workflow configuration")
}

func TestFindDuplicateTaskIDsHelper(t *testing.T) {
	t.Parallel()

	input := []task.Config{
		{BaseConfig: task.BaseConfig{ID: "task-1"}},
		{BaseConfig: task.BaseConfig{ID: "task-2"}},
		{BaseConfig: task.BaseConfig{ID: "task-1"}},
		{BaseConfig: task.BaseConfig{ID: "task-3"}},
		{BaseConfig: task.BaseConfig{ID: "task-2"}},
	}

	duplicates := findDuplicateTaskIDs(input)
	require.ElementsMatch(t, []string{"task-1", "task-2"}, duplicates)
}

func TestBuildReturnsClonedConfig(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)

	cfg.Description = "mutated"
	require.NotEqual(t, builder.config.Description, cfg.Description)
}

func TestBuildReturnsErrorWhenContextNil(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))

	var missingCtx context.Context
	cfg, err := builder.Build(missingCtx)
	require.Error(t, err)
	require.Nil(t, cfg)
	require.False(t, errors.Is(err, &sdkerrors.BuildError{}))
}

func TestAddTaskEmptyIDProducesError(t *testing.T) {
	t.Parallel()

	taskCfg := sampleTask("   ")
	builder := New("wf")
	builder.AddTask(taskCfg)
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "task id cannot be empty")
}

func TestAddAgentEmptyIDProducesError(t *testing.T) {
	t.Parallel()

	agentCfg := sampleAgent("   ")
	builder := New("wf")
	builder.AddAgent(agentCfg)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "agent id cannot be empty")
}

func TestWithOutputsNilClearsOutputs(t *testing.T) {
	t.Parallel()

	builder := New("wf").WithOutputs(map[string]string{"result": "value"}).WithOutputs(nil)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Nil(t, cfg.Outputs)
}

func TestBuildReturnsWorkflowConfigType(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.IsType(t, &engineworkflow.Config{}, cfg)
}

func TestAddAgentDeepCopyPreventsMutation(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	agentCfg := sampleAgent("agent-a")
	builder.AddAgent(agentCfg)

	agentCfg.Instructions = "mutated"
	require.NotEqual(t, agentCfg.Instructions, builder.config.Agents[0].Instructions)
}

func TestAddTaskDeepCopyPreventsMutation(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	taskCfg := sampleTask("task-1")
	builder.AddTask(taskCfg)

	taskCfg.Type = task.TaskTypeRouter
	require.NotEqual(t, taskCfg.Type, builder.config.Tasks[0].Type)
}

func TestWithOutputsTrimsKeys(t *testing.T) {
	t.Parallel()

	builder := New("wf").WithOutputs(map[string]string{"  result  ": "value"})
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Contains(t, *cfg.Outputs, "result")
}

func TestBuildErrorImplementsUnwrap(t *testing.T) {
	t.Parallel()

	builder := New(" ")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	_, err := builder.Build(ctx)
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)

	require.NotNil(t, errors.Unwrap(buildErr))
}

func TestBuildIncludesAgentAndTaskCountsInLog(t *testing.T) {
	t.Parallel()

	rec := &recordingLogger{}
	ctx := logger.ContextWithLogger(t.Context(), rec)

	builder := New("wf").
		AddAgent(sampleAgent("agent-a")).
		AddTask(sampleTask("task-1"))

	_, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, rec.debugMsgs)
}

func TestFindDuplicateTaskIDsSkipsEmptyIDs(t *testing.T) {
	t.Parallel()

	input := []task.Config{
		{BaseConfig: task.BaseConfig{ID: ""}},
		{BaseConfig: task.BaseConfig{ID: "task-1"}},
		{BaseConfig: task.BaseConfig{ID: ""}},
		{BaseConfig: task.BaseConfig{ID: "task-1"}},
	}
	duplicates := findDuplicateTaskIDs(input)
	require.Equal(t, []string{"task-1"}, duplicates)
}

func TestContainsStringHelper(t *testing.T) {
	t.Parallel()
	require.True(t, containsString([]string{"a", "b"}, "a"))
	require.False(t, containsString([]string{"a", "b"}, "c"))
}

func TestBuildPreservesOutputs(t *testing.T) {
	t.Parallel()

	builder := New("wf").
		WithOutputs(map[string]string{"result": "value"}).
		AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)

	require.Equal(t, "value", (*cfg.Outputs)["result"])
}

func TestBuildHandlesNoAgentsGracefully(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Agents, 0)
}

func TestBuildReturnsErrorWhenBuilderNil(t *testing.T) {
	t.Parallel()

	var builder *Builder
	cfg, err := builder.Build(t.Context())
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestWithDescriptionNilBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder
	require.Nil(t, builder.WithDescription("test"))
}

func TestAddAgentNilBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder
	require.Nil(t, builder.AddAgent(sampleAgent("agent")))
}

func TestAddTaskNilBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder
	require.Nil(t, builder.AddTask(sampleTask("task")))
}

func TestWithInputNilBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder
	require.Nil(t, builder.WithInput(&schema.Schema{}))
}

func TestWithOutputsNilBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder
	require.Nil(t, builder.WithOutputs(map[string]string{}))
}

func TestNewTrimsIdentifier(t *testing.T) {
	t.Parallel()

	builder := New("  wf-id  ")
	require.Equal(t, "wf-id", builder.config.ID)
}

func TestBuildDuplicateTaskIDsReportOnce(t *testing.T) {
	t.Parallel()

	tasks := []task.Config{
		{BaseConfig: task.BaseConfig{ID: "a"}},
		{BaseConfig: task.BaseConfig{ID: "a"}},
		{BaseConfig: task.BaseConfig{ID: "a"}},
	}
	duplicates := findDuplicateTaskIDs(tasks)
	require.Equal(t, []string{"a"}, duplicates)
}

func TestBuildWithOutputsClonesMap(t *testing.T) {
	t.Parallel()

	source := map[string]string{"result": "value"}
	builder := New("wf").WithOutputs(source)
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)

	source["result"] = "changed"
	require.Equal(t, "value", (*cfg.Outputs)["result"])
}

func TestBuildReturnsErrorWhenLoggerMissingButContextProvided(t *testing.T) {
	t.Parallel()

	builder := New("wf")
	builder.AddTask(sampleTask("task-1"))
	ctx := t.Context()

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestBuildErrorContainsAllCollectedErrors(t *testing.T) {
	t.Parallel()

	builder := New(" ")
	builder.AddTask(nil)
	ctx := t.Context()

	_, err := builder.Build(ctx)
	require.Error(t, err)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Len(t, buildErr.Errors, len(buildErr.Errors))
}
