<role>
You are a senior Go backend engineer. You will implement four new native builtin tools (`cp__call_task`, `cp__call_tasks`, `cp__call_workflow`, and `cp__call_workflows`) following the exact same architectural patterns and best practices as the existing `cp__call_agent` and `cp__call_agents` implementations, adhering to all project standards and greenfield approach.
</role>

<dependent_tasks>
- Based on existing implementations:
  - `engine/tool/builtin/callagent/*` (single agent execution)
  - `engine/tool/builtin/callagents/*` (parallel agents execution)
  - `engine/agent/router/exec.go` (agent execution API)
  - `engine/agent/exec/runner.go` (agent runner)
- Follows patterns from:
  - `engine/task/router/exec.go` (task execution API)
  - `engine/workflow/router/execute.go` (workflow execution API)
  - `engine/worker/mod.go` (workflow triggering)
  - Native tool registration via `engine/tool/native/catalog.go`
</dependent_tasks>

<context>
- Currently, `cp__call_agent` and `cp__call_agents` enable orchestrators to dynamically invoke agents from workflows
- Tasks and workflows can be executed via HTTP API endpoints but cannot be called as native tools from within other workflows
- Need to enable the same capability for tasks and workflows as builtin tools
- This allows workflow-to-workflow and workflow-to-task orchestration patterns
- Both sync execution modes should be supported (parallel for batch operations)
</context>

<scope>
Greenfield implementation (no backwards compatibility needed):

**New Native Tools to Implement:**
1. `cp__call_task` - Execute a single task synchronously
2. `cp__call_tasks` - Execute multiple tasks in parallel
3. `cp__call_workflow` - Execute a single workflow synchronously
4. `cp__call_workflows` - Execute multiple workflows in parallel

**Backend Changes:**

**1. Tool Implementation Structure**
- Create `engine/tool/builtin/calltask/` package
  - `handler.go` - Main handler logic
  - `schema.go` - JSON schema definitions
  - `types.go` - Input/output types
  - `executor.go` - Task execution orchestration
  - `README.md` - Implementation notes
- Create `engine/tool/builtin/calltasks/` package
  - `handler.go` - Parallel execution handler
  - `schema.go` - JSON schema definitions
  - `types.go` - Input/output types
  - `executor.go` - Concurrent execution with semaphore
  - `README.md` - Implementation notes
- Create `engine/tool/builtin/callworkflow/` package
  - `handler.go` - Main handler logic
  - `schema.go` - JSON schema definitions
  - `types.go` - Input/output types
  - `executor.go` - Workflow execution orchestration
  - `README.md` - Implementation notes
- Create `engine/tool/builtin/callworkflows/` package
  - `handler.go` - Parallel execution handler
  - `schema.go` - JSON schema definitions
  - `types.go` - Input/output types
  - `executor.go` - Concurrent execution with semaphore
  - `README.md` - Implementation notes

**2. ToolEnv Interface Extensions**
- Add to `engine/runtime/toolenv/environment.go`:
  - `TaskExecutor() TaskExecutor` interface
  - `WorkflowExecutor() WorkflowExecutor` interface
- Create `engine/runtime/toolenv/task_executor.go`:
  - `TaskExecutor` interface with `ExecuteTask(ctx, TaskRequest) (*TaskResult, error)`
  - `TaskRequest` struct with fields: `TaskID`, `With`, `Timeout`
  - `TaskResult` struct with fields: `ExecID`, `Output`
- Create `engine/runtime/toolenv/workflow_executor.go`:
  - `WorkflowExecutor` interface with `ExecuteWorkflow(ctx, WorkflowRequest) (*WorkflowResult, error)`
  - `WorkflowRequest` struct with fields: `WorkflowID`, `Input`, `InitialTaskID`, `Timeout`
  - `WorkflowResult` struct with fields: `WorkflowExecID`, `Output`, `Status`

**3. Configuration Extension**
- Add to `pkg/config/native_tools.go`:
  - `NativeCallTaskConfig` struct (enabled, default_timeout)
  - `NativeCallTasksConfig` struct (enabled, default_timeout, max_concurrent)
  - `NativeCallWorkflowConfig` struct (enabled, default_timeout)
  - `NativeCallWorkflowsConfig` struct (enabled, default_timeout, max_concurrent)
- Update `NativeToolsConfig` struct with new fields
- Update `DefaultNativeToolsConfig()` with defaults

**4. Configuration Registry**
- Add to `pkg/config/definition/schema.go`:
  - Register fields for `runtime.native_tools.call_task.*`
  - Register fields for `runtime.native_tools.call_tasks.*`
  - Register fields for `runtime.native_tools.call_workflow.*`
  - Register fields for `runtime.native_tools.call_workflows.*`

**5. Task Executor Implementation**
- Create `engine/task/exec/runner.go` (if not exists or extend):
  - `Runner` struct implementing `toolenv.TaskExecutor`
  - `ExecuteTask(ctx, toolenv.TaskRequest) (*toolenv.TaskResult, error)`
  - `Prepare(ctx, ExecuteRequest) (*PreparedExecution, error)`
  - Reuse existing `DirectExecutor` pattern from `engine/task/router/exec.go`

**6. Workflow Executor Implementation**  
- Create `engine/workflow/exec/runner.go`:
  - `Runner` struct implementing `toolenv.WorkflowExecutor`
  - `ExecuteWorkflow(ctx, toolenv.WorkflowRequest) (*toolenv.WorkflowResult, error)`
  - Leverage `Worker.TriggerWorkflow()` and poll for completion
  - Support sync execution with timeout

**7. Native Tool Registration**
- Register providers in each package's `init()`:
  - `native.RegisterProvider(Definition)` in `calltask/handler.go`
  - `native.RegisterProvider(Definition)` in `calltasks/handler.go`
  - `native.RegisterProvider(Definition)` in `callworkflow/handler.go`
  - `native.RegisterProvider(Definition)` in `callworkflows/handler.go`

