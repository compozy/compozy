//go:build examples

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/knowledge"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/schema"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("knowledge RAG example failed", "error", err)
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
	filePath, dirPath, cleanup, err := prepareSampleContent(ctx)
	if err != nil {
		return fmt.Errorf("prepare sample content: %w", err)
	}
	defer cleanup()
	fileSource, dirSource, urlSource, err := buildSources(ctx, filePath, dirPath)
	if err != nil {
		return err
	}
	embedderCfg, err := buildEmbedder(ctx)
	if err != nil {
		return handleBuildError(ctx, "embedder", err)
	}
	vectorDBCfg, err := buildVectorDB(ctx)
	if err != nil {
		return handleBuildError(ctx, "vector_db", err)
	}
	kbCfg, err := buildKnowledgeBase(ctx, fileSource, dirSource, urlSource)
	if err != nil {
		return handleBuildError(ctx, "knowledge_base", err)
	}
	bindingCfg, err := buildKnowledgeBinding(ctx, kbCfg.ID)
	if err != nil {
		return handleBuildError(ctx, "knowledge_binding", err)
	}
	modelCfg, err := buildModel(ctx)
	if err != nil {
		return handleBuildError(ctx, "model", err)
	}
	actionCfg, err := buildAnswerAction(ctx)
	if err != nil {
		return handleBuildError(ctx, "action", err)
	}
	agentCfg, err := buildAgent(ctx, actionCfg, bindingCfg)
	if err != nil {
		return handleBuildError(ctx, "agent", err)
	}
	taskCfg, err := buildKnowledgeTask(ctx, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "task", err)
	}
	workflowCfg, err := buildWorkflow(ctx, agentCfg, taskCfg)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	projectCfg, err := buildProject(ctx, modelCfg, workflowCfg, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	registerKnowledgeResources(projectCfg, embedderCfg, vectorDBCfg, kbCfg, bindingCfg)
	log.Info("✅ knowledge system built", "knowledge_base", kbCfg.ID, "documents", len(kbCfg.Sources))
	printSummary(projectCfg, embedderCfg, vectorDBCfg, kbCfg)
	return nil
}

func prepareSampleContent(ctx context.Context) (string, string, func(), error) {
	baseDir, err := os.MkdirTemp("", "compozy-rag-example")
	if err != nil {
		return "", "", nil, fmt.Errorf("create temp directory: %w", err)
	}
	cleanup := func() {
		if removeErr := os.RemoveAll(baseDir); removeErr != nil {
			logger.FromContext(ctx).Warn("failed to clean up temp content", "error", removeErr)
		}
	}
	filePath := filepath.Join(baseDir, "release_notes.md")
	fileContent := "# Release Notes\n\nCompozy 2.0 introduces SDK-based knowledge orchestration."
	if err := os.WriteFile(filePath, []byte(fileContent), 0o600); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("write sample file: %w", err)
	}
	dirPath := filepath.Join(baseDir, "guides")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("create docs directory: %w", err)
	}
	dirContent := "# Getting Started\n\nUse the knowledge base to answer product questions efficiently."
	dirFile := filepath.Join(dirPath, "getting_started.md")
	if err := os.WriteFile(dirFile, []byte(dirContent), 0o600); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("write directory file: %w", err)
	}
	return filePath, dirPath, cleanup, nil
}

func buildSources(
	ctx context.Context,
	filePath, dirPath string,
) (*engineknowledge.SourceConfig, *engineknowledge.SourceConfig, *engineknowledge.SourceConfig, error) {
	// Stage 1: collect raw documents from multiple source types.
	fileSource, err := knowledge.NewFileSource(filePath).Build(ctx)
	if err != nil {
		return nil, nil, nil, handleBuildError(ctx, "file_source", err)
	}
	dirSource, err := knowledge.NewDirectorySource(dirPath).Build(ctx)
	if err != nil {
		return nil, nil, nil, handleBuildError(ctx, "directory_source", err)
	}
	urlSource, err := knowledge.NewURLSource(
		"https://docs.compozy.dev/overview", "https://docs.compozy.dev/rag-patterns",
	).Build(ctx)
	if err != nil {
		return nil, nil, nil, handleBuildError(ctx, "url_source", err)
	}
	return fileSource, dirSource, urlSource, nil
}

func buildEmbedder(ctx context.Context) (*engineknowledge.EmbedderConfig, error) {
	// Stage 2: encode text with an embedding model.
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		logger.FromContext(ctx).Warn("OPENAI_API_KEY is not set; embedding requests will fail against the provider")
	}
	return knowledge.NewEmbedder("openai-embedder", "openai", "text-embedding-3-small").
		WithAPIKey(apiKey).
		WithDimension(1536).
		WithBatchSize(100).
		WithMaxConcurrentWorkers(4).
		Build(ctx)
}

func buildVectorDB(ctx context.Context) (*engineknowledge.VectorDBConfig, error) {
	// Stage 3: store embeddings in a vector database.
	dsn := strings.TrimSpace(os.Getenv("PGVECTOR_DSN"))
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/compozy?sslmode=disable"
		logger.FromContext(ctx).Warn("PGVECTOR_DSN not set; using local default DSN for demonstration")
	}
	return knowledge.NewVectorDB("docs-db", engineknowledge.VectorDBTypePGVector).
		WithDSN(dsn).
		WithCollection("documentation").
		WithPGVectorIndex("hnsw", 100).
		WithPGVectorPool(5, 20).
		Build(ctx)
}

