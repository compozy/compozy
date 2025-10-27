package main

import (
	"context"
	"fmt"
	"os"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectmonitoring "github.com/compozy/compozy/engine/project/monitoring"
	engineruntime "github.com/compozy/compozy/engine/runtime"
	enginetool "github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	engineschedule "github.com/compozy/compozy/engine/workflow/schedule"
	enginetask "github.com/compozy/compozy/engine/workflow/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	"github.com/compozy/compozy/sdk/client"
	"github.com/compozy/compozy/sdk/compozy"
	"github.com/compozy/compozy/sdk/knowledge"
	"github.com/compozy/compozy/sdk/mcp"
	"github.com/compozy/compozy/sdk/memory"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/runtime"
	"github.com/compozy/compozy/sdk/schedule"
	"github.com/compozy/compozy/sdk/schema"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/tool"
	"github.com/compozy/compozy/sdk/workflow"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	if err := runKitchenSink(ctx); err != nil {
		logger.FromContext(ctx).Error("complete project example failed", "error", err)
		os.Exit(1)
	}
}

// initializeContext prepares the base context with logger and configuration manager.
func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.Background())
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)
	manager := config.NewManager(ctx, config.NewService())
	_, loadErr := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	if loadErr != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", loadErr)
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

// runKitchenSink assembles and executes the all-in-one project workflow.
func runKitchenSink(ctx context.Context) error {
	configureMonitoring(ctx)
	resources, err := buildKitchenResources(ctx)
	if err != nil {
		return err
	}
	if err := demonstrateLifecycle(ctx, resources); err != nil {
		return err
	}
	return nil
}

// kitchenResources aggregates all builder outputs required to run the example.
type kitchenResources struct {
	models         []*enginecore.ProviderConfig
	embedder       *engineknowledge.EmbedderConfig
	vectorDB       *engineknowledge.VectorDBConfig
	knowledgeBase  *engineknowledge.BaseConfig
	memoryConfig   *enginememory.Config
	memoryRef      *memory.ReferenceConfig
	localMCP       *enginemcp.Config
	remoteMCP      *enginemcp.Config
	toolConfig     *enginetool.Config
	runtimeConfig  *engineruntime.Config
	agentConfig    *engineagent.Config
	workflows      []*engineworkflow.Config
	schedules      []*engineschedule.Config
	projectConfig  *engineproject.Config
	clientInstance *client.Client
}

type analysisCoreTasks struct {
	collect   *enginetask.Config
	mcp       *enginetask.Config
	tool      *enginetask.Config
	parallel  *enginetask.Config
	aggregate *enginetask.Config
}

type supportTasks struct {
	memory     *enginetask.Config
	signalSend *enginetask.Config
	signalWait *enginetask.Config
	wait       *enginetask.Config
	router     *enginetask.Config
}

type compositeTasks struct {
	subWorkflow *engineworkflow.Config
	composite   *enginetask.Config
	collection  *enginetask.Config
}

// buildKitchenResources composes all builders so the lifecycle demonstration can execute.
func buildKitchenResources(ctx context.Context) (*kitchenResources, error) {
	models, err := buildModels(ctx)
	if err != nil {
		return nil, err
	}
	embedderCfg, vectorCfg, knowledgeBaseCfg, err := buildKnowledgeResources(ctx)
	if err != nil {
		return nil, err
	}
	memCfg, memRef, err := buildMemoryResources(ctx)
	if err != nil {
		return nil, err
	}
	localMCP, remoteMCP, err := buildMCPServers(ctx)
	if err != nil {
		return nil, err
	}
	toolCfg, runtimeCfg, err := buildRuntimeAndTools(ctx)
	if err != nil {
		return nil, err
	}
	agentCfg, err := buildAgentConfig(ctx, knowledgeBaseCfg, memRef, toolCfg.ID, localMCP.ID, remoteMCP.ID)
	if err != nil {
		return nil, err
	}
	workflows, schedules, err := buildWorkflowSuite(ctx, agentCfg, memCfg, toolCfg.ID, remoteMCP.ID)
	if err != nil {
		return nil, err
	}
	projectCfg, err := assembleProject(
		ctx,
		models,
		workflows,
		schedules,
		agentCfg,
		embedderCfg,
		vectorCfg,
		knowledgeBaseCfg,
		memCfg,
		localMCP,
		remoteMCP,
		toolCfg,
		runtimeCfg,
	)
	if err != nil {
		return nil, err
	}
	clientInstance, err := prepareSDKClient(ctx)
	if err != nil {
		return nil, err
	}
	return &kitchenResources{
		models:         models,
		embedder:       embedderCfg,
		vectorDB:       vectorCfg,
		knowledgeBase:  knowledgeBaseCfg,
		memoryConfig:   memCfg,
		memoryRef:      memRef,
		localMCP:       localMCP,
		remoteMCP:      remoteMCP,
		toolConfig:     toolCfg,
		runtimeConfig:  runtimeCfg,
		agentConfig:    agentCfg,
		workflows:      workflows,
		schedules:      schedules,
		projectConfig:  projectCfg,
		clientInstance: clientInstance,
	}, nil
}