**8. Documentation**
- Create `docs/content/docs/core/tools/call-task.mdx`
- Create `docs/content/docs/core/tools/call-tasks.mdx`
- Create `docs/content/docs/core/tools/call-workflow.mdx`
- Create `docs/content/docs/core/tools/call-workflows.mdx`
- Update system prompts in `engine/llm/orchestrator/prompts/templates/system_prompt_with_builtins.tmpl`

**9. Testing**
- Create `engine/tool/builtin/calltask/handler_test.go`
- Create `engine/tool/builtin/calltasks/handler_test.go`
- Create `engine/tool/builtin/callworkflow/handler_test.go`
- Create `engine/tool/builtin/callworkflows/handler_test.go`
- Create integration tests in `test/integration/tool/`
</scope>

<input_output_specifications>

## cp__call_task

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "task_id": {
      "type": "string",
      "description": "Identifier of the task to execute. Must be a valid standalone task ID."
    },
    "with": {
      "type": "object",
      "additionalProperties": true,
      "description": "Structured input payload matching the task's input schema."
    },
    "timeout_ms": {
      "type": "integer",
      "minimum": 0,
      "description": "Optional timeout override in milliseconds for the task execution."
    }
  },
  "required": ["task_id"],
  "additionalProperties": false
}
```

**Output Schema:**
```json
{
  "type": "object",
  "required": ["success", "task_id", "exec_id"],
  "properties": {
    "success": { "type": "boolean" },
    "task_id": { "type": "string" },
    "exec_id": { "type": "string" },
    "output": {
      "type": "object",
      "additionalProperties": true
    },
    "duration_ms": { "type": "integer" }
  },
  "additionalProperties": true
}
```

## cp__call_tasks

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "tasks": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "task_id": { "type": "string" },
          "with": { 
            "type": "object",
            "additionalProperties": true
          },
          "timeout_ms": { 
            "type": "integer",
            "minimum": 0
          }
        },
        "required": ["task_id"],
        "additionalProperties": false
      },
      "description": "Ordered list of task execution requests."
    }
  },
  "required": ["tasks"],
  "additionalProperties": false
}
```

**Output Schema:**
```json
{
  "type": "object",
  "required": ["results", "total_count", "success_count", "failure_count", "total_duration_ms"],
  "properties": {
    "results": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "task_id": { "type": "string" },
          "exec_id": { "type": "string" },
          "output": { "type": "object" },
          "error": { "type": "string" },
          "duration_ms": { "type": "integer" }
        }
      }
    },
    "total_count": { "type": "integer" },
    "success_count": { "type": "integer" },
    "failure_count": { "type": "integer" },
    "total_duration_ms": { "type": "integer" }
  },
  "additionalProperties": false
}
```

## cp__call_workflow

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "workflow_id": {
      "type": "string",
      "description": "Identifier of the workflow to execute."
    },
    "input": {
      "type": "object",
      "additionalProperties": true,
      "description": "Input data passed to the workflow."
    },
    "initial_task_id": {
      "type": "string",
      "description": "Optional task ID to start execution from (instead of first task)."
    },
    "timeout_ms": {
      "type": "integer",
      "minimum": 0,
      "description": "Optional timeout override in milliseconds for workflow execution."
    }
  },
  "required": ["workflow_id"],
  "additionalProperties": false
}
```

**Output Schema:**
```json
{
  "type": "object",
  "required": ["success", "workflow_id", "workflow_exec_id"],
  "properties": {
    "success": { "type": "boolean" },
    "workflow_id": { "type": "string" },
    "workflow_exec_id": { "type": "string" },
    "status": { 
      "type": "string",
      "enum": ["SUCCESS", "FAILED", "TIMED_OUT", "CANCELED"]
    },
    "output": {
      "type": "object",
      "additionalProperties": true
    },
    "duration_ms": { "type": "integer" }
  },
  "additionalProperties": true
}
```

## cp__call_workflows

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "workflows": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "workflow_id": { "type": "string" },
          "input": { 
            "type": "object",
            "additionalProperties": true
          },
          "initial_task_id": { "type": "string" },
          "timeout_ms": { 
            "type": "integer",
            "minimum": 0
          }
        },
        "required": ["workflow_id"],
        "additionalProperties": false
      },
      "description": "Ordered list of workflow execution requests."
    }
  },
  "required": ["workflows"],
  "additionalProperties": false
}
```

**Output Schema:**
```json
{
  "type": "object",
  "required": ["results", "total_count", "success_count", "failure_count", "total_duration_ms"],
  "properties": {
    "results": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "workflow_id": { "type": "string" },
          "workflow_exec_id": { "type": "string" },
          "status": { "type": "string" },
          "output": { "type": "object" },
          "error": { "type": "string" },
          "duration_ms": { "type": "integer" }
        }
      }
    },
    "total_count": { "type": "integer" },
    "success_count": { "type": "integer" },
    "failure_count": { "type": "integer" },
    "total_duration_ms": { "type": "integer" }
  },
  "additionalProperties": false
}
```

</input_output_specifications>

<usage_examples>

## cp__call_task Example

**Direct task invocation:**
```json
{
  "task_id": "data-processor",
  "with": {
    "dataset": "customer_records",
    "operation": "transform"
  }
}
```

**With timeout override:**
```json
{
  "task_id": "long-running-batch",
  "with": {
    "batch_size": 1000
  },
  "timeout_ms": 120000
}
```

## cp__call_tasks Example

