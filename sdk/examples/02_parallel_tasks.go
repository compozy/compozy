//go:build examples

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/schema"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

const (
	defaultProvider = "openai"
	defaultModel    = "gpt-4o-mini"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("parallel tasks example failed", "error", err)
		os.Exit(1)
	}
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
	env := "unknown"
	if cfg := config.FromContext(ctx); cfg != nil {
		env = cfg.Runtime.Environment
	}
	log.Info("running parallel analysis example", "environment", env)
	modelCfg, err := buildModel(ctx)
	if err != nil {
		return handleBuildError(ctx, "model", err)
	}
	sentimentAgent, err := buildSentimentAgent(ctx)
	if err != nil {
		return handleBuildError(ctx, "sentiment agent", err)
	}
	entityAgent, err := buildEntityAgent(ctx)
	if err != nil {
		return handleBuildError(ctx, "entity agent", err)
	}
	summaryAgent, err := buildSummaryAgent(ctx)
	if err != nil {
		return handleBuildError(ctx, "summary agent", err)
	}
	sentimentTask, err := buildSentimentTask(ctx, sentimentAgent)
	if err != nil {
		return handleBuildError(ctx, "sentiment task", err)
	}
	entityTask, err := buildEntityTask(ctx, entityAgent)
	if err != nil {
		return handleBuildError(ctx, "entity task", err)
	}
	summaryTask, err := buildSummaryTask(ctx, summaryAgent)
	if err != nil {
		return handleBuildError(ctx, "summary task", err)
	}
	parallelTask, err := buildParallelAnalysis(ctx, sentimentTask, entityTask, summaryTask)
	if err != nil {
		return handleBuildError(ctx, "parallel task", err)
	}
	aggregateTask, err := buildAggregateResults(ctx, sentimentTask, entityTask, summaryTask)
	if err != nil {
		return handleBuildError(ctx, "aggregate task", err)
	}
	workflowCfg, err := buildWorkflow(
		ctx,
		sentimentAgent,
		entityAgent,
		summaryAgent,
		sentimentTask,
		entityTask,
		summaryTask,
		parallelTask,
		aggregateTask,
	)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	projectCfg, err := buildProject(ctx, modelCfg, workflowCfg, sentimentAgent, entityAgent, summaryAgent)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	printSummary(projectCfg, workflowCfg, parallelTask, aggregateTask)
	return nil
}

func buildModel(ctx context.Context) (*core.ProviderConfig, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		logger.FromContext(ctx).Warn("OPENAI_API_KEY is not set; API calls will fail without it")
	}
	return model.New(defaultProvider, defaultModel).
		WithAPIKey(apiKey).
		WithDefault(true).
		WithTemperature(0.1).
		Build(ctx)
}

func buildSentimentAgent(ctx context.Context) (*engineagent.Config, error) {
	sentimentProperty := schema.NewString().
		WithDescription("Sentiment label").
		WithEnum("positive", "neutral", "negative")
	confidenceProperty := schema.NewNumber().
		WithMinimum(0).
		WithMaximum(1)
	output, err := schema.NewObject().
		AddProperty("sentiment", sentimentProperty).
		AddProperty("confidence", confidenceProperty).
		RequireProperty("sentiment").
		RequireProperty("confidence").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	action, err := agent.NewAction("analyze-sentiment").
		WithPrompt("Analyze the sentiment of {{ .input.text }} and return a label and confidence between 0 and 1.").
		WithOutput(output).
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return agent.New("sentiment-analyst").
		WithModel(defaultProvider, defaultModel).
		WithInstructions("You classify customer feedback sentiment with calibrated confidence scores.").
		AddAction(action).
		Build(ctx)
}

func buildEntityAgent(ctx context.Context) (*engineagent.Config, error) {
	entityItems := schema.NewString().
		WithDescription("Named entity extracted from text")
	entityOutput, err := schema.NewObject().
		AddProperty("entities", schema.NewArray(entityItems)).
		RequireProperty("entities").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	action, err := agent.NewAction("extract-entities").
		WithPrompt("Extract key entities from {{ .input.text }} and return a list of labeled entities.").
		WithOutput(entityOutput).
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return agent.New("entity-extractor").
		WithModel(defaultProvider, defaultModel).
		WithInstructions("You identify people, organizations, and products mentioned in customer feedback.").
		AddAction(action).
		Build(ctx)
}

func buildSummaryAgent(ctx context.Context) (*engineagent.Config, error) {
	summaryProperty := schema.NewString().
		WithMinLength(10)
	summaryOutput, err := schema.NewObject().
		AddProperty("summary", summaryProperty).
		RequireProperty("summary").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	action, err := agent.NewAction("summarize-feedback").
		WithPrompt("Summarize {{ .input.text }} in two concise bullet points focused on customer sentiment and requests.").
		WithOutput(summaryOutput).
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return agent.New("feedback-summarizer").
		WithModel(defaultProvider, defaultModel).
		WithInstructions("You craft concise summaries that capture emotion and customer asks.").
		AddAction(action).
		Build(ctx)
}

func buildSentimentTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("sentiment agent config is required")
	}
	return task.NewBasic("sentiment-analysis").
		WithAgent(agentCfg.ID).
		WithAction("analyze-sentiment").
		WithInput(map[string]string{"text": "{{ .workflow.input.feedback }}"}).
		WithOutput("sentiment = {{ .result.output.sentiment }}").
		WithOutput("confidence = {{ .result.output.confidence }}").
		Build(ctx)
}

func buildEntityTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("entity agent config is required")
	}
	return task.NewBasic("entity-extraction").
		WithAgent(agentCfg.ID).
		WithAction("extract-entities").
		WithInput(map[string]string{"text": "{{ .workflow.input.feedback }}"}).
		WithOutput("entities = {{ .result.output.entities }}").
		Build(ctx)
}

func buildSummaryTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("summary agent config is required")
	}
	return task.NewBasic("feedback-summary").
		WithAgent(agentCfg.ID).
		WithAction("summarize-feedback").
		WithInput(map[string]string{"text": "{{ .workflow.input.feedback }}"}).
		WithOutput("summary = {{ .result.output.summary }}").
		Build(ctx)
}

func buildParallelAnalysis(
	ctx context.Context,
	sentimentTask, entityTask, summaryTask *enginetask.Config,
) (*enginetask.Config, error) {
	if sentimentTask == nil || entityTask == nil || summaryTask == nil {
		return nil, fmt.Errorf("all analysis tasks are required for parallel execution")
	}
	parallel, err := task.NewParallel("analysis-parallel").
		AddTask(sentimentTask.ID).
		AddTask(entityTask.ID).
		AddTask(summaryTask.ID).
		WithWaitAll(true). // Wait for all branches so aggregation sees consistent data
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return parallel, nil
}

func buildAggregateResults(
	ctx context.Context,
	sentimentTask, entityTask, summaryTask *enginetask.Config,
) (*enginetask.Config, error) {
	aggregate, err := task.NewAggregate("analysis-aggregate").
		AddTask(sentimentTask.ID).
		AddTask(entityTask.ID).
		AddTask(summaryTask.ID).
		WithStrategy("merge").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	aggregate.Final = true
	return aggregate, nil
}

func buildWorkflow(
	ctx context.Context,
	sentimentAgent, entityAgent, summaryAgent *engineagent.Config,
	sentimentTask, entityTask, summaryTask, parallelTask, aggregateTask *enginetask.Config,
) (*engineworkflow.Config, error) {
	textInput := schema.NewString().
		WithDescription("Customer feedback to analyze").
		WithMinLength(20)
	feedbackSchema, err := schema.NewObject().
		AddProperty("feedback", textInput).
		RequireProperty("feedback").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return workflow.New("parallel-analysis-workflow").
		WithDescription("Runs sentiment, entity, and summary tasks in parallel and merges outputs").
		WithInput(feedbackSchema).
		AddAgent(sentimentAgent).
		AddAgent(entityAgent).
		AddAgent(summaryAgent).
		AddTask(sentimentTask).
		AddTask(entityTask).
		AddTask(summaryTask).
		AddTask(parallelTask).
		AddTask(aggregateTask).
		WithOutputs(map[string]string{
			"analysis": "{{ task \"analysis-aggregate\" \"aggregated.result\" }}",
		}).
		Build(ctx)
}

func buildProject(
	ctx context.Context,
	modelCfg *core.ProviderConfig,
	workflowCfg *engineworkflow.Config,
	sentimentAgent, entityAgent, summaryAgent *engineagent.Config,
) (*engineproject.Config, error) {
	return project.New("parallel-analysis-demo").
		WithVersion("1.0.0").
		WithDescription("Demonstrates parallel task execution with aggregated results").
		AddModel(modelCfg).
		AddWorkflow(workflowCfg).
		AddAgent(sentimentAgent).
		AddAgent(entityAgent).
		AddAgent(summaryAgent).
		Build(ctx)
}

func printSummary(
	projectCfg *engineproject.Config,
	workflowCfg *engineworkflow.Config,
	parallelTask, aggregateTask *enginetask.Config,
) {
	fmt.Println("✅ Parallel analysis project built successfully")
	fmt.Printf("Project: %s\n", projectCfg.Name)
	fmt.Printf("Workflow: %s (tasks: %d)\n", workflowCfg.ID, len(workflowCfg.Tasks))
	fmt.Printf(
		"Parallel task %q runs %d analyses concurrently with wait-all strategy for consistent aggregation.\n",
		parallelTask.ID,
		len(parallelTask.Tasks),
	)
	fmt.Printf(
		"Aggregate task %q merges results so downstream systems consume a single payload.\n",
		aggregateTask.ID,
	)
	fmt.Println(
		"Use `go run ./sdk/examples/02_parallel_tasks.go` after setting OPENAI_API_KEY to explore the workflow.",
	)
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