// configureMonitoring enables Prometheus metrics and sets tracing metadata.
func configureMonitoring(ctx context.Context) {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return
	}
	cfg.Runtime.LogLevel = "debug"
	cfg.Server.Timeouts.MonitoringInit = 3 * time.Second
	cfg.Server.Timeouts.MonitoringShutdown = 3 * time.Second
	cfg.Server.Timeouts.ServerShutdown = 5 * time.Second
}

// demonstrateLifecycle starts the embedded engine, runs a workflow, and shuts down gracefully.
func demonstrateLifecycle(ctx context.Context, resources *kitchenResources) error {
	log := logger.FromContext(ctx)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	appBuilder := compozy.New(resources.projectConfig).
		WithWorkflows(resources.workflows...).
		WithServerHost("127.0.0.1").
		WithServerPort(18080).
		WithDatabase("postgres://postgres:postgres@localhost:5432/compozy?sslmode=disable").
		WithTemporal("localhost:7233", "default").
		WithRedis("redis://localhost:6379/0").
		WithCorsOrigins("http://localhost:3000").
		WithAuth(false).
		WithWorkingDirectory(cwd).
		WithLogLevel("debug")
	app, err := appBuilder.Build(ctx)
	if err != nil {
		return fmt.Errorf("build embedded compozy: %w", err)
	}
	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := app.StartWithContext(startCtx); err != nil {
		return fmt.Errorf("start embedded compozy: %w", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stopCancel()
		if stopErr := app.Stop(stopCtx); stopErr != nil {
			log.Warn("failed to stop embedded compozy cleanly", "error", stopErr)
		}
	}()
	input := map[string]any{
		"customer": map[string]any{
			"name":  "Casey",
			"issue": "The analytics dashboard stalls when exporting CSV files.",
			"tier":  "enterprise",
		},
		"targets": []map[string]any{
			{"feature": "reporting", "priority": "high"},
			{"feature": "workflow", "priority": "medium"},
		},
	}
	result, execErr := app.ExecuteWorkflow(ctx, resources.workflows[0].ID, input)
	if execErr != nil {
		return fmt.Errorf("execute workflow: %w", execErr)
	}
	fmt.Println("✅ Complete project executed successfully")
	fmt.Printf("Workflow Outputs: %+v\n", result.Output)
	fmt.Printf("Monitoring Endpoint: http://127.0.0.1:18080/metrics\n")
	return nil
}

// buildModels configures multiple LLM providers for the project.
func buildModels(ctx context.Context) ([]*enginecore.ProviderConfig, error) {
	primary, err := model.New("openai", "gpt-4o").
		WithDefault(true).
		WithAPIKey("{{ .env.OPENAI_API_KEY }}").
		WithTemperature(0.1).
		WithMaxTokens(2000).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build primary model: %w", err)
	}
	fallback, err := model.New("anthropic", "claude-3-5-sonnet").
		WithAPIKey("{{ .env.ANTHROPIC_API_KEY }}").
		WithTemperature(0.2).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build fallback model: %w", err)
	}
	return []*enginecore.ProviderConfig{primary, fallback}, nil
}