**Parallel task execution:**
```json
{
  "tasks": [
    {
      "task_id": "validate-email",
      "with": { "email": "user@example.com" }
    },
    {
      "task_id": "validate-phone",
      "with": { "phone": "+1234567890" }
    },
    {
      "task_id": "check-duplicates",
      "with": { "user_id": "12345" }
    }
  ]
}
```

## cp__call_workflow Example

**Execute workflow with input:**
```json
{
  "workflow_id": "user-onboarding",
  "input": {
    "user_data": {
      "name": "John Doe",
      "email": "john@example.com"
    }
  }
}
```

**With initial task override:**
```json
{
  "workflow_id": "multi-stage-pipeline",
  "input": {
    "resume_from": "checkpoint_5"
  },
  "initial_task_id": "stage-3-processor",
  "timeout_ms": 180000
}
```

## cp__call_workflows Example

**Parallel workflow orchestration:**
```json
{
  "workflows": [
    {
      "workflow_id": "process-region-us",
      "input": { "region": "us-east-1", "dataset": "daily" }
    },
    {
      "workflow_id": "process-region-eu",
      "input": { "region": "eu-west-1", "dataset": "daily" }
    },
    {
      "workflow_id": "process-region-asia",
      "input": { "region": "ap-south-1", "dataset": "daily" }
    }
  ]
}
```

</usage_examples>

<implementation_requirements>

## Code Organization

**Follow exact pattern from callagent/callagents:**

1. **Package Structure** (each tool):
   ```
   engine/tool/builtin/<tool_name>/
   ├── handler.go       # Main handler, processRequest, validation
   ├── schema.go        # Input/output schemas
   ├── types.go         # Request/response types
   ├── executor.go      # Execution logic (for parallel tools)
   ├── README.md        # Implementation notes
   ```

2. **Handler Structure** (`handler.go`):
   - Constant `toolID` (e.g., `"cp__call_task"`)
   - `init()` function calling `native.RegisterProvider(Definition)`
   - `Definition(env toolenv.Environment) builtin.BuiltinDefinition`
   - `newHandler(env toolenv.Environment) builtin.Handler`
   - `processRequest(ctx, env, payload) (core.Output, string, int, string, error)`
   - Input decode with `mapstructure`
   - Request building and validation
   - Executor invocation
   - Output construction
   - Telemetry recording with `builtin.RecordInvocation()`

3. **Schema Structure** (`schema.go`):
   - `var inputSchema = schema.Schema{ ... }`
   - `var outputSchema = schema.Schema{ ... }`
   - Helper functions (commented out): `buildInputSchema()`, `buildOutputSchema()`

4. **Types Structure** (`types.go`):
   - `handlerInput` struct with json/mapstructure tags
   - Request/result structures
   - For parallel tools: execution plan types

