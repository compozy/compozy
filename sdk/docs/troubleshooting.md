# SDK Troubleshooting Guide

## Quick Diagnostic Commands
- `go mod tidy` — Reconcile module definitions and resolve `cannot find module providing package` errors before building.
- `go list ./...` — Validate package graph after edits; combine with `-deps` to reveal missing indirect dependencies.
- `go test ./sdk/...` — Exercise builders end to end; failures surface validation misconfigurations early.
- `go vet ./sdk/...` — Catch suspicious constructs such as incorrect format verbs or shadowed variables in builder helpers.
- `golangci-lint run ./sdk/...` — Apply the project lint suite; required before merging as described in tasks/prd-sdk/07-testing-strategy.md.
- `gotestsum --format pkgname -- -race -parallel=4 ./sdk/...` — Run scoped tests with race detection when debugging concurrency-related issues.
- `COMPOZY_LOG_LEVEL=debug go run ./cmd/compozy -- diagnostics` — Produce verbose runtime logs when reproducing integration failures.

## Error Categories

### 1. Compilation Errors

#### `cannot find module providing package github.com/compozy/compozy/sdk/project`
- **Symptoms:** `go build` stops with the above message immediately after import statements.
- **Cause:** The workspace go.work file no longer references the `sdk` module; common after cloning without running the migration scripts from [Migration Basics](./migration-basics.md).
- **Resolution:**
  1. Run `go work sync` from repository root.
  2. Execute `go mod tidy` inside both the root and `sdk` modules.
- **Example fix:**
  ```bash
  go work use ./sdk
  go mod tidy
  ```