// buildKnowledgeResources wires all knowledge builders: embedder, vector DB, sources, and base.
func buildKnowledgeResources(
	ctx context.Context,
) (*engineknowledge.EmbedderConfig, *engineknowledge.VectorDBConfig, *engineknowledge.BaseConfig, error) {
	embedderCfg, err := knowledge.NewEmbedder("docs-embedder", "openai", "text-embedding-3-large").
		WithAPIKey("{{ .env.OPENAI_API_KEY }}").
		WithDimension(3072).
		WithBatchSize(64).
		WithMaxConcurrentWorkers(8).
		Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build embedder: %w", err)
	}
	vectorCfg, err := knowledge.NewVectorDB("pgvector-store", knowledge.VectorDBType("pgvector")).
		WithDSN("{{ .env.PGVECTOR_DSN }}").
		WithPGVectorIndex("ivfflat", 200).
		WithPGVectorPool(2, 10).
		Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build vector db: %w", err)
	}
	fileSource, err := knowledge.NewFileSource("./README.md").Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build file source: %w", err)
	}
	dirSource, err := knowledge.NewDirectorySource("./docs", "./tasks").Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build directory source: %w", err)
	}
	urlSource, err := knowledge.NewURLSource("https://docs.compozy.dev/guides/orchestration").Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build url source: %w", err)
	}
	apiSource, err := knowledge.NewAPISource("github").Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build api source: %w", err)
	}
	baseCfg, err := knowledge.NewBase("product-knowledge").
		WithDescription("Unified view across documentation, issues, and release notes").
		WithEmbedder(embedderCfg.ID).
		WithVectorDB(vectorCfg.ID).
		AddSource(fileSource).
		AddSource(dirSource).
		AddSource(urlSource).
		AddSource(apiSource).
		WithChunking(knowledge.ChunkStrategy("recursive_text_splitter"), 800, 80).
		WithPreprocess(true, true).
		WithRetrieval(8, 0.7, 1200).
		WithIngestMode(knowledge.IngestOnStart).
		Build(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build knowledge base: %w", err)
	}
	return embedderCfg, vectorCfg, baseCfg, nil
}

// buildMemoryResources configures memory persistence, privacy, and references.
func buildMemoryResources(ctx context.Context) (*enginememory.Config, *memory.ReferenceConfig, error) {
	memCfg, err := memory.New("support-memory").
		WithTokenCounter("openai", "gpt-4o").
		WithMaxTokens(4000).
		WithSummarizationFlush("openai", "gpt-4o", 800).
		WithPrivacy(memory.PrivacyUserScope).
		WithExpiration(24 * time.Hour).
		WithPersistence(memory.PersistenceRedis).
		WithDistributedLocking(true).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build memory config: %w", err)
	}
	memRef, err := memory.NewReference(memCfg.ID).
		WithKey("conversation-{{ .input.customer.name }}").
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build memory reference: %w", err)
	}
	return memCfg, memRef, nil
}

// buildMCPServers configures both local and remote MCP servers.
func buildMCPServers(ctx context.Context) (*enginemcp.Config, *enginemcp.Config, error) {
	local, err := mcp.New("filesystem-mcp").
		WithCommand("mcp-server-filesystem").
		WithEnvVar("ROOT_DIR", "./").
		WithStartTimeout(5 * time.Second).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build local mcp: %w", err)
	}
	remote, err := mcp.New("github-mcp").
		WithURL("https://mcp.github.com/api/v1").
		WithHeader("Authorization", "Bearer {{ .env.GITHUB_TOKEN }}").
		WithProto("2025-03-26").
		WithMaxSessions(8).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build remote mcp: %w", err)
	}
	return local, remote, nil
}

// buildRuntimeAndTools configures bun runtime and native tool enablement.
func buildRuntimeAndTools(ctx context.Context) (*enginetool.Config, *engineruntime.Config, error) {
	nativeTools := runtime.NewNativeTools().
		WithCallAgents().
		WithCallWorkflows().
		Build(ctx)
	runtimeCfg, err := runtime.NewBun().
		WithEntrypoint("./sdk/examples/scripts/index.ts").
		WithBunPermissions("--allow-read=./", "--allow-net", "--allow-env").
		WithToolTimeout(45 * time.Second).
		WithNativeTools(nativeTools).
		WithMaxMemoryMB(2048).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build runtime config: %w", err)
	}
	inputSchema, outputSchema, err := buildToolSchemas(ctx)
	if err != nil {
		return nil, nil, err
	}
	toolCfg, err := tool.New("structured-summarizer").
		WithName("Structured Summarizer").
		WithDescription("Aggregates agent findings into JSON payloads").
		WithRuntime("bun").
		WithCode(`export default async function main({ items }) { return { summary: items.map((item) => ({ feature: item.feature, priority: item.priority, status: "triaged" })) }; }`).
		WithInput(inputSchema).
		WithOutput(outputSchema).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build tool config: %w", err)
	}
	return toolCfg, runtimeCfg, nil
}

