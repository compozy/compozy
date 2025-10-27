package project

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/testutil"
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

func sampleModel(id string) *core.ProviderConfig {
	return &core.ProviderConfig{
		Provider: core.ProviderOpenAI,
		Model:    id,
		Default:  id == "default",
	}
}

func sampleWorkflow(id string) *workflow.Config {
	return &workflow.Config{
		ID: id,
	}
}

func sampleAgent(id string) *agent.Config {
	return &agent.Config{
		ID: id,
	}
}

func TestNewCreatesValidBuilder(t *testing.T) {
	t.Parallel()

	builder := New("  example-project  ")
	require.NotNil(t, builder)
	require.NotNil(t, builder.config)
	require.Equal(t, "example-project", builder.config.Name)
	require.Len(t, builder.config.Models, 0)
	require.Len(t, builder.workflows, 0)
	require.Len(t, builder.agents, 0)
}

func TestWithVersionValidatesSemver(t *testing.T) {
	t.Parallel()

	builder := New("proj").WithVersion("1.2.3")
	builder.AddWorkflow(sampleWorkflow("wf-1"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "1.2.3", cfg.Version)
}

func TestWithVersionInvalidProducesBuildError(t *testing.T) {
	t.Parallel()

	builder := New("proj").WithVersion("not-semver")
	builder.AddWorkflow(sampleWorkflow("wf-1"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.True(t, errors.Is(err, buildErr))
	require.NotEmpty(t, buildErr.Errors)
}

func TestWithDescriptionStoresValue(t *testing.T) {
	t.Parallel()

	builder := New("proj").WithDescription("  sample description ")
	builder.AddWorkflow(sampleWorkflow("wf-1"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "sample description", cfg.Description)
}

func TestWithDescriptionEmptyAddsError(t *testing.T) {
	t.Parallel()

	builder := New("proj").WithDescription("")
	builder.AddWorkflow(sampleWorkflow("wf-1"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "description cannot be empty")
}

func TestWithAuthorValidSetsMetadata(t *testing.T) {
	t.Parallel()

	builder := New("proj").
		WithAuthor("Jane Doe", "jane@example.com", "ACME").
		AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Equal(t, "Jane Doe", cfg.Author.Name)
	require.Equal(t, "jane@example.com", cfg.Author.Email)
	require.Equal(t, "ACME", cfg.Author.Organization)
}

func TestWithAuthorInvalidEmailFailsBuild(t *testing.T) {
	t.Parallel()

	builder := New("proj").
		WithAuthor("Jane Doe", "invalid-email", "ACME").
		AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "author email must be valid")
}

func TestWithAuthorEmptyNameFailsBuild(t *testing.T) {
	t.Parallel()

	builder := New("proj").
		WithAuthor(" ", "jane@example.com", "ACME").
		AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	_, err := builder.Build(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "author name cannot be empty")
}

func TestWithAuthorEmptyEmailFailsBuild(t *testing.T) {
	t.Parallel()

	builder := New("proj").
		WithAuthor("Jane Doe", " ", "ACME").
		AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	_, err := builder.Build(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "author email cannot be empty")
}

func TestAddModelAccumulatesModels(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	builder.AddModel(sampleModel("model-a"))
	builder.AddModel(sampleModel("model-b"))

	require.Len(t, builder.config.Models, 2)
	require.Equal(t, "model-a", builder.config.Models[0].Model)
	require.Equal(t, "model-b", builder.config.Models[1].Model)
}

func TestAddModelNilProducesError(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	builder.AddModel(nil)
	builder.AddWorkflow(sampleWorkflow("wf-1"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "model cannot be nil")
}

func TestAddWorkflowAccumulatesWorkflows(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	builder.AddWorkflow(sampleWorkflow("wf-1"))
	builder.AddWorkflow(sampleWorkflow("wf-2"))

	require.Len(t, builder.workflows, 2)
	require.Equal(t, "wf-1", builder.workflows[0].ID)
	require.Equal(t, "wf-2", builder.workflows[1].ID)
}

func TestAddWorkflowNilProducesError(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	builder.AddWorkflow(nil)
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "workflow cannot be nil")
}

func TestAddAgentAccumulatesAgents(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	builder.AddAgent(sampleAgent("agent-1"))
	builder.AddAgent(sampleAgent("agent-2"))

	require.Len(t, builder.agents, 2)
	require.Equal(t, "agent-1", builder.agents[0].ID)
	require.Equal(t, "agent-2", builder.agents[1].ID)
}

func TestAddAgentNilProducesError(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	builder.AddAgent(nil)
	builder.AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "agent cannot be nil")
}

func TestBuildWithValidConfigurationSucceeds(t *testing.T) {
	t.Parallel()

	builder := New("proj").
		WithVersion("0.1.0").
		WithAuthor("Jane Doe", "jane@example.com", "ACME").
		AddModel(sampleModel("model-a")).
		AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotSame(t, builder.config, cfg)
	require.Equal(t, "proj", cfg.Name)
	require.Equal(t, "0.1.0", cfg.Version)
	require.Len(t, cfg.Models, 1)
}

func TestBuildWithEmptyNameFails(t *testing.T) {
	t.Parallel()

	builder := New("   ")
	builder.AddWorkflow(sampleWorkflow("wf"))
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "project name")
}

func TestBuildWithoutWorkflowsFails(t *testing.T) {
	t.Parallel()

	builder := New("proj")
	ctx := testutil.NewTestContext(t)

	cfg, err := builder.Build(ctx)
	require.Error(t, err)
	require.Nil(t, cfg)

	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	require.Contains(t, buildErr.Error(), "at least one workflow must be registered")
}

func TestBuildAggregatesMultipleErrors(t *testing.T) {
	t.Parallel()

	builder := New(" ").
		WithDescription("").
		AddModel(nil).
		AddWorkflow(nil)
	ctx := testutil.NewTestContext(t)

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
	ctx := logger.ContextWithLogger(testutil.NewTestContext(t), recLogger)

	builder := New("proj").
		WithAuthor("Jane Doe", "jane@example.com", "ACME").
		AddWorkflow(sampleWorkflow("wf"))

	_, err := builder.Build(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, recLogger.debugMsgs)
	require.Contains(t, recLogger.debugMsgs[0], "building project configuration")
}

func TestAddScheduleClonesAndTrimsConfig(t *testing.T) {
	t.Parallel()

	scheduleCfg := &engineschedule.Config{
		ID:         "  weekly-report  ",
		WorkflowID: "  analytics ",
		Cron:       "0 9 * * 1",
		Input: map[string]any{
			"team": "growth",
		},
	}

	builder := New("proj").
		AddWorkflow(sampleWorkflow("analytics")).
		AddSchedule(scheduleCfg)

	ctx := testutil.NewTestContext(t)
	cfg, err := builder.Build(ctx)
	require.NoError(t, err)
	require.Len(t, cfg.Schedules, 1)
	require.Equal(t, "weekly-report", cfg.Schedules[0].ID)
	require.Equal(t, "analytics", cfg.Schedules[0].WorkflowID)
	require.Equal(t, "growth", cfg.Schedules[0].Input["team"])

	scheduleCfg.ID = "changed"
	require.Equal(t, "weekly-report", cfg.Schedules[0].ID)
}

func TestAddScheduleValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schedule   *engineschedule.Config
		errMessage string
	}{
		{
			name:       "nil schedule",
			schedule:   nil,
			errMessage: "schedule cannot be nil",
		},
		{
			name: "empty schedule id",
			schedule: &engineschedule.Config{
				ID:         "  ",
				WorkflowID: "wf",
				Cron:       "* * * * *",
			},
			errMessage: "schedule id cannot be empty",
		},
		{
			name: "empty workflow id",
			schedule: &engineschedule.Config{
				ID:         "sched-1",
				WorkflowID: " ",
				Cron:       "* * * * *",
			},
			errMessage: "schedule workflow id cannot be empty",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builder := New("proj").AddWorkflow(sampleWorkflow("wf"))
			builder.AddSchedule(tc.schedule)
			_, err := builder.Build(testutil.NewTestContext(t))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.errMessage)
		})
	}
}

