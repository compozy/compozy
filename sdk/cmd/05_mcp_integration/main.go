package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/mcp"
	"github.com/compozy/compozy/sdk/project"
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
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("mcp integration example failed", "error", err)
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
	env := "unknown"
	if cfg := config.FromContext(ctx); cfg != nil {
		env = cfg.Runtime.Environment
	}
	log.Info("running MCP integration example", "environment", env)
	githubMCP, err := buildGitHubMCP(ctx)
	if err != nil {
		return handleBuildError(ctx, "github mcp", err)
	}
	filesystemMCP, err := buildFilesystemMCP(ctx)
	if err != nil {
		return handleBuildError(ctx, "filesystem mcp", err)
	}
	dockerMCP, err := buildDockerMCP(ctx)
	if err != nil {
		return handleBuildError(ctx, "docker mcp", err)
	}
	devAgent, err := buildDeveloperAgent(ctx)
	if err != nil {
		return handleBuildError(ctx, "agent", err)
	}
	reviewTask, err := buildReviewTask(ctx, devAgent)
	if err != nil {
		return handleBuildError(ctx, "task", err)
	}
	workflowCfg, err := buildWorkflowConfig(ctx, devAgent, reviewTask)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	projectCfg, err := buildProjectConfig(ctx, githubMCP, filesystemMCP, dockerMCP, devAgent, workflowCfg)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	printSummary(ctx, projectCfg, githubMCP, filesystemMCP, dockerMCP)
	return nil
}

func buildGitHubMCP(ctx context.Context) (*enginemcp.Config, error) {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		logger.FromContext(ctx).Warn("GITHUB_TOKEN is not set; GitHub MCP will reject authenticated routes")
	}
	builder := mcp.New("github-api")
	// Remote MCP uses SSE transport for streaming events and attaches auth headers.
	builder = builder.
		WithURL("https://api.github.com/mcp/v1").
		WithTransport(mcpproxy.TransportSSE).
		WithHeader("Authorization", "Bearer {{ .env.GITHUB_TOKEN }}").
		WithHeader("User-Agent", "compozy-mcp-example").
		WithProto("2025-03-26").
		WithMaxSessions(10)
	return builder.Build(ctx)
}

func buildFilesystemMCP(ctx context.Context) (*enginemcp.Config, error) {
	root := strings.TrimSpace(os.Getenv("MCP_FS_ROOT"))
	if root == "" {
		root = "/data"
	}
	builder := mcp.New("filesystem")
	// Local MCP runs via stdio transport and receives environment configuration.
	builder = builder.
		WithCommand("mcp-server-filesystem").
		WithTransport(mcpproxy.TransportStdio).
		WithEnvVar("ROOT_DIR", root).
		WithEnvVar("LOG_LEVEL", "info").
		WithStartTimeout(10 * time.Second)
	return builder.Build(ctx)
}

func buildDockerMCP(ctx context.Context) (*enginemcp.Config, error) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/compozy?sslmode=disable"
	}
	builder := mcp.New("postgres-db")
	// Docker MCP wraps a containerized server and still uses stdio transport.
	builder = builder.
		WithCommand("docker", "run", "--rm", "-i", "mcp-postgres:latest").
		WithTransport(mcpproxy.TransportStdio).
		WithEnvVar("DATABASE_URL", dsn).
		WithEnvVar("PGSSLMODE", "disable").
		WithStartTimeout(30 * time.Second)
	return builder.Build(ctx)
}

func buildDeveloperAgent(ctx context.Context) (*engineagent.Config, error) {
	action, err := agent.NewAction("review-change").
		WithPrompt("Summarize repository updates and call MCP tools when additional context is required.").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	// Agent references all MCP servers to access remote APIs, filesystem, and database helpers.
	return agent.New("developer-assistant").
		WithModel(defaultProvider, defaultModel).
		WithInstructions("You are a code assistant with access to GitHub, local files, and database diagnostics via MCP.").
		AddAction(action).
		AddMCP("github-api").
		AddMCP("filesystem").
		AddMCP("postgres-db").
		Build(ctx)
}

func buildReviewTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	return task.NewBasic("review-request").
		WithAgent(agentCfg.ID).
		WithAction("review-change").
		WithFinal(true).
		WithOutput("summary = {{ .result.output }}").
		Build(ctx)
}

func buildWorkflowConfig(
	ctx context.Context,
	agentCfg *engineagent.Config,
	taskCfg *enginetask.Config,
) (*engineworkflow.Config, error) {
	if agentCfg == nil || taskCfg == nil {
		return nil, fmt.Errorf("agent and task configs are required")
	}
	return workflow.New("developer-workflow").
		WithDescription("Aggregates MCP integrations for repository assistance").
		AddAgent(agentCfg).
		AddTask(taskCfg).
		WithOutputs(map[string]string{"summary": "{{ task \"review-request\" \"summary\" }}"}).
		Build(ctx)
}

func buildProjectConfig(
	ctx context.Context,
	githubMCP *enginemcp.Config,
	filesystemMCP *enginemcp.Config,
	dockerMCP *enginemcp.Config,
	agentCfg *engineagent.Config,
	workflowCfg *engineworkflow.Config,
) (*engineproject.Config, error) {
	projectBuilder := project.New("mcp-integration-demo").
		WithVersion("1.0.0").
		WithDescription("Project showcasing remote, local, and docker MCP integrations").
		AddAgent(agentCfg).
		AddWorkflow(workflowCfg)
	projectCfg, err := projectBuilder.Build(ctx)
	if err != nil {
		return nil, err
	}
	if githubMCP != nil {
		projectCfg.MCPs = append(projectCfg.MCPs, *githubMCP)
	}
	if filesystemMCP != nil {
		projectCfg.MCPs = append(projectCfg.MCPs, *filesystemMCP)
	}
	if dockerMCP != nil {
		projectCfg.MCPs = append(projectCfg.MCPs, *dockerMCP)
	}
	return projectCfg, nil
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

func printSummary(
	ctx context.Context,
	projectCfg *engineproject.Config,
	githubMCP *enginemcp.Config,
	filesystemMCP *enginemcp.Config,
	dockerMCP *enginemcp.Config,
) {
	if projectCfg == nil {
		return
	}
	log := logger.FromContext(ctx)
	fmt.Println("✅ MCP integration configured successfully")
	fmt.Printf("Project: %s\n", projectCfg.Name)
	fmt.Printf(
		"MCP servers: %d (remote: %s, local: %s, docker: %s)\n",
		len(projectCfg.MCPs),
		githubMCP.ID,
		filesystemMCP.ID,
		dockerMCP.ID,
	)
	fmt.Printf(
		"Remote transport: %s, max sessions: %d, headers: %d\n",
		githubMCP.Transport,
		githubMCP.MaxSessions,
		len(githubMCP.Headers),
	)
	fmt.Printf(
		"Local transport: %s, env vars: %d, timeout: %s\n",
		filesystemMCP.Transport,
		len(filesystemMCP.Env),
		filesystemMCP.StartTimeout,
	)
	fmt.Printf(
		"Docker transport: %s, env vars: %d, timeout: %s\n",
		dockerMCP.Transport,
		len(dockerMCP.Env),
		dockerMCP.StartTimeout,
	)
	if log != nil {
		log.Info("MCP integration summary emitted")
	}
}