#### `undefined: task.NewBasic`
- **Symptoms:** Compiler reports `undefined: task.NewBasic` when building workflows copied from docs.
- **Cause:** The `sdk/task` import path is missing; importing `github.com/compozy/compozy/engine/task` instead of the SDK wrapper triggers this error.
- **Resolution:** Update imports to use `github.com/compozy/compozy/sdk/task` as highlighted in [Migration Basics](./migration-basics.md#context-setup-required-for-all-examples).
- **Example fix:**
  ```go
  import "github.com/compozy/compozy/sdk/task"
  ```

#### `go: module github.com/compozy/compozy@latest found (v1.13.0), but does not contain package github.com/compozy/compozy/sdk/runtime`
- **Symptoms:** `go get` pulls an older tag without the SDK folders.
- **Cause:** Replace directives were removed before the repository tag containing the SDK was published.
- **Resolution:** Pin the repository to the SDK-enabled tag and re-run `go mod tidy`.
- **Example fix:**
  ```bash
  go get github.com/compozy/compozy@sdk.0.0
  go mod tidy
  ```

#### `import cycle not allowed: github.com/compozy/compozy/sdk/project -> github.com/compozy/compozy/sdk/compozy -> github.com/compozy/compozy/sdk/project`
- **Symptoms:** `go build` fails after adding helper functions that mix builder creation and embedded lifecycle types.
- **Cause:** Helper package attempted to import both `sdk/project` and `sdk/compozy` in the same file; see dependency rules in tasks/prd-sdk/02-architecture.md.
- **Resolution:** Split lifecycle orchestration into a separate package to keep imports one-directional.
- **Example fix:** Move embedded lifecycle helpers to `sdk/embed` while builders stay in `sdk/project`.

#### `too many arguments in call to workflow.New`
- **Symptoms:** Compilation error referencing `workflow.New` arity.
- **Cause:** Code copied from pre-SDK release where builders accepted context directly; in the SDK, `Build(ctx)` applies validation.
- **Resolution:** Remove the `context.Context` parameter from constructor calls and pass context only to `Build(ctx)`.
- **Example fix:**
  ```go
  wf, err := workflow.New("process").AddTask(taskCfg).Build(ctx)
  ```

### 2. Validation Errors

#### `BuildError: project validation failed: id "" is invalid`
- **Symptoms:** Running `project.New("").Build(ctx)` returns a `BuildError` with accumulated messages.
- **Cause:** Missing project ID; see requirements in tasks/prd-sdk/06-migration-guide.md.
- **Resolution:** Provide a non-empty, slug-safe ID before calling `Build(ctx)`.
- **Example fix:**
  ```go
  proj, err := project.New("customer-support").Build(ctx)
  ```

#### `BuildError: workflow "support" requires at least one task`
- **Symptoms:** Validation fails when building workflows used for router-only orchestration.
- **Cause:** `workflow.Build(ctx)` enforces minimum task count; placeholder workflows must include a starter task as described in [Migration Advanced](./migration-advanced.md#hybrid-projects-with-autoload).
- **Resolution:** Add a lightweight `task.NewSignal` or `task.NewBasic` step before routing.
- **Example fix:**
  ```go
  start, _ := task.NewSignal("ready").WithMode(task.SignalModeSend).Build(ctx)
  wf, err := workflow.New("support").AddTask(start).Build(ctx)
  ```

#### `BuildError: duplicate agent id "dispatcher"`
- **Symptoms:** Project build reports duplicates even though agent builders live in different files.
- **Cause:** Builders reuse the same pointer from a shared setup function without cloning; Build(ctx) deep-clones but duplicate IDs remain.
- **Resolution:** Ensure each agent uses a unique ID or call `WithID` when deriving from templates.
- **Example fix:**
  ```go
  router := baseAgent.Clone().WithID("dispatcher-router")
  ```

#### `BuildError: knowledge base "docs" references missing embedder "openai_emb"`
- **Symptoms:** Build aggregates reference errors after reorganizing knowledge builders.
- **Cause:** Registration order changed; embedder was never added to the project before the knowledge base.
- **Resolution:** Register embedders and vector DBs before `knowledge.NewBase`. Cross-check with the dependency chart in tasks/prd-sdk/06-migration-guide.md.
- **Example fix:**
  ```go
  proj := project.New("kb").AddEmbedder(embedder).AddVectorDB(store).AddKnowledgeBase(base)
  ```

#### `BuildError: memory config conversation retention must be >= 1`
- **Symptoms:** Memory builder rejects zero retention values when porting YAML defaults.
- **Cause:** YAML allowed implicit defaults; SDK enforces explicit ranges as documented in tasks/prd-sdk/03-sdk-entities.md.
- **Resolution:** Provide a positive retention value or omit the override to use defaults.
- **Example fix:**
  ```go
  memoryCfg, _ := memory.New("session").WithRetention(30 * time.Minute).Build(ctx)
  ```

### 3. Context Errors

#### `panic: logger: value not found in context`
- **Symptoms:** Runtime panic when executing builders inside HTTP handlers.
- **Cause:** Middleware failed to attach logger using `logger.ContextWithLogger`; see context-first rules in tasks/prd-sdk/02-architecture.md.
- **Resolution:** Ensure request contexts inherit the startup context configured in [Migration Basics](./migration-basics.md#context-setup-required-for-all-examples).
- **Example fix:**
  ```go
  func middleware(next http.Handler) http.Handler {
      return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          ctx := logger.ContextWithLogger(r.Context(), log)
          next.ServeHTTP(w, r.WithContext(ctx))
      })
  }
  ```

#### `config: manager not found in context`
- **Symptoms:** Builders that read configuration values via `config.FromContext(ctx)` return errors.
- **Cause:** The setup helper omitted `config.ContextWithManager`; common when tests construct contexts manually.
- **Resolution:** Attach the configuration manager before invoking builders, reusing the helper from [Migration Basics](./migration-basics.md#context-setup-required-for-all-examples).
- **Example fix:**
  ```go
  ctx := config.ContextWithManager(baseCtx, mgr)
  ```

#### `go test` warning: `use of context.Background in tests`
- **Symptoms:** Lint fails with `testcontext` warnings when running `golangci-lint`.
- **Cause:** Tests call `context.Background()` instead of `t.Context()`; violates testing rules in tasks/prd-sdk/07-testing-strategy.md.
- **Resolution:** Replace call sites with `t.Context()` and thread through helper functions.
- **Example fix:**
  ```go
  func TestWorkflowBuilder(t *testing.T) {
      ctx := t.Context()
      wf, err := workflow.New("greet").AddTask(taskCfg).Build(ctx)
      // ...
  }
  ```

#### `logger.FromContext(ctx).Error` prints `logger=default`
- **Symptoms:** Logs show fallback logger without structured fields.
- **Cause:** Derived contexts discarded existing logger (e.g., using `context.Background()` in goroutines).
- **Resolution:** Always derive from the incoming context using `context.WithValue` alternatives like `logger.CloneWithFields` helpers instead of resetting.
- **Example fix:**
  ```go
  go func(parent context.Context) {
      ctx := logger.CloneWithFields(parent, "component", "scheduler")
      run(ctx)
  }(ctx)
  ```

### 4. Integration Errors

#### `pq: connection refused (SQLSTATE 08001)`
- **Symptoms:** Embedded Compozy startup fails when building runtime integrations.
- **Cause:** Database DSN in `compozy.New(...).WithDatabase()` points to an offline host; cross-reference integration checklist in tasks/prd-sdk/06-migration-guide.md.
- **Resolution:** Verify database service availability and credentials, then retry after running `docker compose up db` or equivalent.
- **Example fix:**
  ```bash
  docker compose up db
  ```

#### `temporal client: rpc error: code = Unavailable desc = connection closed`
- **Symptoms:** Workflows using Temporal transport fail on execution.
- **Cause:** Temporal endpoint mismatched namespace; the SDK builder defaults to `default` but the server expects a custom namespace.
- **Resolution:** Align namespace via `WithTemporal(address, namespace)` and confirm connectivity using `tctl namespace list`.
- **Example fix:**
  ```go
  app, err := compozy.New(proj).WithTemporal("localhost:7233", "default").Build(ctx)
  ```

#### `mcp transport handshake failed: missing bearer token`
- **Symptoms:** MCP commands return 401 when calling external tools.
- **Cause:** Token was not provided through `WithAuthToken` after migrating from YAML secrets; described in [Migration Advanced](./migration-advanced.md#advanced-feature-examples-examples-3-10).
- **Resolution:** Set the authentication token via builder options or environment-driven config.
- **Example fix:**
  ```go
  githubMCP, _ := mcp.New("github").WithAuthToken(os.Getenv("GITHUB_TOKEN")).Build(ctx)
  ```

#### `native tool "call_workflow" returned 404`
- **Symptoms:** Runtime native tool invocations fail when referencing workflows.
- **Cause:** Workflow ID differs between registration and invocation; often due to slug sanitization.
- **Resolution:** Use constants for IDs and confirm registration order; see workflow ID guidance in tasks/prd-sdk/03-sdk-entities.md.
- **Example fix:**
  ```go
  const workflowID = "generate-summary"
  ```

#### `redis: MOVED 3999 127.0.0.1:7002`
- **Symptoms:** Memory flush strategy tests fail when Redis cluster mode is enabled locally.
- **Cause:** SDK runtime expects a single-node Redis during development.
- **Resolution:** Point tests to the non-cluster endpoint or enable cluster support using the advanced memory example in [Migration Advanced](./migration-advanced.md#advanced-feature-examples-examples-3-10).
- **Example fix:**
  ```go
  app, _ := compozy.New(proj).WithRedis("redis://localhost:6379").Build(ctx)
  ```

### 5. Template Expression Errors

#### `template: input:1: unexpected "}" in operand`
- **Symptoms:** Build fails when setting task inputs with templated data.
- **Cause:** Unescaped braces copied from YAML; Go template engine enforces balanced delimiters.
- **Resolution:** Review expressions against the syntax table in tasks/prd-sdk/06-migration-guide.md and ensure each `{{` has a matching `}}`.
- **Example fix:**
  ```go
  WithInput(map[string]any{"prompt": "{{ .tasks.prepare.output }}"})
  ```

#### `template execution error: map has no entry for key "workflow"`
- **Symptoms:** Runtime error when executing workflow tasks.
- **Cause:** Template references nested data not present in the execution context; common when renaming tasks without updating expressions.
- **Resolution:** Use the inspector described in [Migration Advanced](./migration-advanced.md#advanced-feature-examples-examples-3-10) to inspect available keys, or log `BuildError` details.
- **Example fix:** Update template to `{{ .tasks.prepare.output.workflow }}` or adjust the producing task output structure.

#### `template parsing error: function "env" not defined`
- **Symptoms:** Templates copied from YAML rely on custom functions configured via the YAML runtime.
- **Cause:** SDK templates execute with the default function map unless you register helpers in code.
- **Resolution:** Register equivalent helper functions using `template.FuncMap` before rendering, or replace with standard Go template logic.
- **Example fix:**
  ```go
  tmpl := template.New("input").Funcs(template.FuncMap{"env": os.Getenv})
  ```

## Debugging Techniques
- Enable debug logging with `COMPOZY_LOG_LEVEL=debug` and inspect structured output using `logger.FromContext(ctx)` as documented in tasks/prd-sdk/02-architecture.md.
- Convert `BuildError` to rich details by iterating `errors.As(err, &buildErr)` and calling `buildErr.Errors()` to list root causes.
- Isolate builder validation by running `go test ./sdk/<package> -run Test<Builder>_Validation` as recommended in tasks/prd-sdk/07-testing-strategy.md.
- Use the embedded diagnostics CLI (`go run ./cmd/compozy -- diagnostics`) to confirm integration endpoints before executing workflows.
- Capture template rendering errors by enabling the inspector workflow introduced in [Migration Advanced](./migration-advanced.md#advanced-feature-examples-examples-3-10).

## Common Patterns & Best Practices
- Always store builder IDs in constants shared between registration and invocation to avoid typos in template expressions.
- Call `Build(ctx)` immediately after configuring each builder; avoid reusing partially built instances across goroutines.
- In tests, derive contexts from `t.Context()` and attach logger/config using the helper in [Migration Basics](./migration-basics.md#context-setup-required-for-all-examples).
- Register embedders, vector DBs, and knowledge bases in the order shown in tasks/prd-sdk/06-migration-guide.md to prevent missing reference errors.
- Keep `go.work` and `go.mod` synchronized using `go work sync` after branching or rebasing; mismatched workspace files are the root cause of most compilation issues recorded in support tickets.
- Before escalating to support, run the quick diagnostic commands—80% of reported issues resolve after `go mod tidy`, `go vet`, and `golangci-lint` cleanups as tracked in tasks/prd-sdk/07-testing-strategy.md.