// buildToolSchemas constructs JSON schemas using both Builder and PropertyBuilder APIs.
func buildToolSchemas(ctx context.Context) (*schema.Schema, *schema.Schema, error) {
	recordSchema := schema.NewObject().
		AddProperty("feature", schema.NewString().WithMinLength(3)).
		AddProperty("priority", schema.NewString().WithEnum("high", "medium", "low")).
		RequireProperty("feature").
		RequireProperty("priority")
	itemsProperty, err := schema.NewProperty("items").
		WithType("array").
		WithDescription("List of task records to normalize").
		WithItems(recordSchema).
		WithMinItems(1).
		WithMaxItems(25).
		Required().
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build tool input property: %w", err)
	}
	inputSchema := schema.NewObject().
		AddProperty(itemsProperty.Name, schema.NewArray(recordSchema)).
		RequireProperty("items")
	input, err := inputSchema.Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build tool input schema: %w", err)
	}
	outputSchema := schema.NewObject().
		AddProperty("summary", schema.NewArray(schema.NewObject().
			AddProperty("feature", schema.NewString()).
			AddProperty("priority", schema.NewString()).
			AddProperty("status", schema.NewString().WithEnum("triaged", "pending")).
			RequireProperty("feature").
			RequireProperty("status"),
		)).
		AddProperty("generated_at", schema.NewInteger()).
		RequireProperty("summary").
		RequireProperty("generated_at")
	output, err := outputSchema.Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build tool output schema: %w", err)
	}
	return input, output, nil
}

// buildAgentConfig wires knowledge, memory, tool, and MCP integrations onto an agent.
func buildAgentConfig(
	ctx context.Context,
	base *engineknowledge.BaseConfig,
	memRef *memory.ReferenceConfig,
	toolID string,
	localMCPID string,
	remoteMCPID string,
) (*engineagent.Config, error) {
	binding, err := knowledge.NewBinding(base.ID).
		WithTopK(5).
		WithMinScore(0.65).
		WithMaxTokens(1024).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build knowledge binding: %w", err)
	}
	actionInsights, err := agent.NewAction("collect-insights").
		WithPrompt("Synthesize the user's request, referencing available knowledge and MCP repositories.").
		WithTimeout(30 * time.Second).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build insight action: %w", err)
	}
	actionMCP, err := agent.NewAction("survey-repos").
		WithPrompt("Use MCPs to fetch release information relevant to the incident.").
		WithRetry(3, 2*time.Second).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build mcp action: %w", err)
	}
	actionDelegate, err := agent.NewAction("delegate-tool").
		WithPrompt("Summarize the consolidated findings into a triage-ready structure.").
		AddTool(toolID).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build delegate action: %w", err)
	}
	agentCfg, err := agent.New("orchestrator").
		WithInstructions("You are a senior incident manager coordinating analysis across knowledge, MCPs, and custom tools.").
		WithKnowledge(binding).
		WithMemory(memRef).
		AddAction(actionInsights).
		AddAction(actionMCP).
		AddAction(actionDelegate).
		AddMCP(localMCPID).
		AddMCP(remoteMCPID).
		AddTool(toolID).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build agent config: %w", err)
	}
	return agentCfg, nil
}

