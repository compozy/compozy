package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkagent "github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdkknowledge "github.com/compozy/compozy/sdk/v2/knowledge"
	sdkproject "github.com/compozy/compozy/sdk/v2/project"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunKnowledgeRag ingests markdown notes into a knowledge base and answers a question using retrieval.
func RunKnowledgeRag(ctx context.Context) error {
	_, err := ensureOpenAIKey(ctx)
	if err != nil {
		return err
	}
	dir, err := writeKnowledgeDocuments()
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	options, workflowID, err := buildKnowledgeOptions(ctx, dir)
	if err != nil {
		return err
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		question := "What are the two pillars of the Helios launch plan?"
		req := &compozy.ExecuteSyncRequest{Input: map[string]any{"question": question}}
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowID, req)
		if err != nil {
			return fmt.Errorf("execute workflow: %w", err)
		}
		answer := stringOutput(resp.Output, "answer")
		fmt.Printf("Knowledge grounded answer:\n%s\n", strings.TrimSpace(answer))
		logger.FromContext(execCtx).Info("knowledge workflow completed", "answer", answer)
		return nil
	})
}

func buildKnowledgeOptions(ctx context.Context, dir string) ([]compozy.Option, string, error) {
	key, err := ensureOpenAIKey(ctx)
	if err != nil {
		return nil, "", err
	}
	embedderCfg, vectorCfg, kbCfg, err := createKnowledgeDefinitions(ctx, dir, key)
	if err != nil {
		return nil, "", err
	}
	agentCfg, workflowCfg, err := createKnowledgeWorkflow(ctx, kbCfg)
	if err != nil {
		return nil, "", err
	}
	projectCfg, err := createKnowledgeProject(ctx, workflowCfg, embedderCfg, vectorCfg, kbCfg)
	if err != nil {
		return nil, "", err
	}
	options := []compozy.Option{
		compozy.WithProject(projectCfg),
		compozy.WithAgent(agentCfg),
		compozy.WithKnowledge(kbCfg),
		compozy.WithWorkflow(workflowCfg),
	}
	return options, workflowCfg.ID, nil
}

func createKnowledgeDefinitions(
	ctx context.Context,
	dir string,
	key string,
) (*engineknowledge.EmbedderConfig, *engineknowledge.VectorDBConfig, *engineknowledge.BaseConfig, error) {
	embedderCfg, err := sdkknowledge.NewEmbedder(ctx, "openai-notes", "openai", "text-embedding-3-small",
		sdkknowledge.WithAPIKey(key),
		sdkknowledge.WithDimension(1536),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create embedder: %w", err)
	}
	vectorCfg, err := sdkknowledge.NewVectorDB(ctx, "notes-vector", string(engineknowledge.VectorDBTypeFilesystem),
		sdkknowledge.WithPath(filepath.Join(dir, "notes.db")),
		sdkknowledge.WithVectorDBDimension(1536),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create vector db: %w", err)
	}
	overlap := 48
	kbCfg, err := sdkknowledge.NewBase(
		ctx,
		"launch-notes",
		sdkknowledge.WithEmbedder(embedderCfg.ID),
		sdkknowledge.WithVectorDB(vectorCfg.ID),
		sdkknowledge.WithIngest(engineknowledge.IngestOnStart),
		sdkknowledge.WithChunking(
			engineknowledge.ChunkingConfig{
				Strategy: engineknowledge.ChunkStrategyRecursiveTextSplitter,
				Size:     400,
				Overlap:  &overlap,
			},
		),
		sdkknowledge.WithSources([]engineknowledge.SourceConfig{knowledgeSource(filepath.Join(dir, "*.md"))}),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create knowledge base: %w", err)
	}
	return embedderCfg, vectorCfg, kbCfg, nil
}

func createKnowledgeWorkflow(
	ctx context.Context,
	kbCfg *engineknowledge.BaseConfig,
) (*engineagent.Config, *engineworkflow.Config, error) {
	model, err := newOpenAIModel(ctx, "gpt-4o-mini")
	if err != nil {
		return nil, nil, err
	}
	binding := enginecore.KnowledgeBinding{ID: kbCfg.ID, TopK: intPtr(3), MinScore: floatPtr(0.15)}
	agentCfg, err := newAgentWithModel(
		ctx,
		"knowledge-expert",
		"Answer using only supplied documents and cite sections by title.",
		model,
		sdkagent.WithKnowledge([]enginecore.KnowledgeBinding{binding}),
	)
	if err != nil {
		return nil, nil, err
	}
	taskCfg, err := sdktask.New(
		ctx,
		"answer-question",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt(
			"You are reviewing internal launch notes. Using the retrieved context, answer: {{ .workflow.input.question }}. "+
				"Cite section names in parentheses.",
		),
		sdktask.WithOutputs(inputPtr(map[string]any{"answer": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "knowledge-rag",
		sdkworkflow.WithDescription("Retrieves launch documentation and answers a question"),
		sdkworkflow.WithTasks([]enginetask.Config{*taskCfg}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{"answer": "{{ .tasks.answer-question.output.answer }}"})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create workflow: %w", err)
	}
	return agentCfg, workflowCfg, nil
}

func createKnowledgeProject(
	ctx context.Context,
	workflowCfg *engineworkflow.Config,
	embedderCfg *engineknowledge.EmbedderConfig,
	vectorCfg *engineknowledge.VectorDBConfig,
	kbCfg *engineknowledge.BaseConfig,
) (*engineproject.Config, error) {
	projectCfg, err := sdkproject.New(ctx, "knowledge-demo",
		sdkproject.WithWorkflows([]*engineproject.WorkflowSourceConfig{{Source: workflowCfg.ID}}),
		sdkproject.WithEmbedders([]engineknowledge.EmbedderConfig{*embedderCfg}),
		sdkproject.WithVectorDBs([]engineknowledge.VectorDBConfig{*vectorCfg}),
		sdkproject.WithKnowledgeBases([]engineknowledge.BaseConfig{*kbCfg}),
	)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return projectCfg, nil
}

func writeKnowledgeDocuments() (string, error) {
	dir, err := os.MkdirTemp("", "compozy-notes-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	content := strings.TrimSpace(`# Helios Launch Brief

## Strategy Pillars

1. Customer immersion weeks to gather qualitative insight.
2. Progressive rollout that starts with design partners before public launch.

## Rollout Timeline

- **July**: finalize onboarding scripts and training docs.
- **August**: private preview with five enterprise design partners.
- **September**: GA announcement with coordinated marketing campaign.

## Success Criteria

- Net Promoter Score above 45 from preview customers.
- Onboarding time under 30 minutes for new tenant.
`)
	filePath := filepath.Join(dir, "helios.md")
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write knowledge document: %w", err)
	}
	return dir, nil
}

func intPtr(v int) *int {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}