func buildKnowledgeBase(
	ctx context.Context,
	fileSource, dirSource, urlSource *engineknowledge.SourceConfig,
) (*engineknowledge.BaseConfig, error) {
	// Stage 4: orchestrate ingestion, chunking, and retrieval policies.
	return knowledge.NewBase("product-docs").
		WithDescription("Product documentation synthesized from local files and web sources").
		WithEmbedder("openai-embedder").
		WithVectorDB("docs-db").
		AddSource(fileSource).
		AddSource(dirSource).
		AddSource(urlSource).
		WithChunking(knowledge.ChunkStrategyRecursiveTextSplitter, 900, 150).
		WithPreprocess(true, true).
		WithIngestMode(knowledge.IngestModeOnStart).
		WithRetrieval(5, 0.72, 1800).
		Build(ctx)
}

func buildKnowledgeBinding(ctx context.Context, knowledgeBaseID string) (*knowledge.BindingConfig, error) {
	// Stage 5: bind retrieval parameters to agents at query time.
	return knowledge.NewBinding(knowledgeBaseID).
		WithTopK(3).
		WithMinScore(0.75).
		WithMaxTokens(1500).
		Build(ctx)
}

func buildModel(ctx context.Context) (*core.ProviderConfig, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	return model.New("openai", "gpt-4o-mini").
		WithAPIKey(apiKey).
		WithDefault(true).
		WithTemperature(0.3).
		Build(ctx)
}

func buildAnswerAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	output, err := schema.NewObject().
		AddProperty("answer", schema.NewString().WithDescription("Concise response grounded in retrieved documents").WithMinLength(1).Build(ctx)).
		RequireProperty("answer").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return agent.NewAction("answer-question").
		WithPrompt("Use the knowledge base to answer: {{ .input.question }}. Return the final answer in the `answer` field.").
		WithOutput(output).
		Build(ctx)
}

func buildAgent(
	ctx context.Context,
	actionCfg *engineagent.ActionConfig,
	binding *knowledge.BindingConfig,
) (*engineagent.Config, error) {
	if actionCfg == nil {
		return nil, fmt.Errorf("action config is required")
	}
	return agent.New("docs-assistant").
		WithInstructions("You ground every response in the indexed knowledge base and cite relevant sections when possible.").
		WithKnowledge(binding).
		AddAction(actionCfg).
		Build(ctx)
}

func buildKnowledgeTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	return task.NewBasic("answer-with-knowledge").
		WithAgent(agentCfg.ID).
		WithAction("answer-question").
		WithFinal(true).
		WithInput(map[string]string{"question": "{{ .input.question }}"}).
		WithOutput("answer = {{ .result.output.answer }}").
		Build(ctx)
}

func buildWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	taskCfg *enginetask.Config,
) (*engineworkflow.Config, error) {
	return workflow.New("knowledge-workflow").
		WithDescription("Answers user questions using a knowledge-enriched agent").
		AddAgent(agentCfg).
		AddTask(taskCfg).
		WithOutputs(map[string]string{"answer": "{{ task \"answer-with-knowledge\" \"answer\" }}"}).
		Build(ctx)
}

func buildProject(
	ctx context.Context,
	modelCfg *core.ProviderConfig,
	workflowCfg *engineworkflow.Config,
	agentCfg *engineagent.Config,
) (*engineproject.Config, error) {
	return project.New("rag-demo").
		WithVersion("1.0.0").
		WithDescription("Demonstrates RAG with Compozy knowledge builders").
		AddModel(modelCfg).
		AddWorkflow(workflowCfg).
		AddAgent(agentCfg).
		Build(ctx)
}

func registerKnowledgeResources(
	projectCfg *engineproject.Config,
	embedder *engineknowledge.EmbedderConfig,
	vectorDB *engineknowledge.VectorDBConfig,
	kb *engineknowledge.BaseConfig,
	binding *knowledge.BindingConfig,
) {
	if projectCfg == nil {
		return
	}
	if embedder != nil {
		projectCfg.Embedders = append(projectCfg.Embedders, *embedder)
	}
	if vectorDB != nil {
		projectCfg.VectorDBs = append(projectCfg.VectorDBs, *vectorDB)
	}
	if kb != nil {
		projectCfg.KnowledgeBases = append(projectCfg.KnowledgeBases, *kb)
	}
	if binding != nil {
		clone := binding.Clone()
		projectCfg.Knowledge = append(projectCfg.Knowledge, clone)
	}
}

func printSummary(
	projectCfg *engineproject.Config,
	embedder *engineknowledge.EmbedderConfig,
	vectorDB *engineknowledge.VectorDBConfig,
	kb *engineknowledge.BaseConfig,
) {
	if projectCfg == nil {
		return
	}
	fmt.Println("\nKnowledge RAG Example Summary")
	fmt.Printf("Project: %s (%s)\n", projectCfg.Name, projectCfg.Version)
	if embedder != nil {
		fmt.Printf("Embedder: %s (%s / %s)\n", embedder.ID, embedder.Provider, embedder.Model)
	}
	if vectorDB != nil {
		fmt.Printf("Vector DB: %s (%s collection=%s)\n", vectorDB.ID, vectorDB.Type, vectorDB.Config.Collection)
	}
	if kb != nil {
		fmt.Printf(
			"Knowledge Base: %s (sources=%d, chunk_size=%d, overlap=%d)\n",
			kb.ID,
			len(kb.Sources),
			kb.Chunking.Size,
			valueOrZero(kb.Chunking.Overlap),
		)
	}
	fmt.Println("Agent `docs-assistant` is bound to the knowledge base via project defaults.")
	fmt.Println("Use `go run ./sdk/examples/03_knowledge_rag.go` after configuring OPENAI_API_KEY and PGVECTOR_DSN.")
}

func valueOrZero(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
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