// buildAnalysisCoreTasks constructs the core analysis tasks (basic, parallel, aggregate).
func buildAnalysisCoreTasks(ctx context.Context, agentID string, toolID string) (*analysisCoreTasks, error) {
	collectTask, err := task.NewBasic("collect-insights-task").
		WithAgent(agentID).
		WithAction("collect-insights").
		WithInput(map[string]string{"customer": "{{ .input.customer }}"}).
		WithOutput("insights = {{ .result.output }}").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build collect task: %w", err)
	}
	mcpTask, err := task.NewBasic("survey-mcp-task").
		WithAgent(agentID).
		WithAction("survey-repos").
		WithInput(map[string]string{"targets": "{{ .input.targets }}"}).
		WithOutput("repos = {{ .result.output }}").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build mcp task: %w", err)
	}
	toolTask, err := task.NewBasic("delegate-tool-task").
		WithTool(toolID).
		WithInput(map[string]string{"items": "{{ merge (dict) .tasks.collect-insights-task.output .tasks.survey-mcp-task.output }}"}).
		WithOutput("summary = {{ .result.output.summary }}").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build tool task: %w", err)
	}
	parallelTask, err := task.NewParallel("parallel-analysis").
		AddTask(collectTask.ID).
		AddTask(mcpTask.ID).
		WithWaitAll(true).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build parallel task: %w", err)
	}
	aggregateTask, err := task.NewAggregate("aggregate-findings").
		AddTask(collectTask.ID).
		AddTask(mcpTask.ID).
		WithStrategy("merge").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build aggregate task: %w", err)
	}
	return &analysisCoreTasks{
		collect:   collectTask,
		mcp:       mcpTask,
		tool:      toolTask,
		parallel:  parallelTask,
		aggregate: aggregateTask,
	}, nil
}

// buildSupportTasks constructs memory, signal, wait, and router tasks.
func buildSupportTasks(
	ctx context.Context,
	memoryID string,
	aggregateID string,
	remoteMCPID string,
) (*supportTasks, error) {
	memoryTask, err := task.NewMemoryTask("append-memory").
		WithOperation("append").
		WithMemory(memoryID).
		WithContent("{{ toJson .tasks." + aggregateID + ".output }}").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build memory task: %w", err)
	}
	signalSend, err := task.NewSignal("signal-ready").
		WithSignalID("triage-ready").
		WithMode(task.SignalModeSend).
		WithData(map[string]any{"mcp": remoteMCPID}).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build signal send: %w", err)
	}
	signalWait, err := task.NewSignal("wait-on-feedback").
		WithSignalID("triage-feedback").
		WithMode(task.SignalModeWait).
		WithTimeout(30 * time.Second).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build signal wait: %w", err)
	}
	waitTask, err := task.NewWait("cooldown").
		WithDuration(2 * time.Second).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build wait task: %w", err)
	}
	routerTask, err := task.NewRouter("priority-router").
		WithCondition(`{{ eq (index .input.customer "tier") "enterprise" }}`).
		AddRoute("true", memoryTask.ID).
		WithDefault(waitTask.ID).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build router task: %w", err)
	}
	return &supportTasks{
		memory:     memoryTask,
		signalSend: signalSend,
		signalWait: signalWait,
		wait:       waitTask,
		router:     routerTask,
	}, nil
}

// buildCompositeTasks constructs the nested workflow, composite, and collection tasks.
func buildCompositeTasks(
	ctx context.Context,
	agentID string,
	toolID string,
	delegateTaskID string,
) (*compositeTasks, error) {
	subWorkflow, err := buildPostProcessWorkflow(ctx, agentID, toolID)
	if err != nil {
		return nil, err
	}
	compositeTask, err := task.NewComposite("post-process").
		WithWorkflow(subWorkflow.ID).
		WithInput(map[string]string{"items": "{{ .tasks." + delegateTaskID + ".output.summary }}"}).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build composite task: %w", err)
	}
	collectionTask, err := task.NewCollection("iterate-summary").
		WithCollection("{{ .tasks." + delegateTaskID + ".output.summary }}").
		WithTask(compositeTask.ID).
		WithItemVar("record").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build collection task: %w", err)
	}
	return &compositeTasks{subWorkflow: subWorkflow, composite: compositeTask, collection: collectionTask}, nil
}

