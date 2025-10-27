package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/schedule"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

const (
	projectID        = "scheduled-reports"
	workflowID       = "analytics-report"
	reportAgentID    = "reporting-analyst"
	reportActionID   = "generate-report"
	reportTaskID     = "produce-report"
	dailyScheduleID  = "daily-report-schedule"
	weeklyScheduleID = "weekly-summary-schedule"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("scheduled workflow example failed", "error", err)
		cleanup()
		os.Exit(1)
	}
	cleanup()
}

func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.WithoutCancel(context.Background()))
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", err)
	}
	ctx = config.ContextWithManager(ctx, manager)
	cleanup := func() {
		if err := manager.Close(ctx); err != nil {
			logger.FromContext(ctx).Warn("failed to close configuration manager", "error", err)
		}
		cancel()
	}
	return ctx, cleanup, nil
}

func run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("running scheduled workflow example", "environment", currentEnvironment(ctx))
	modelCfg, err := buildModel(ctx)
	if err != nil {
		return handleBuildError(ctx, "model", err)
	}
	actionCfg, err := buildReportAction(ctx)
	if err != nil {
		return handleBuildError(ctx, "action", err)
	}
	agentCfg, err := buildReportingAgent(ctx, actionCfg)
	if err != nil {
		return handleBuildError(ctx, "agent", err)
	}
	taskCfg, err := buildReportingTask(ctx, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "task", err)
	}
	workflowCfg, err := buildReportingWorkflow(ctx, agentCfg, taskCfg)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	dailySchedule, err := buildDailySchedule(ctx, workflowCfg.ID)
	if err != nil {
		return handleBuildError(ctx, "daily_schedule", err)
	}
	weeklySchedule, err := buildWeeklySchedule(ctx, workflowCfg.ID)
	if err != nil {
		return handleBuildError(ctx, "weekly_schedule", err)
	}
	projectCfg, err := buildProject(ctx, modelCfg, workflowCfg, agentCfg, dailySchedule, weeklySchedule)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	summarizeSchedules(ctx, projectCfg, dailySchedule, weeklySchedule)
	return nil
}

func currentEnvironment(ctx context.Context) string {
	if cfg := config.FromContext(ctx); cfg != nil {
		return cfg.Runtime.Environment
	}
	return "unknown"
}

func buildModel(ctx context.Context) (*core.ProviderConfig, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		logger.FromContext(ctx).Warn("OPENAI_API_KEY is not set; scheduled runs will fail without provider credentials")
	}
	return model.New("openai", "gpt-4o-mini").
		WithAPIKey(apiKey).
		WithDefault(true).
		WithTemperature(0.2).
		Build(ctx)
}

func buildReportAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	return agent.NewAction(reportActionID).
		WithPrompt("Summarize key metrics and highlight anomalies for the reporting window.").
		Build(ctx)
}

func buildReportingAgent(ctx context.Context, actionCfg *engineagent.ActionConfig) (*engineagent.Config, error) {
	if actionCfg == nil {
		return nil, fmt.Errorf("action config is required")
	}
	return agent.New(reportAgentID).
		WithInstructions("You generate analytics summaries and focus on trend shifts.").
		WithModel("openai", "gpt-4o-mini").
		AddAction(actionCfg).
		Build(ctx)
}

func buildReportingTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	return task.NewBasic(reportTaskID).
		WithAgent(agentCfg.ID).
		WithAction(reportActionID).
		WithFinal(true).
		Build(ctx)
}

func buildReportingWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	taskCfg *enginetask.Config,
) (*engineworkflow.Config, error) {
	return workflow.New(workflowID).
		WithDescription("Generates analytics reports for scheduled dispatch").
		AddAgent(agentCfg).
		AddTask(taskCfg).
		Build(ctx)
}

func buildDailySchedule(ctx context.Context, wfID string) (*engineschedule.Config, error) {
	// Cron layout: minute hour day month weekday -> "0 9 * * *" fires at 09:00 UTC every day.
	return schedule.New(dailyScheduleID).
		WithWorkflow(wfID).
		WithCron("0 9 * * *").
		WithInput(map[string]any{
			"report_type":    "daily",
			"include_charts": true,
		}).
		WithRetry(3, 5*time.Minute).
		Build(ctx)
}

func buildWeeklySchedule(ctx context.Context, wfID string) (*engineschedule.Config, error) {
	// "0 10 * * 1" fires at 10:00 UTC every Monday to capture weekly performance trends.
	return schedule.New(weeklyScheduleID).
		WithWorkflow(wfID).
		WithCron("0 10 * * 1").
		WithInput(map[string]any{
			"report_type":       "weekly",
			"include_charts":    true,
			"comparison_window": "7d",
		}).
		WithRetry(3, 10*time.Minute).
		Build(ctx)
}

func buildProject(
	ctx context.Context,
	modelCfg *core.ProviderConfig,
	workflowCfg *engineworkflow.Config,
	agentCfg *engineagent.Config,
	schedules ...*engineschedule.Config,
) (*engineproject.Config, error) {
	builder := project.New(projectID).
		WithDescription("Analytics project with automated schedules").
		AddModel(modelCfg).
		AddWorkflow(workflowCfg).
		AddAgent(agentCfg)
	for _, sched := range schedules {
		builder = builder.AddSchedule(sched)
	}
	return builder.Build(ctx)
}

func summarizeSchedules(
	ctx context.Context,
	projectCfg *engineproject.Config,
	schedules ...*engineschedule.Config,
) {
	log := logger.FromContext(ctx)
	log.Info("project configured with schedules", "project", projectCfg.Name, "schedule_count", len(schedules))
	for _, sched := range schedules {
		if sched == nil {
			continue
		}
		retryLabel := "disabled"
		if sched.Retry != nil {
			retryLabel = fmt.Sprintf("%d attempts with %s backoff", sched.Retry.MaxAttempts, sched.Retry.Backoff)
		}
		log.Info(
			"schedule ready",
			"schedule_id",
			sched.ID,
			"workflow",
			sched.WorkflowID,
			"cron",
			sched.Cron,
			"input_keys",
			mapKeys(sched.Input),
			"retry",
			retryLabel,
		)
	}
}

func mapKeys(input map[string]any) []string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}

func handleBuildError(ctx context.Context, stage string, err error) error {
	var buildErr *sdkerrors.BuildError
	if errors.As(err, &buildErr) {
		log := logger.FromContext(ctx)
		for idx, cause := range buildErr.Errors {
			if cause == nil {
				continue
			}
			log.Error("builder validation failed", "stage", stage, "index", idx+1, "cause", cause.Error())
		}
	}
	return fmt.Errorf("%s build failed: %w", stage, err)
}
