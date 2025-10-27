# Quick Start: YAML → SDK Migration

Accelerate migration of simple Compozy projects from YAML to the Go SDK. This quick reference covers the fundamental patterns for Examples 1-2, including context-first setup, basic project and workflow builders, and validation error handling. For advanced scenarios (hybrid deployments, knowledge, memory, MCP, runtime, and tooling), continue with the full guide in ../../tasks/prd-sdk/06-migration-guide.md.

## Context Setup (Required for All Examples)

All SDK builders require a `context.Context` carrying both logger and configuration. Attach them once at the edge of your application (CLI entrypoint, server bootstrap, test harness) using `logger.ContextWithLogger` and `config.ContextWithManager`, then call `Build(ctx)` downstream.

```go
package main

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.WithoutCancel(context.Background()))
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)

	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
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
```

**Key checks**

- Use the same `ctx` for every builder `Build(ctx)` call.
- Read configuration with `config.FromContext(ctx)` inside helpers and services.
- Retrieve loggers with `logger.FromContext(ctx)` for structured logging.

## 1. Simple Project

Maps migration guide Example 1 (project scaffolding) from YAML to the Go SDK. The Go sample is copy-paste ready and reuses the `initializeContext` helper above.

| Before (YAML)                                                                                                                                                                                                                                                                        | After (Go SDK)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `yaml<br># compozy.yaml<br>name: simple-demo<br>version: 1.0.0<br>description: Simple demo project<br><br>models:<br>  - provider: openai<br>    model: gpt-4<br>    api_key: "{{ .env.OPENAI_API_KEY }}"<br>    default: true<br><br>workflows:<br>  - source: ./workflow.yaml<br>` | `go<br>package main<br><br>import (<br>	"context"<br>	"errors"<br>	"fmt"<br>	"os"<br><br>	engineworkflow "github.com/compozy/compozy/engine/workflow"<br>	"github.com/compozy/compozy/pkg/config"<br>	"github.com/compozy/compozy/pkg/logger"<br>	"github.com/compozy/compozy/sdk/agent"<br>	internalerrors "github.com/compozy/compozy/sdk/internal/errors"<br>	"github.com/compozy/compozy/sdk/model"<br>	"github.com/compozy/compozy/sdk/project"<br>	"github.com/compozy/compozy/sdk/schema"<br>	"github.com/compozy/compozy/sdk/task"<br>	"github.com/compozy/compozy/sdk/workflow"<br>)<br><br>func main() {<br>	ctx, cleanup, err := initializeContext()<br>	if err != nil {<br>		panic(err)<br>	}<br>	defer cleanup()<br><br>	wf, err := buildGreetingWorkflow(ctx)<br>	if err != nil {<br>		panic(handleBuildError(ctx, "workflow", err))<br>	}<br><br>	mdl, err := model.New("openai", "gpt-4o-mini").<br>		WithAPIKey(os.Getenv("OPENAI_API_KEY")).<br>		WithDefault(true).<br>		Build(ctx)<br>	if err != nil {<br>		panic(handleBuildError(ctx, "model", err))<br>	}<br><br>	proj, err := project.New("simple-demo").<br>		WithVersion("1.0.0").<br>		WithDescription("Simple demo project").<br>		AddModel(mdl).<br>		AddWorkflow(wf).<br>		Build(ctx)<br>	if err != nil {<br>		panic(handleBuildError(ctx, "project", err))<br>	}<br><br>	log := logger.FromContext(ctx)<br>	log.Info("project configured", "name", proj.Name, "workflows", len(proj.Workflows))<br>}<br><br>func buildGreetingWorkflow(ctx context.Context) (*engineworkflow.Config, error) {<br>	greetingSchema := schema.NewString().<br>		WithDescription("Rendered greeting message").<br>		WithMinLength(1)<br>	outputSchema, err := schema.NewObject().<br>		AddProperty("greeting", greetingSchema).<br>		RequireProperty("greeting").<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	action, err := agent.NewAction("greet").<br>		WithPrompt("Greet: {{ .input.name }}").<br>		WithOutput(outputSchema).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	assistant, err := agent.New("assistant").<br>		WithInstructions("You are a helpful assistant.").<br>		AddAction(action).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	greetTask, err := task.NewBasic("greet_user").<br>		WithAgent("assistant").<br>		WithAction("greet").<br>		WithInput(map[string]string{"name": "{{ .input.name }}"}).<br>		WithFinal(true).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	return workflow.New("greeting").<br>		WithDescription("Simple greeting workflow").<br>		AddAgent(assistant).<br>		AddTask(greetTask).<br>		Build(ctx)<br>}<br><br>func handleBuildError(ctx context.Context, stage string, err error) error {<br>	if err == nil {<br>		return nil<br>	}<br>	var buildErr *internalerrors.BuildError<br>	if errors.As(err, &buildErr) {<br>		log := logger.FromContext(ctx)<br>		log.Error("validation failed", "stage", stage, "issues", len(buildErr.Errors))<br>		for idx, cause := range buildErr.Errors {<br>			log.Error("validation issue", "stage", stage, "index", idx+1, "error", cause)<br>		}<br>		return fmt.Errorf("%s configuration is invalid: %w", stage, err)<br>	}<br>	return err<br>}<br>` |