// buildWorkflowSuite demonstrates every task builder and prepares schedules.
func buildWorkflowSuite(
	ctx context.Context,
	agentCfg *engineagent.Config,
	memCfg *enginememory.Config,
	toolID string,
	remoteMCPID string,
) ([]*engineworkflow.Config, []*engineschedule.Config, error) {
	coreTasks, err := buildAnalysisCoreTasks(ctx, agentCfg.ID, toolID)
	if err != nil {
		return nil, nil, err
	}
	supportSet, err := buildSupportTasks(ctx, memCfg.ID, coreTasks.aggregate.ID, remoteMCPID)
	if err != nil {
		return nil, nil, err
	}
	compositeSet, err := buildCompositeTasks(ctx, agentCfg.ID, toolID, coreTasks.tool.ID)
	if err != nil {
		return nil, nil, err
	}
	workflowInputSchema, err := buildWorkflowInputSchema(ctx)
	if err != nil {
		return nil, nil, err
	}
	mainWorkflow, err := workflow.New("complete-automation").
		WithDescription("Kitchen sink workflow combining every builder type").
		WithInput(workflowInputSchema).
		AddAgent(agentCfg).
		AddTask(coreTasks.parallel).
		AddTask(coreTasks.collect).
		AddTask(coreTasks.mcp).
		AddTask(coreTasks.aggregate).
		AddTask(coreTasks.tool).
		AddTask(supportSet.memory).
		AddTask(supportSet.signalSend).
		AddTask(supportSet.signalWait).
		AddTask(supportSet.wait).
		AddTask(supportSet.router).
		AddTask(compositeSet.composite).
		AddTask(compositeSet.collection).
		WithOutputs(map[string]string{
			"summary": "{{ .tasks." + coreTasks.tool.ID + ".output.summary }}",
			"signal":  "{{ .tasks." + supportSet.signalSend.ID + ".output }}",
		}).
		Build(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("build main workflow: %w", err)
	}
	schedules, err := buildProjectSchedules(ctx, mainWorkflow.ID)
	if err != nil {
		return nil, nil, err
	}
	return []*engineworkflow.Config{mainWorkflow, compositeSet.subWorkflow}, schedules, nil
}

// buildProjectSchedules creates daily and weekly schedules for the workflow.
func buildProjectSchedules(ctx context.Context, workflowID string) ([]*engineschedule.Config, error) {
	daily, err := schedule.New("daily-triage").
		WithDescription("Runs every weekday morning").
		WithCron("0 13 * * 1-5").
		WithTimezone("UTC").
		WithWorkflow(workflowID).
		WithInput(map[string]any{
			"customer": map[string]any{"name": "Daily Ops", "tier": "enterprise", "issue": "Automated triage review"},
			"targets":  []map[string]string{{"feature": "reporting", "priority": "medium"}},
		}).
		WithRetry(3, 10*time.Second).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build daily schedule: %w", err)
	}
	weekly, err := schedule.New("weekly-retro").
		WithDescription("Aggregates weekly learnings").
		WithCron("0 15 * * FR").
		WithWorkflow(workflowID).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build weekly schedule: %w", err)
	}
	return []*engineschedule.Config{daily, weekly}, nil
}

// buildWorkflowInputSchema constructs the workflow input JSON schema.
func buildWorkflowInputSchema(ctx context.Context) (*schema.Schema, error) {
	customerBuilder := schema.NewObject().
		AddProperty("name", schema.NewString().WithMinLength(2)).
		AddProperty("issue", schema.NewString().WithMinLength(5)).
		AddProperty("tier", schema.NewString().WithEnum("enterprise", "growth", "starter")).
		RequireProperty("name").
		RequireProperty("issue")
	targetBuilder := schema.NewObject().
		AddProperty("feature", schema.NewString().WithMinLength(2)).
		AddProperty("priority", schema.NewString().WithEnum("high", "medium", "low")).
		RequireProperty("feature").
		RequireProperty("priority")
	root := schema.NewObject().
		AddProperty("customer", customerBuilder).
		AddProperty("targets", schema.NewArray(targetBuilder)).
		RequireProperty("customer")
	return root.Build(ctx)
}