5. **Executor Structure** (`executor.go` - only for *_tasks/*_workflows):
   - Parallel execution with `sync.WaitGroup.Go()`
   - Concurrency control with `golang.org/x/sync/semaphore.Weighted`
   - Per-execution telemetry with `builtin.RecordStep()`
   - Error recovery and structured failure handling
   - Result aggregation

## Configuration Standards

**Follow pattern from CallAgent/CallAgents:**

1. **Config Structs** (`pkg/config/native_tools.go`):
   ```go
   type NativeCallTaskConfig struct {
       Enabled        bool          `koanf:"enabled" json:"enabled" yaml:"enabled" mapstructure:"enabled"`
       DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout"`
   }

   type NativeCallTasksConfig struct {
       Enabled        bool          `koanf:"enabled" json:"enabled" yaml:"enabled" mapstructure:"enabled"`
       DefaultTimeout time.Duration `koanf:"default_timeout" json:"default_timeout" yaml:"default_timeout" mapstructure:"default_timeout" validate:"min=0"`
       MaxConcurrent  int           `koanf:"max_concurrent" json:"max_concurrent" yaml:"max_concurrent" mapstructure:"max_concurrent" validate:"min=0"`
   }
   ```

2. **Defaults**:
   - `enabled: true`
   - `default_timeout: 60s`
   - `max_concurrent: 10` (for parallel tools)

3. **Registry Entries** (`pkg/config/definition/schema.go`):
   - Path: `runtime.native_tools.call_task.enabled`
   - Path: `runtime.native_tools.call_task.default_timeout`
   - Path: `runtime.native_tools.call_tasks.enabled`
   - Path: `runtime.native_tools.call_tasks.default_timeout`
   - Path: `runtime.native_tools.call_tasks.max_concurrent`
   - (Same for call_workflow and call_workflows)

## ToolEnv Interface Extensions

**Create new executor interfaces:**

1. **TaskExecutor** (`engine/runtime/toolenv/task_executor.go`):
   ```go
   type TaskRequest struct {
       TaskID    string
       With      core.Input
       Timeout   time.Duration
   }

   type TaskResult struct {
       ExecID core.ID
       Output *core.Output
   }

   type TaskExecutor interface {
       ExecuteTask(ctx context.Context, req TaskRequest) (*TaskResult, error)
   }
   ```

2. **WorkflowExecutor** (`engine/runtime/toolenv/workflow_executor.go`):
   ```go
   type WorkflowRequest struct {
       WorkflowID     string
       Input          *core.Input
       InitialTaskID  string
       Timeout        time.Duration
   }

   type WorkflowResult struct {
       WorkflowExecID core.ID
       Output         *core.Output
       Status         core.StatusType
   }

   type WorkflowExecutor interface {
       ExecuteWorkflow(ctx context.Context, req WorkflowRequest) (*WorkflowResult, error)
   }
   ```

3. **Environment Interface Update** (`engine/runtime/toolenv/environment.go`):
   ```go
   type Environment interface {
       AgentExecutor() AgentExecutor
       TaskExecutor() TaskExecutor       // NEW
       WorkflowExecutor() WorkflowExecutor // NEW
   }
   ```

## Executor Implementations

**Task Executor** (`engine/task/exec/runner.go`):
- Struct `Runner` implementing `toolenv.TaskExecutor`
- Method `ExecuteTask(ctx, toolenv.TaskRequest) (*toolenv.TaskResult, error)`
- Leverage existing `DirectExecutor` from `engine/task/router/exec.go`
- Validation, config loading, timeout handling
- Similar pattern to `engine/agent/exec/runner.go`

**Workflow Executor** (`engine/workflow/exec/runner.go`):
- Struct `Runner` implementing `toolenv.WorkflowExecutor`
- Method `ExecuteWorkflow(ctx, toolenv.WorkflowRequest) (*toolenv.WorkflowResult, error)`
- Use `Worker.TriggerWorkflow()` to start execution
- Poll workflow state repository for completion
- Support sync execution with timeout
- Handle workflow statuses (SUCCESS, FAILED, TIMED_OUT, CANCELED)

## Error Handling

**Follow builtin error patterns:**

1. **Error Codes**:
   - `builtin.CodeInvalidArgument` - Missing/invalid input
   - `builtin.CodeNotFound` - Task/workflow not found
   - `builtin.CodePermissionDenied` - Tool disabled
   - `builtin.CodeDeadlineExceeded` - Execution timeout
   - `builtin.CodeInternal` - Internal errors

2. **Error Metadata**:
   - `remediation_hint` - User-friendly guidance
   - `task_id` / `workflow_id` - Context for errors

3. **Structured Errors**:
   ```go
   return nil, status, 0, builtin.CodeInvalidArgument, builtin.InvalidArgument(
       errors.New("task_id is required"),
       map[string]any{
           "remediation_hint": "Provide a valid task_id to execute.",
       },
   )
   ```

## Telemetry & Metrics

**Follow existing patterns:**

1. **Invocation Recording**:
   ```go
   defer func() {
       builtin.RecordInvocation(
           ctx,
           toolID,
           builtin.RequestIDFromContext(ctx),
           status,
           time.Since(start),
           responseBytes,
           errorCode,
       )
   }()
   ```

2. **Step Recording** (parallel tools):
   ```go
   builtin.RecordStep(
       ctx,
       toolID,
       stepType,
       builtin.RequestIDFromContext(ctx),
       identifier,
       stepStatus,
       time.Since(stepStart),
   )
   ```

## Concurrency Pattern (for cp__call_tasks and cp__call_workflows)

**Follow cp__call_agents pattern exactly:**

1. **Use `sync.WaitGroup.Go()`** for goroutine management:
   ```go
   var wg sync.WaitGroup
   for i, plan := range plans {
       wg.Go(func() {
           // Acquire semaphore
           // Execute task/workflow
           // Release semaphore
       })
   }
   wg.Wait()
   ```

2. **Use `semaphore.Weighted`** for concurrency control:
   ```go
   sem := semaphore.NewWeighted(int64(maxConcurrent))
   sem.Acquire(ctx, 1)
   defer sem.Release(1)
   ```

3. **Panic Recovery**:
   ```go
   defer func() {
       if r := recover(); r != nil {
           log.Error("panic in execution", "panic", r)
           results[idx] = buildErrorResult(...)
       }
   }()
   ```

4. **Context Propagation**:
   - Each goroutine gets `context.WithTimeout()` derived from parent
   - Timeout per-execution or use default from config

</implementation_requirements>

<acceptance_criteria>

**Core Implementation:**
- ✅ Four new native tools registered and discoverable
- ✅ All tools follow exact architectural patterns from callagent/callagents
- ✅ Configuration properly integrated with registry and defaults
- ✅ ToolEnv interfaces extended with TaskExecutor and WorkflowExecutor
- ✅ Task and Workflow executors implemented
- ✅ Input/output schemas validated with JSON Schema
- ✅ Error handling with proper codes and remediation hints

**Functionality:**
- ✅ cp__call_task executes standalone tasks synchronously
- ✅ cp__call_tasks executes multiple tasks in parallel with semaphore
- ✅ cp__call_workflow triggers workflows and waits for completion
- ✅ cp__call_workflows executes multiple workflows in parallel
- ✅ Timeout handling works correctly for all tools
- ✅ Failures don't stop parallel executions (fail-fast disabled)
- ✅ Results returned in input order for parallel tools

**Telemetry:**
- ✅ `builtin.RecordInvocation()` called for all executions
- ✅ `builtin.RecordStep()` called for parallel execution steps
- ✅ Response byte counts tracked accurately
- ✅ Duration metrics captured

**Configuration:**
- ✅ Enabled flags work correctly (tools disabled when false)
- ✅ Default timeouts applied when not specified
- ✅ Max concurrent limits enforced for parallel tools
- ✅ Config validation prevents negative values

**Documentation:**
- ✅ All four MDX documentation files created
- ✅ System prompts updated with new tool descriptions
- ✅ README files in each package with implementation notes
- ✅ Usage examples provided for all tools

**Testing:**
- ✅ Unit tests for all handlers
- ✅ Integration tests for execution flows
- ✅ `make lint` passes
- ✅ `make test` passes
- ✅ Error cases covered (not found, timeout, validation)
- ✅ Parallel execution edge cases tested

</acceptance_criteria>

<suggested_steps>

## Phase 1: Infrastructure & Interfaces (Foundational)

### Step 1.1: ToolEnv Interface Extensions
1. Create `engine/runtime/toolenv/task_executor.go`:
   - Define `TaskRequest`, `TaskResult`, `TaskExecutor` interface
2. Create `engine/runtime/toolenv/workflow_executor.go`:
   - Define `WorkflowRequest`, `WorkflowResult`, `WorkflowExecutor` interface
3. Update `engine/runtime/toolenv/environment.go`:
   - Add `TaskExecutor()` method to `Environment` interface
   - Add `WorkflowExecutor()` method to `Environment` interface
4. Update `engine/runtime/toolenv/env.go`:
   - Add fields for task and workflow executors
   - Implement getter methods

### Step 1.2: Configuration Setup
1. Update `pkg/config/native_tools.go`:
   - Add `NativeCallTaskConfig` struct
   - Add `NativeCallTasksConfig` struct
   - Add `NativeCallWorkflowConfig` struct
   - Add `NativeCallWorkflowsConfig` struct
   - Add fields to `NativeToolsConfig`
   - Update `DefaultNativeToolsConfig()` with defaults
2. Update `pkg/config/definition/schema.go`:
   - Register `runtime.native_tools.call_task.*` fields
   - Register `runtime.native_tools.call_tasks.*` fields
   - Register `runtime.native_tools.call_workflow.*` fields
   - Register `runtime.native_tools.call_workflows.*` fields
3. Update `pkg/config/config.go`:
   - Add to builder functions to map registry to typed structs

## Phase 2: Executor Implementations

### Step 2.1: Task Executor
1. Create `engine/task/exec/` directory if not exists
2. Create `engine/task/exec/runner.go`:
   - `Runner` struct with state, repo, store dependencies
   - `NewRunner(state, repo, store) *Runner` constructor
   - `ExecuteTask(ctx, toolenv.TaskRequest) (*toolenv.TaskResult, error)` method
   - Leverage `DirectExecutor` from `engine/task/router/exec.go`
   - Validation, config resolution, timeout handling
   - Follow `engine/agent/exec/runner.go` as reference
3. Create `engine/task/exec/types.go`:
   - `ExecuteRequest` struct for internal use
   - Helper types
4. Test runner implementation

### Step 2.2: Workflow Executor
1. Create `engine/workflow/exec/` directory
2. Create `engine/workflow/exec/runner.go`:
   - `Runner` struct with worker, repo dependencies
   - `NewRunner(worker, repo) *Runner` constructor
   - `ExecuteWorkflow(ctx, toolenv.WorkflowRequest) (*toolenv.WorkflowResult, error)` method
   - Use `Worker.TriggerWorkflow()` to start
   - Implement polling mechanism for sync execution
   - Handle workflow completion statuses
   - Timeout handling with context
3. Create `engine/workflow/exec/poller.go`:
   - `pollWorkflowCompletion(ctx, repo, workflowExecID, pollInterval, timeout) (*State, error)`
   - Poll repository until workflow reaches terminal state
   - Return state or timeout error
4. Test workflow executor implementation

### Step 2.3: Wire Executors to ToolEnv
1. Update `engine/runtime/toolenv/env.go`:
   - Initialize task executor in `New()`
   - Initialize workflow executor in `New()`
   - Store references
2. Update relevant state initialization code

## Phase 3: Native Tool Implementation - cp__call_task

### Step 3.1: Package Setup
1. Create `engine/tool/builtin/calltask/` directory
2. Create `engine/tool/builtin/calltask/types.go`:
   ```go
   type handlerInput struct {
       TaskID    string         `json:"task_id"    mapstructure:"task_id"`
       With      map[string]any `json:"with"       mapstructure:"with"`
       TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
   }
   ```

### Step 3.2: Schema Definition
1. Create `engine/tool/builtin/calltask/schema.go`:
   - Define `inputSchema` with proper validation
   - Define `outputSchema` for response structure
   - Follow callagent schema pattern

### Step 3.3: Handler Implementation
1. Create `engine/tool/builtin/calltask/handler.go`:
   - Constant `toolID = "cp__call_task"`
   - `init()` with `native.RegisterProvider(Definition)`
   - `Definition(env) builtin.BuiltinDefinition`
   - `newHandler(env) builtin.Handler`
   - `processRequest(ctx, env, payload) (core.Output, string, int, string, error)`
     - Check if task executor is available
     - Check if tool is enabled in config
     - Decode input with mapstructure
     - Build TaskRequest
     - Call executor.ExecuteTask()
     - Handle errors with proper codes
     - Build output response
     - Record invocation metrics
   - `decodeHandlerInput(payload) (handlerInput, string, error)`
   - `buildTaskRequest(cfg, input) (toolenv.TaskRequest, string, error)`
   - `buildOutput(req, res) core.Output`

### Step 3.4: Documentation & Testing
1. Create `engine/tool/builtin/calltask/README.md`:
   - Implementation notes
   - Configuration reference
   - Usage patterns
2. Create `engine/tool/builtin/calltask/handler_test.go`:
   - Test successful execution
   - Test validation errors
   - Test timeout handling
   - Test disabled tool
   - Test not found errors

## Phase 4: Native Tool Implementation - cp__call_tasks

### Step 4.1: Package Setup
1. Create `engine/tool/builtin/calltasks/` directory
2. Create `engine/tool/builtin/calltasks/types.go`:
   ```go
   type TaskExecutionRequest struct {
       TaskID    string         `json:"task_id"    mapstructure:"task_id"`
       With      map[string]any `json:"with"       mapstructure:"with"`
       TimeoutMs int            `json:"timeout_ms" mapstructure:"timeout_ms"`
   }

   type handlerInput struct {
       Tasks []TaskExecutionRequest `json:"tasks" mapstructure:"tasks"`
   }

   type TaskExecutionResult struct {
       Success    bool           `json:"success"`
       TaskID     string         `json:"task_id"`
       ExecID     string         `json:"exec_id,omitempty"`
       Output     map[string]any `json:"output,omitempty"`
       Error      string         `json:"error,omitempty"`
       DurationMs int64          `json:"duration_ms"`
   }

   type taskPlan struct {
       index      int
       request    toolenv.TaskRequest
       userConfig TaskExecutionRequest
   }
   ```

### Step 4.2: Schema Definition
1. Create `engine/tool/builtin/calltasks/schema.go`:
   - Define `inputSchema` for array of task requests
   - Define `outputSchema` with results array and summary

### Step 4.3: Handler Implementation
1. Create `engine/tool/builtin/calltasks/handler.go`:
   - Similar structure to callagents
   - Check max_concurrent limit
   - Build execution plans
   - Delegate to executor

### Step 4.4: Executor Implementation
1. Create `engine/tool/builtin/calltasks/executor.go`:
   - `executeTasksParallel(ctx, env, plans, maxConcurrent) []TaskExecutionResult`
   - Use `sync.WaitGroup.Go()` pattern
   - Use `semaphore.Weighted` for concurrency control
   - Per-task telemetry with `builtin.RecordStep()`
   - Panic recovery
   - Return results in input order

### Step 4.5: Documentation & Testing
1. Create `engine/tool/builtin/calltasks/README.md`
2. Create `engine/tool/builtin/calltasks/handler_test.go`
3. Test parallel execution
4. Test concurrency limits
5. Test partial failures

## Phase 5: Native Tool Implementation - cp__call_workflow

### Step 5.1: Package Setup
1. Create `engine/tool/builtin/callworkflow/` directory
2. Create `engine/tool/builtin/callworkflow/types.go`:
   ```go
   type handlerInput struct {
       WorkflowID    string         `json:"workflow_id"     mapstructure:"workflow_id"`
       Input         map[string]any `json:"input"           mapstructure:"input"`
       InitialTaskID string         `json:"initial_task_id" mapstructure:"initial_task_id"`
       TimeoutMs     int            `json:"timeout_ms"      mapstructure:"timeout_ms"`
   }
   ```

### Step 5.2: Schema & Handler
1. Create `engine/tool/builtin/callworkflow/schema.go`
2. Create `engine/tool/builtin/callworkflow/handler.go`:
   - Similar to calltask but for workflows
   - Handle workflow status in response
   - Map workflow states to success/failure

### Step 5.3: Documentation & Testing
1. Create `engine/tool/builtin/callworkflow/README.md`
2. Create `engine/tool/builtin/callworkflow/handler_test.go`

## Phase 6: Native Tool Implementation - cp__call_workflows

### Step 6.1: Package Setup
1. Create `engine/tool/builtin/callworkflows/` directory
2. Create `engine/tool/builtin/callworkflows/types.go`:
   - `WorkflowExecutionRequest`
   - `WorkflowExecutionResult`
   - `workflowPlan`

### Step 6.2: Schema, Handler & Executor
1. Create `engine/tool/builtin/callworkflows/schema.go`
2. Create `engine/tool/builtin/callworkflows/handler.go`
3. Create `engine/tool/builtin/callworkflows/executor.go`:
   - Parallel workflow execution
   - Follow calltasks pattern

### Step 6.3: Documentation & Testing
1. Create `engine/tool/builtin/callworkflows/README.md`
2. Create `engine/tool/builtin/callworkflows/handler_test.go`

## Phase 7: Documentation

### Step 7.1: Tool Documentation
1. Create `docs/content/docs/core/tools/call-task.mdx`:
   - Description and purpose
   - Input fields table
   - Output structure
   - Usage examples
   - Remediation hints
   - Configuration reference
2. Create `docs/content/docs/core/tools/call-tasks.mdx`
3. Create `docs/content/docs/core/tools/call-workflow.mdx`
4. Create `docs/content/docs/core/tools/call-workflows.mdx`

### Step 7.2: System Prompts
1. Update `engine/llm/orchestrator/prompts/templates/system_prompt_with_builtins.tmpl`:
   - Add section for Task Execution tools
   - Add section for Workflow Orchestration tools
   - Include usage patterns and examples

## Phase 8: Integration Testing

### Step 8.1: Integration Tests
1. Create `test/integration/tool/call_task_integration_test.go`:
   - Test end-to-end task execution
   - Test with real task configs
2. Create `test/integration/tool/call_tasks_integration_test.go`:
   - Test parallel execution
   - Test concurrency limits
3. Create `test/integration/tool/call_workflow_integration_test.go`:
   - Test workflow triggering and completion
4. Create `test/integration/tool/call_workflows_integration_test.go`:
   - Test parallel workflow orchestration

### Step 8.2: Example Workflows
1. Create example in `examples/orchestration/`:
   - YAML workflow demonstrating task orchestration with cp__call_task
   - YAML workflow demonstrating workflow-to-workflow with cp__call_workflow
   - README with usage instructions

## Phase 9: Final Validation

### Step 9.1: Code Quality
1. Run `make fmt` to format all code
2. Run `make lint` and fix all issues
3. Run `make test` and ensure all tests pass
4. Review code against Go coding standards

### Step 9.2: Manual Testing
1. Start Compozy server with new tools
2. Test cp__call_task from an agent
3. Test cp__call_tasks with multiple tasks
4. Test cp__call_workflow orchestration
5. Test cp__call_workflows in parallel
6. Verify telemetry in logs/metrics
7. Test error scenarios
8. Test timeout handling
9. Test disabled tool behavior

### Step 9.3: Documentation Review
1. Verify all MDX files render correctly
2. Check system prompts include new tools
3. Ensure README files are complete
4. Validate usage examples

</suggested_steps>

<best_practices>

## Architecture & Design

- **Greenfield Approach**: No backwards compatibility - design for optimal architecture
- **Consistency**: Follow exact patterns from callagent/callagents implementations
- **Separation of Concerns**: Handlers orchestrate, executors execute, schemas validate
- **Interface Segregation**: TaskExecutor and WorkflowExecutor as focused interfaces
- **Dependency Injection**: Environment provides executors, no global singletons
- **Context Propagation**: Always pass context, retrieve logger/config from context
- **Error Handling**: Use builtin error codes and structured metadata

## Concurrency & Performance

- **Go 1.25+ Patterns**: Use `sync.WaitGroup.Go()` for goroutine lifecycle
- **Semaphore Control**: Use `semaphore.Weighted` for max concurrent limits
- **Fail-Safe**: Parallel executions continue despite individual failures
- **Panic Recovery**: Recover panics in goroutines, log and return structured errors
- **Timeout Propagation**: Derive child contexts with timeouts for each execution
- **Ordered Results**: Return results in input order for predictability

## Configuration Management

- **Registry-First**: Define in registry, then build typed config
- **Context-Based**: Retrieve config with `config.FromContext(ctx)`
- **Validation**: Use struct tags for validation, registry for defaults
- **Zero Globals**: Never use global config singletons
- **Sensible Defaults**: Provide safe defaults for all configuration

## Telemetry & Observability

- **Invocation Metrics**: Record at handler level with builtin.RecordInvocation
- **Step Metrics**: Record per-execution steps in parallel tools with builtin.RecordStep
- **Byte Tracking**: Measure response sizes for resource monitoring
- **Duration Tracking**: Capture wall-clock time for performance analysis
- **Error Codes**: Use consistent error codes across all tools

## Testing Strategy

- **Unit Tests**: Test handlers with mocked executors
- **Integration Tests**: Test end-to-end with real executors
- **Error Coverage**: Test all error paths (validation, not found, timeout, disabled)
- **Parallel Edge Cases**: Test concurrency limits, partial failures, panic recovery
- **Timeout Scenarios**: Verify timeout handling and context cancellation

## Code Quality

- **Function Length**: Keep functions under 50 lines (extract helpers if needed)
- **No Line Breaks**: Follow no-linebreaks rule (minimal spacing in functions)
- **Logger from Context**: Always use `logger.FromContext(ctx)`
- **Config from Context**: Always use `config.FromContext(ctx)`
- **Magic Numbers**: Use named constants or config, never hardcode values
- **Error Wrapping**: Provide context when wrapping errors with `fmt.Errorf("%w", err)`

</best_practices>

<should_not>

- ❌ Don't maintain backwards compatibility (greenfield project)
- ❌ Don't use global configuration singletons
- ❌ Don't pass logger or config as function parameters
- ❌ Don't use `context.Background()` in tests (use `t.Context()`)
- ❌ Don't hardcode timeouts, limits, or thresholds (use config)
- ❌ Don't create functions longer than 50 lines
- ❌ Don't add extra line breaks in function bodies
- ❌ Don't skip error handling or validation
- ❌ Don't forget telemetry recording (invocation and steps)
- ❌ Don't mix parallel tool patterns into single execution tools
- ❌ Don't forget to test error scenarios
- ❌ Don't use manual `Add(1)` + `go func()` + `defer Done()` (use `wg.Go()`)
- ❌ Don't allow unbounded concurrency (always use semaphore)
- ❌ Don't forget panic recovery in goroutines
- ❌ Don't return results out of order in parallel tools
- ❌ Don't skip documentation (README, MDX, prompts)
- ❌ Don't forget to update system prompts with new tools
- ❌ Don't create tools without proper JSON schemas
- ❌ Don't copy-paste code without understanding the pattern

</should_not>

<references>

## Existing Implementations (Primary References)

### Agent Call Tools
- `engine/tool/builtin/callagent/handler.go` - Single agent execution pattern
- `engine/tool/builtin/callagent/schema.go` - Schema definitions
- `engine/tool/builtin/callagents/handler.go` - Parallel agent execution
- `engine/tool/builtin/callagents/executor.go` - Concurrency pattern
- `engine/tool/builtin/callagents/README.md` - Implementation notes
- `engine/agent/exec/runner.go` - Agent executor implementation

### Task & Workflow APIs
- `engine/task/router/exec.go` - Task execution HTTP API
- `engine/task/router/register.go` - Task routes
- `engine/workflow/router/execute.go` - Workflow execution HTTP API
- `engine/workflow/router/execute_sync.go` - Workflow sync execution
- `engine/workflow/router/register.go` - Workflow routes
- `engine/worker/mod.go` - Workflow triggering via Worker

### Infrastructure
- `engine/runtime/toolenv/environment.go` - ToolEnv interface
- `engine/runtime/toolenv/agent_executor.go` - Agent executor interface
- `engine/tool/native/catalog.go` - Native tool registration
- `engine/tool/builtin/registry.go` - Builtin tool registration
- `pkg/config/native_tools.go` - Native tools configuration
- `pkg/config/definition/schema.go` - Configuration registry

### Documentation
- `docs/content/docs/core/tools/call-agent.mdx` - Agent tool docs
- `docs/content/docs/core/tools/call-agents.mdx` - Agents tool docs
- `engine/llm/orchestrator/prompts/templates/system_prompt_with_builtins.tmpl` - System prompts

### Project Standards
- `.cursor/rules/go-coding-standards.mdc` - Go coding standards
- `.cursor/rules/architecture.mdc` - Architecture patterns
- `.cursor/rules/test-standards.mdc` - Testing requirements
- `.cursor/rules/backwards-compatibility.mdc` - Greenfield approach
- `.cursor/rules/magic-numbers.mdc` - Constants & magic numbers
- `.cursor/rules/global-config.mdc` - Configuration standards
- `.cursor/rules/logger-config.mdc` - Logger & config context patterns
- `.cursor/rules/no-linebreaks.mdc` - Code formatting rules

</references>

<relevant_files>

**ToolEnv & Interfaces:**
- `engine/runtime/toolenv/environment.go` - Environment interface (to extend)
- `engine/runtime/toolenv/agent_executor.go` - Agent executor interface (reference)
- `engine/runtime/toolenv/env.go` - Environment implementation (to update)
- `engine/runtime/toolenv/task_executor.go` - NEW (to create)
- `engine/runtime/toolenv/workflow_executor.go` - NEW (to create)

**Native Tool Implementations:**
- `engine/tool/builtin/calltask/*` - NEW (to create)
- `engine/tool/builtin/calltasks/*` - NEW (to create)
- `engine/tool/builtin/callworkflow/*` - NEW (to create)
- `engine/tool/builtin/callworkflows/*` - NEW (to create)

**Executor Implementations:**
- `engine/task/exec/runner.go` - NEW (to create)
- `engine/workflow/exec/runner.go` - NEW (to create)
- `engine/workflow/exec/poller.go` - NEW (to create)

**Configuration:**
- `pkg/config/native_tools.go` - Native tools config (to extend)
- `pkg/config/definition/schema.go` - Config registry (to extend)
- `pkg/config/config.go` - Config builders (to extend)

**Registration:**
- `engine/tool/native/catalog.go` - Native tool catalog (auto-registration)
- `engine/tool/builtin/imports/imports.go` - Import side effects (verify imports)

**Documentation:**
- `docs/content/docs/core/tools/call-task.mdx` - NEW (to create)
- `docs/content/docs/core/tools/call-tasks.mdx` - NEW (to create)
- `docs/content/docs/core/tools/call-workflow.mdx` - NEW (to create)
- `docs/content/docs/core/tools/call-workflows.mdx` - NEW (to create)
- `engine/llm/orchestrator/prompts/templates/system_prompt_with_builtins.tmpl` - System prompts (to update)

**Testing:**
- `engine/tool/builtin/calltask/handler_test.go` - NEW (to create)
- `engine/tool/builtin/calltasks/handler_test.go` - NEW (to create)
- `engine/tool/builtin/callworkflow/handler_test.go` - NEW (to create)
- `engine/tool/builtin/callworkflows/handler_test.go` - NEW (to create)
- `test/integration/tool/call_task_integration_test.go` - NEW (to create)
- `test/integration/tool/call_tasks_integration_test.go` - NEW (to create)
- `test/integration/tool/call_workflow_integration_test.go` - NEW (to create)
- `test/integration/tool/call_workflows_integration_test.go` - NEW (to create)

**Reference Files:**
- `engine/tool/builtin/callagent/*` - Reference implementation
- `engine/tool/builtin/callagents/*` - Parallel execution reference
- `engine/agent/exec/runner.go` - Executor pattern reference
- `engine/task/router/exec.go` - Task execution API reference
- `engine/workflow/router/execute.go` - Workflow execution API reference

</relevant_files>

<output>

This implementation enables powerful orchestration patterns:

1. **Workflow-to-Task Orchestration**: Workflows can dynamically call standalone tasks as subroutines
2. **Workflow-to-Workflow Orchestration**: Workflows can trigger other workflows, enabling modular design
3. **Parallel Task Execution**: Execute multiple independent tasks concurrently from a single workflow step
4. **Parallel Workflow Orchestration**: Fan out to multiple workflows for regional processing, multi-tenant operations, etc.

**Key Architectural Benefits:**
- **Modularity**: Tasks and workflows become reusable building blocks
- **Flexibility**: Dynamic composition at runtime vs static YAML definitions
- **Scalability**: Parallel execution with configurable concurrency limits
- **Observability**: Built-in telemetry for all tool invocations
- **Consistency**: Same execution patterns as cp__call_agent/agents

**Use Cases Enabled:**
- Multi-region processing workflows
- Dynamic sub-workflow dispatch based on runtime data
- Task batching and parallel validation
- Workflow-as-function patterns
- Recursive workflow orchestration
- Event-driven workflow chains

**Implementation Priorities:**
1. Follow existing patterns exactly (don't reinvent)
2. Test thoroughly (unit + integration)
3. Document comprehensively (code + user docs)
4. Maintain telemetry consistency
5. Adhere to project standards (lint + format)

</output>

<perplexity>
Use perplexity and context7 to research:
- Go 1.25+ sync.WaitGroup.Go() best practices
- Golang semaphore.Weighted concurrency patterns
- Context timeout propagation in Go
- JSON Schema validation patterns
- Temporal workflow polling strategies (for workflow executor)
</perplexity>

<greenfield>
**YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach. We don't need to care about backwards compatibility since the project is in alpha, and supporting old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility.
</greenfield>

<critical>
**CRITICAL: Read these rule files before starting implementation:**

Before implementing ANY part of this feature, you MUST read and follow these rules:

**Quick Read Command:**
```bash
cat .cursor/rules/{go-coding-standards,architecture,test-standards,backwards-compatibility,magic-numbers,api-standards,global-config,logger-config,no-linebreaks}.mdc
```

**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before implementing to avoid wrong patterns
- **NEVER** use workarounds - implement proper solutions following established patterns
- **NEVER** create functions with more than 50 lines of code
- **MUST** use `logger.FromContext(ctx)` - NEVER pass logger as parameter
- **MUST** use `config.FromContext(ctx)` - NEVER use global config
- **NEVER** use `context.Background()` in tests (use `t.Context()`)
- **MUST** inherit context properly throughout call chains
- **MUST** run `make lint` and `make test` before completing ANY subtask

**For tests and linting:**
- **MUST** run `make lint` and `make test` before finishing
- **YOU CAN ONLY** finish task if both pass
- During development, use scoped commands for faster feedback:
  - Tests: `gotestsum --format pkgname -- -race -parallel=4 <scope>`
  - Linting: `golangci-lint run --fix --allow-parallel-runners <scope>`

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