**Why it works**

- Models, workflows, and agents map directly from YAML sections to fluent builder calls.
- Environment variables stay accessible through `os.Getenv` and template expressions remain unchanged inside `WithInput`.
- Validation errors appear as a single aggregated `Build(ctx)` failure, logged via `handleBuildError`.

## 2. Workflow with Agent

Example 2 focuses on translating the workflow YAML into builder calls. The snippet below reuses `initializeContext` and `handleBuildError` from above.

| Before (YAML)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | After (Go SDK)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `yaml<br># workflow.yaml<br>id: greeting<br>description: Simple greeting workflow<br><br>agents:<br>  - id: assistant<br>    model: openai:gpt-4<br>    instructions: You are a helpful assistant.<br>    actions:<br>      - id: greet<br>        prompt: "Greet: {{ .input.name }}"<br>        output:<br>          type: object<br>          properties:<br>            greeting:<br>              type: string<br><br>tasks:<br>  - id: greet_user<br>    agent: assistant<br>    action: greet<br>    final: true<br>` | `go<br>package main<br><br>import (<br>	"context"<br><br>	engineworkflow "github.com/compozy/compozy/engine/workflow"<br>	"github.com/compozy/compozy/pkg/logger"<br>	"github.com/compozy/compozy/sdk/agent"<br>	"github.com/compozy/compozy/sdk/schema"<br>	"github.com/compozy/compozy/sdk/task"<br>	"github.com/compozy/compozy/sdk/workflow"<br>)<br><br>func buildGreetingWorkflow(ctx context.Context) (*engineworkflow.Config, error) {<br>	greetingSchema := schema.NewString().<br>		WithDescription("Rendered greeting message").<br>		WithMinLength(1)<br>	outputSchema, err := schema.NewObject().<br>		AddProperty("greeting", greetingSchema).<br>		RequireProperty("greeting").<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	action, err := agent.NewAction("greet").<br>		WithPrompt("Greet: {{ .input.name }}").<br>		WithOutput(outputSchema).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	assistant, err := agent.New("assistant").<br>		WithModelRef("openai:gpt-4").<br>		WithInstructions("You are a helpful assistant.").<br>		AddAction(action).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	greetTask, err := task.NewBasic("greet_user").<br>		WithAgent("assistant").<br>		WithAction("greet").<br>		WithFinal(true).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	cfg, err := workflow.New("greeting").<br>		WithDescription("Simple greeting workflow").<br>		AddAgent(assistant).<br>		AddTask(greetTask).<br>		Build(ctx)<br>	if err != nil {<br>		return nil, err<br>	}<br><br>	logger.FromContext(ctx).Info("workflow built", "id", cfg.ID, "tasks", len(cfg.Tasks))<br>	return cfg, nil<br>}<br>` |

**Notes**

- `WithModelRef("openai:gpt-4")` links the agent to the project model registered in Example 1.
- Schema builders mirror the YAML `output` definition with explicit type safety.
- Task wiring stays declarative while allowing Go conditionals or loops if you expand the workflow later.

## Common Patterns

- **Environment variables**: continue to use `os.Getenv` inside builder chains for secrets such as API keys.
- **Template expressions**: identical to YAML—embed `{{ }}` strings inside `WithInput`, `WithOutput`, or `WithPrompt`.
- **Context access**: whenever you need configuration or logging inside helpers, call `config.FromContext(ctx)` and `logger.FromContext(ctx)` to avoid global singletons.
- **Build(ctx) everywhere**: every builder requires `ctx` and returns either the engine configuration or a `BuildError`.
- **Reusable helpers**: split large builders into smaller functions (<50 lines) to keep validation localized.

## Validation Error Handling

Aggregated validation failures return `*internalerrors.BuildError`. Use `errors.As` to inspect each cause and surface friendly messages.

```go
package main

import (
	"context"
	"errors"
	"fmt"

	internalerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/project"
)

func createProject(ctx context.Context) error {
	_, err := project.New("").Build(ctx)
	if err == nil {
		return nil
	}
	var buildErr *internalerrors.BuildError
	if errors.As(err, &buildErr) {
		for idx, cause := range buildErr.Errors {
			fmt.Printf("validation %d: %v\n", idx+1, cause)
		}
		return fmt.Errorf("project is invalid: %w", err)
	}
	return err
}
```

## Troubleshooting

- **Import errors** (`cannot find package`): run `go get github.com/compozy/compozy/sdk@latest` followed by `go mod tidy` so the SDK module resolves and internal imports are available.
- **Validation failures on `Build(ctx)`**: inspect the aggregated `BuildError` (see section above) to list every invalid field, then update builder inputs accordingly.
- **Missing logger/config in context**: ensure `initializeContext()` (or equivalent) runs before calling any builder and that you pass the resulting `ctx` through your application; `nil` contexts trigger immediate build failures.
- **Template resolution issues**: template syntax matches YAML; double-check that `{{ .input.* }}` paths exist and that you pass data via `WithInput`.

## Next Steps

- Continue with advanced workflows, hybrid projects, RAG, memory, MCP, runtime, and tooling scenarios in the full migration guide: ../../tasks/prd-sdk/06-migration-guide.md.
- Explore runnable end-to-end examples under `sdk/examples` for more complex compositions and debugging strategies.