// buildPostProcessWorkflow constructs the nested workflow used by the composite task.
func buildPostProcessWorkflow(ctx context.Context, agentID string, toolID string) (*engineworkflow.Config, error) {
	validateTask, err := task.NewBasic("validate-entry").
		WithAgent(agentID).
		WithAction("collect-insights").
		WithInput(map[string]string{"record": "{{ .input.items }}"}).
		WithOutput("validated = {{ .result.output }}").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build validate task: %w", err)
	}
	refineTask, err := task.NewBasic("refine-summary").
		WithAgent(agentID).
		WithAction("delegate-tool").
		WithInput(map[string]string{"items": "{{ list .tasks.validate-entry.output.validated }}"}).
		WithOutput("refined = {{ .result.output.summary }}").
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build refine task: %w", err)
	}
	waitTask, err := task.NewWait("post-wait").
		WithDuration(1 * time.Second).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build post wait task: %w", err)
	}
	nested, err := workflow.New("post-process-workflow").
		WithDescription("Nested workflow transforming summary entries").
		AddAgent(&engineagent.Config{ID: agentID}).
		AddTask(validateTask).
		AddTask(refineTask).
		AddTask(waitTask).
		WithOutputs(map[string]string{"record": "{{ .tasks.refine-summary.output.refined }}"}).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build nested workflow: %w", err)
	}
	return nested, nil
}

// assembleProject registers all components on the project builder.
func assembleProject(
	ctx context.Context,
	models []*enginecore.ProviderConfig,
	workflows []*engineworkflow.Config,
	schedules []*engineschedule.Config,
	agentCfg *engineagent.Config,
	embedderCfg *engineknowledge.EmbedderConfig,
	vectorCfg *engineknowledge.VectorDBConfig,
	knowledgeBaseCfg *engineknowledge.BaseConfig,
	memCfg *enginememory.Config,
	localMCP *enginemcp.Config,
	remoteMCP *enginemcp.Config,
	toolCfg *enginetool.Config,
	runtimeCfg *engineruntime.Config,
) (*engineproject.Config, error) {
	builder := project.New("all-in-one").
		WithVersion("1.0.0").
		WithDescription("Complete reference project covering every SDK builder").
		WithAuthor("Kitchen Sink Team", "platform@example.com", "Compozy Labs")
	for _, modelCfg := range models {
		builder = builder.AddModel(modelCfg)
	}
	for _, wf := range workflows {
		builder = builder.AddWorkflow(wf)
	}
	builder = builder.AddAgent(agentCfg)
	for _, sched := range schedules {
		builder = builder.AddSchedule(sched)
	}
	projectCfg, err := builder.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("build project config: %w", err)
	}
	projectCfg.Embedders = append(projectCfg.Embedders, *embedderCfg)
	projectCfg.VectorDBs = append(projectCfg.VectorDBs, *vectorCfg)
	projectCfg.KnowledgeBases = append(projectCfg.KnowledgeBases, *knowledgeBaseCfg)
	projectCfg.Memories = append(projectCfg.Memories, *memCfg)
	projectCfg.MCPs = append(projectCfg.MCPs, *localMCP, *remoteMCP)
	projectCfg.Tools = append(projectCfg.Tools, *toolCfg)
	projectCfg.Runtime.RuntimeType = runtimeCfg.RuntimeType
	projectCfg.Runtime.EntrypointPath = runtimeCfg.EntrypointPath
	projectCfg.Runtime.BunPermissions = append([]string{}, runtimeCfg.BunPermissions...)
	if runtimeCfg.NativeTools != nil {
		projectCfg.Runtime.NativeTools.CallAgents = runtimeCfg.NativeTools.CallAgents
		projectCfg.Runtime.NativeTools.CallWorkflows = runtimeCfg.NativeTools.CallWorkflows
	}
	projectCfg.Opts.SourceOfTruth = "builder"
	projectCfg.MonitoringConfig = &projectmonitoring.Config{Enabled: true, Path: "/metrics"}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determine project cwd: %w", err)
	}
	if err := projectCfg.SetCWD(cwd); err != nil {
		return nil, fmt.Errorf("set project cwd: %w", err)
	}
	return projectCfg, nil
}

// prepareSDKClient demonstrates the client.Builder usage for completeness.
func prepareSDKClient(ctx context.Context) (*client.Client, error) {
	builder := client.New("http://127.0.0.1:18080").
		WithAPIKey("local-dev-key").
		WithTimeout(5 * time.Second)
	return builder.Build(ctx)
}