func TestBuildValidatesSchedules(t *testing.T) {
	t.Parallel()

	ctx := testutil.NewTestContext(t)
	builder := New("proj").AddWorkflow(sampleWorkflow("wf"))
	builder.AddSchedule(&engineschedule.Config{ID: "sched", WorkflowID: "missing", Cron: "* * * * *"})
	_, err := builder.Build(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown workflow")

	builder = New("proj").AddWorkflow(sampleWorkflow("wf"))
	builder.AddSchedule(&engineschedule.Config{ID: "dup", WorkflowID: "wf", Cron: "* * * * *"})
	builder.AddSchedule(&engineschedule.Config{ID: "dup", WorkflowID: "wf", Cron: "* * * * *"})
	_, err = builder.Build(testutil.NewTestContext(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate schedule ids")
}

func TestFindDuplicateScheduleIDs(t *testing.T) {
	t.Parallel()

	schedules := []*engineschedule.Config{
		{ID: "dup"},
		{ID: "dup"},
		{ID: "dup"},
		{ID: " unique "},
		{ID: ""},
		nil,
	}

	dupes := findDuplicateScheduleIDs(schedules)
	require.Len(t, dupes, 1)
	require.Equal(t, "dup", dupes[0])
}

func TestContainsString(t *testing.T) {
	t.Parallel()
	require.True(t, containsString([]string{"a", "b"}, "b"))
	require.False(t, containsString([]string{"a"}, "c"))
}

func TestNilBuilderMethods(t *testing.T) {
	t.Parallel()

	var builder *Builder
	require.Nil(t, builder.WithVersion("1.0.0"))
	require.Nil(t, builder.WithDescription("desc"))
	require.Nil(t, builder.WithAuthor("name", "mail@example.com", "org"))
	require.Nil(t, builder.AddModel(&core.ProviderConfig{}))
	require.Nil(t, builder.AddWorkflow(&workflow.Config{}))
	require.Nil(t, builder.AddAgent(&agent.Config{}))
	require.Nil(t, builder.AddSchedule(&engineschedule.Config{}))
}

func TestBuildNilContext(t *testing.T) {
	t.Parallel()

	builder := New("proj").AddWorkflow(sampleWorkflow("wf"))
	cfg, err := builder.Build(nil)
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestBuildNilBuilder(t *testing.T) {
	t.Parallel()

	var builder *Builder
	cfg, err := builder.Build(testutil.NewTestContext(t))
	require.Error(t, err)
	require.Nil(t, cfg)
}

func TestBuildRejectsInvalidProjectNameCharacters(t *testing.T) {
	t.Parallel()

	builder := New("invalid name").AddWorkflow(sampleWorkflow("wf"))
	_, err := builder.Build(testutil.NewTestContext(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "alphanumeric or hyphenated")
}

func TestBuildHandlesNilCollections(t *testing.T) {
	t.Parallel()

	builder := New("proj").AddWorkflow(sampleWorkflow("wf"))
	builder.workflows = append(builder.workflows, nil)
	builder.schedules = append(builder.schedules, nil)
	cfg, err := builder.Build(testutil.NewTestContext(t))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, cfg.Schedules)
}
