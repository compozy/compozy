// Package task provides configuration types and structures for Compozy task orchestration.
//
// Tasks are the **fundamental execution units** in Compozy workflows, representing discrete
// operations that can be composed into sophisticated automation flows. This package defines
// the configuration schemas for all task types, their execution parameters, and orchestration
// patterns that enable powerful workflow orchestration.
//
// ## Task Type Overview
//
// Compozy supports multiple task types, each optimized for specific orchestration patterns:
//
// - **Basic Tasks**: Execute single operations using agents or tools
// - **Router Tasks**: Implement conditional branching based on runtime data
// - **Parallel Tasks**: Run multiple tasks concurrently with various strategies
// - **Collection Tasks**: Iterate over data arrays with batch processing support
// - **Composite Tasks**: Group related tasks into logical units
// - **Signal Tasks**: Enable inter-workflow communication and coordination
// - **Wait Tasks**: Implement delays and synchronization points
// - **Memory Tasks**: Provide persistent state management across executions
// - **Aggregate Tasks**: Combine outputs from multiple task executions
//
// ## Configuration Philosophy
//
// Task configurations follow Compozy's declarative YAML approach, emphasizing:
//
// - **Composability**: Tasks can be nested and combined arbitrarily
// - **Reusability**: Agent and tool references enable configuration reuse
// - **Expressiveness**: Template expressions provide dynamic runtime behavior
// - **Validation**: JSON Schema integration ensures configuration correctness
// - **Observability**: Built-in logging, metrics, and error handling
//
// ## Example Usage
//
//	# Multi-stage data processing workflow
//	tasks:
//	  - id: validate-input
//	    type: basic
//	    agent: { id: data-validator }
//	    with:
//	      data: "{{ .workflow.input.raw_data }}"
//
//	  - id: process-parallel
//	    type: parallel
//	    strategy: wait_all
//	    tasks:
//	      - id: extract-entities
//	        type: basic
//	        agent: { id: entity-extractor }
//	      - id: analyze-sentiment
//	        type: basic
//	        agent: { id: sentiment-analyzer }
//
//	  - id: route-results
//	    type: router
//	    condition: "tasks.validate_input.output.confidence > 0.8"
//	    routes:
//	      true: high-confidence-processor
//	      false: manual-review-queue
//
// ## Integration Points
//
// Task configurations integrate with other Compozy components:
//
// - **Agents**: AI-powered processing units defined in engine/agent
// - **Tools**: External command execution defined in engine/tool
// - **Schemas**: Input/output validation defined in engine/schema
// - **Memory**: Persistent state management via engine/memory
// - **MCP**: External tool servers via Model Context Protocol
package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/mitchellh/mapstructure"
)

// -----------------------------------------------------------------------------
// BaseConfig - Common fields shared between Config and ParallelTaskItem
// -----------------------------------------------------------------------------

// BaseConfig contains the common configuration fields used by all task types.
//
// Every task in Compozy shares these foundational fields that control execution behavior,
// resource management, and workflow orchestration. These fields provide the foundation
// for all task types and enable sophisticated workflow orchestration patterns.
//
// ## Core Capabilities
//
// BaseConfig provides tasks with essential capabilities:
//
// - **🏷️ Identification**: Unique task IDs and resource references
// - **⚙️ Execution**: Agent or tool configurations for processing
// - **✅ Validation**: JSON Schema validation for inputs and outputs
// - **🔀 Flow Control**: Conditional execution and workflow transitions
// - **🔧 Error Handling**: Retry logic and fallback strategies
// - **🌍 Environment**: Variable management and working directories
//
// ## Example: Complete Task Configuration
//
// ```yaml
// # Task identification and type
// id: process-customer-data
// type: basic
// resource: "compozy:task:customer-processor"
//
// # Execution configuration
// agent:
//
//	id: data-processor
//	model: claude-3-5-haiku-latest
//	instructions: "Extract key information from customer data"
//
// # Input validation schema
// input:
//
//	type: object
//	properties:
//	  customer_data: { type: string, minLength: 10 }
//	  priority: { type: string, enum: ["low", "medium", "high"] }
//	required: [customer_data]
//
// # Task parameters with template expressions
// with:
//
//	customer_data: "{{ .workflow.input.raw_data }}"
//	priority: "{{ .workflow.input.priority | default('medium') }}"
//	context:
//	  user_id: "{{ .workflow.input.user_id }}"
//	  timestamp: "{{ now }}"
//
// # Output mappings for subsequent tasks
// outputs:
//
//	processed_data: "{{ .task.output.result }}"
//	confidence_score: "{{ .task.output.confidence }}"
//	requires_review: "{{ .task.output.confidence < 0.8 }}"
//
// # Environment variables
// env:
//
//	LOG_LEVEL: debug
//	API_KEY: "{{ .env.SECRET_API_KEY }}"
//
// # Success flow control
// on_success:
//
//	next: save-results
//	with:
//	  data: "{{ .task.outputs.processed_data }}"
//
// # Error handling
// on_error:
//
//	next: error-handler
//	retry: 3
//	backoff: exponential
//
// # Additional controls
// timeout: 30s
// condition: "input.priority != 'low'"
// sleep: 1s
// ```
type BaseConfig struct {
	// Resource reference for the task
	// Format: "compozy:task:<name>" (e.g., "compozy:task:process-data")
	Resource string `json:"resource,omitempty"   yaml:"resource,omitempty"   mapstructure:"resource,omitempty"`
	// Unique identifier for the task instance within a workflow
	// Must be unique within the workflow scope
	ID string `json:"id,omitempty"         yaml:"id,omitempty"         mapstructure:"id,omitempty"`
	// Type of task that determines execution behavior
	// If not specified, defaults to "basic"
	Type Type `json:"type,omitempty"       yaml:"type,omitempty"       mapstructure:"type,omitempty"`
	// Global configuration options inherited from parent contexts
	// Includes provider settings, API keys, and other global parameters
	Config core.GlobalOpts `json:"config"               yaml:"config"               mapstructure:"config"`
	// Agent configuration for AI-powered task execution
	// Only used when the task needs to interact with an LLM agent
	// Mutually exclusive with Tool field
	// $ref: schema://agents
	Agent *agent.Config `json:"agent,omitempty"      yaml:"agent,omitempty"      mapstructure:"agent,omitempty"`
	// Tool configuration for executing specific tool operations
	// Used when the task needs to execute a predefined tool
	// Mutually exclusive with Agent field
	// $ref: schema://tools
	Tool *tool.Config `json:"tool,omitempty"       yaml:"tool,omitempty"       mapstructure:"tool,omitempty"`
	// Schema definition for validating task input parameters
	// Follows JSON Schema specification for type validation
	// Format:
	//   type: object
	//   properties:
	//     user_id: { type: string, description: "User identifier" }
	//   required: ["user_id"]
	InputSchema *schema.Schema `json:"input,omitempty"      yaml:"input,omitempty"      mapstructure:"input,omitempty"`
	// Schema definition for validating task output data
	// Ensures task results conform to expected structure
	// Uses same format as InputSchema
	OutputSchema *schema.Schema `json:"output,omitempty"     yaml:"output,omitempty"     mapstructure:"output,omitempty"`
	// Input parameters passed to the task at execution time
	// Can include references to workflow inputs, previous task outputs, etc.
	// - **Example**: { "user_id": "{{ .workflow.input.user_id }}" }
	With *core.Input `json:"with,omitempty"       yaml:"with,omitempty"       mapstructure:"with,omitempty"`
	// Output mappings that define what data this task exposes to subsequent tasks
	// Uses template expressions to transform task results
	// - **Example**: { "processed_data": "{{ .task.output.result }}" }
	Outputs *core.Input `json:"outputs,omitempty"    yaml:"outputs,omitempty"    mapstructure:"outputs,omitempty"`
	// Environment variables available during task execution
	// Can override or extend workflow-level environment variables
	// - **Example**: { "API_KEY": "{{ .env.SECRET_KEY }}" }
	Env *core.EnvMap `json:"env,omitempty"        yaml:"env,omitempty"        mapstructure:"env,omitempty"`
	// Knowledge declares task-scoped knowledge bindings (MVP single binding).
	Knowledge []core.KnowledgeBinding `json:"knowledge,omitempty"  yaml:"knowledge,omitempty"  mapstructure:"knowledge,omitempty"`
	// Task execution control
	// Defines what happens after successful task completion
	// Can specify next task ID or conditional routing
	OnSuccess *core.SuccessTransition `json:"on_success,omitempty" yaml:"on_success,omitempty" mapstructure:"on_success,omitempty"`
	// Error handling configuration
	// Defines fallback behavior when task execution fails
	// Can specify error task ID or retry configuration
	OnError *core.ErrorTransition `json:"on_error,omitempty"   yaml:"on_error,omitempty"   mapstructure:"on_error,omitempty"`
	// Sleep duration after task completion
	// Format: "5s", "1m", "500ms", "1h30m"
	// Useful for rate limiting or giving external systems time to process
	Sleep string `json:"sleep"                yaml:"sleep"                mapstructure:"sleep"`
	// Marks this task as a terminal node in the workflow
	// No subsequent tasks will execute after a final task
	Final bool `json:"final"                yaml:"final"                mapstructure:"final"`
	// Absolute file path where this task configuration was loaded from
	// Set automatically during configuration loading
	FilePath string `json:"file_path,omitempty"  yaml:"file_path,omitempty"  mapstructure:"file_path,omitempty"`
	// Current working directory for file operations within the task
	// Inherited from parent context if not explicitly set
	CWD *core.PathCWD `json:"CWD,omitempty"        yaml:"CWD,omitempty"        mapstructure:"CWD,omitempty"`
	// Maximum execution time for parallel or composite tasks
	// Format: "30s", "5m", "1h"
	// Task will be canceled if it exceeds this duration
	Timeout string `json:"timeout,omitempty"    yaml:"timeout,omitempty"    mapstructure:"timeout,omitempty"`
	// Number of retry attempts for failed task executions
	// Default: 0 (no retries)
	Retries int `json:"retries,omitempty"    yaml:"retries,omitempty"    mapstructure:"retries,omitempty"`
	// CEL expression for conditional task execution or routing decisions
	// Task only executes if condition evaluates to true
	// - **Example**: "input.status == 'approved' && input.amount > 1000"
	Condition string `json:"condition,omitempty"  yaml:"condition,omitempty"  mapstructure:"condition,omitempty"`

	// Attachments declared at the task scope are available to all nested agents/actions.
	Attachments attachment.Attachments `json:"attachments,omitempty" yaml:"attachments,omitempty" mapstructure:"attachments,omitempty"`
}

// -----------------------------------------------------------------------------
// Task Types
// -----------------------------------------------------------------------------

// Type represents the different execution patterns available in Compozy.
//
// Task types are the foundation of Compozy's workflow orchestration, each providing
// specialized execution patterns for different automation needs. Choosing the right
// task type enables efficient, maintainable, and scalable workflow designs.
//
// ## 📋 Complete Task Types Reference
//
// | Type | Pattern | Concurrency | Use Cases | Example Scenarios |
// |------|---------|-------------|-----------|-------------------|
// | **`basic`** | Single execution | None | Individual operations | API calls, data processing, AI analysis |
// | **`router`** | Conditional branching | None | Decision logic | Approval routing, content classification |
// | **`parallel`** | Concurrent execution | Multi-task | Independent operations | Data enrichment, batch validation |
// | **`collection`** | Iteration | Configurable | Array processing | User processing, file transformation |
// | **`composite`** | Sequential grouping | None | Related tasks | Multi-step processes, reusable workflows |
// | **`aggregate`** | Result combining | None | Data consolidation | Report generation, data merging |
// | **`signal`** | Event emission | None | Coordination | Notifications, workflow triggers |
// | **`wait`** | Event listening | None | Synchronization | Approvals, external events |
// | **`memory`** | State management | None | Persistent data | Caching, session management, counters |
//
// ## 🎯 Decision Matrix: Choosing the Right Task Type
//
// ### **Processing Requirements**
// - **Single item/operation** → `basic`
// - **Multiple items in sequence** → `composite`
// - **Multiple items concurrently** → `parallel`
// - **Array of similar items** → `collection`
// - **Combine multiple results** → `aggregate`
//
// ### **Flow Control Requirements**
// - **Conditional routing** → `router`
// - **Event-driven coordination** → `signal` + `wait`
// - **State persistence** → `memory`
//
// ### **Performance Requirements**
// - **Speed (parallel processing)** → `parallel` or `collection` (parallel mode)
// - **Resource efficiency** → `collection` (sequential mode) or `composite`
// - **Event-driven** → `signal`/`wait` patterns
//
// ## 🏗️ Architecture Patterns
//
// ### **Fan-Out/Fan-In Pattern**
// ```
// Input → Parallel Tasks → Aggregate → Output
// ```
// Use `parallel` followed by `aggregate` for distributed processing.
//
// ### **Pipeline Pattern**
// ```
// Task A → Task B → Task C → Output
// ```
// Use `composite` for sequential processing stages.
//
// ### **Conditional Pipeline**
// ```
// Input → Router → Branch A or Branch B → Output
// ```
// Use `router` for dynamic workflow paths.
//
// ### **Event-Driven Pattern**
// ```
// Process → Signal → Wait → Continue
// ```
// Use `signal`/`wait` for cross-workflow coordination.
type Type string

const (
	// TaskTypeBasic executes a single action using an agent or tool
	// This is the most common task type for individual operations
	// - **Example**:
	//   type: basic
	//   agent:
	//     id: data-processor
	//     model: claude-3-5-haiku-latest
	TaskTypeBasic Type = "basic"
	// TaskTypeRouter conditionally routes to different tasks based on conditions
	// Uses CEL expressions to evaluate routing logic
	// - **Example**:
	//   type: router
	//   condition: "input.status"
	//   routes:
	//     approved: approve-task
	//     rejected: reject-task
	TaskTypeRouter Type = "router"
	// TaskTypeParallel executes multiple tasks concurrently
	// Supports different strategies: wait_all, fail_fast, best_effort, race
	// - **Example**:
	//   type: parallel
	//   strategy: wait_all
	//   max_workers: 5
	//   tasks:
	//     - id: task1
	//     - id: task2
	TaskTypeParallel Type = "parallel"
	// TaskTypeCollection iterates over a list of items, executing tasks for each
	// Can run in parallel or sequential mode with batching support
	// - **Example**:
	//   type: collection
	//   items: "{{ .workflow.input.users }}"
	//   mode: parallel
	//   batch: 10
	//   task:
	//     type: basic
	//     agent: { id: process-user }
	TaskTypeCollection Type = "collection"
	// TaskTypeAggregate combines outputs from multiple previous tasks
	// Useful for consolidating results from parallel executions
	// Currently implemented as a basic task with special handling
	TaskTypeAggregate Type = "aggregate"
	// TaskTypeComposite groups related tasks into a reusable unit
	// Acts as a sub-workflow that can be referenced from other workflows
	// Tasks within composite execute sequentially (always wait_all strategy)
	// - **Example**:
	//   type: composite
	//   tasks:
	//     - id: step1
	//     - id: step2
	TaskTypeComposite Type = "composite"
	// TaskTypeSignal sends signals to other waiting tasks or workflows
	// Enables event-driven coordination between workflow components
	// - **Example**:
	//   type: signal
	//   signal:
	//     id: user-approved
	//     payload: { user_id: "{{ .input.user_id }}" }
	TaskTypeSignal Type = "signal"
	// TaskTypeWait pauses execution until a condition is met or signal received
	// Supports timeout and custom processing of received signals
	// - **Example**:
	//   type: wait
	//   wait_for: user-approved
	//   condition: "signal.payload.user_id == input.user_id"
	//   timeout: 5m
	//   on_timeout: timeout-handler
	TaskTypeWait Type = "wait"
	// TaskTypeMemory performs operations on shared memory stores
	// Supports read, write, append, delete, flush, health check, and stats operations
	// - **Example**:
	//   type: memory
	//   operation: write
	//   memory_ref: user-session
	//   key_template: "user:{{ .input.user_id }}"
	//   payload: { last_seen: "{{ .now }}" }
	TaskTypeMemory Type = "memory"
)

// -----------------------------------------------------------------------------
// Basic Task
// -----------------------------------------------------------------------------

// BasicTask represents a simple task that executes a single action.
//
// The basic task is the **fundamental building block** of Compozy workflows, representing
// atomic operations that cannot be broken down further. It executes exactly one operation
// using either an AI agent or a deterministic tool, making it the most commonly used
// task type in workflow automation.
//
// ## 🎯 When to Use Basic Tasks
//
// Basic tasks are ideal for:
//
// | Use Case | Example | Why Basic? |
// |----------|---------|------------|
// | **AI Processing** | Content analysis, generation, classification | Single LLM operation |
// | **API Calls** | REST requests, webhooks, external services | Single HTTP operation |
// | **Data Transformation** | Format conversion, filtering, mapping | Single processing step |
// | **File Operations** | Read, write, upload, download | Single file action |
// | **Validation** | Schema checking, business rules | Single validation step |
// | **Calculations** | Math operations, scoring, metrics | Single computation |
//
// ## ⚡ Execution Modes: Agent, Tool, Direct LLM
//
// Basic tasks support three execution modes. Use exactly one per task:
// - Agent: AI-powered execution using an agent definition
// - Tool: Deterministic execution using a tool configuration (no action or prompt)
// - Direct LLM: Call the LLM directly using `model_config` + `prompt`
//
// ### 🤖 Agent Execution (AI-Powered)
//
// **Best for:** Dynamic processing, content analysis, decision-making
//
// ```yaml
// type: basic
// agent:
//
//	id: sentiment-analyzer
//	model: claude-3-5-haiku-latest
//	instructions: |
//	  Analyze customer feedback sentiment and extract key insights:
//	  1. Overall sentiment (positive/negative/neutral)
//	  2. Confidence score (0-1)
//	  3. Key themes mentioned
//	  4. Recommended actions
//
// action: analyze_customer_feedback
// with:
//
//	feedback_text: "{{ .workflow.input.customer_message }}"
//	customer_id: "{{ .workflow.input.customer.id }}"
//	metadata:
//	  source: "{{ .workflow.input.source }}"
//	  timestamp: "{{ now }}"
//
// outputs:
//
//	sentiment: "{{ .task.output.sentiment }}"
//	confidence: "{{ .task.output.confidence }}"
//	themes: "{{ .task.output.key_themes }}"
//	next_action: "{{ .task.output.recommended_action }}"
//
// ```
//
// ### 🔧 Tool Execution (Deterministic)
//
// **Best for:** API calls, file operations, data processing, integrations
//
// Note: When using tools, `action` and `prompt` are not allowed.
//
// ```yaml
// type: basic
// tool:
//
//	id: http-client
//	config:
//	  method: POST
//	  url: "https://api.crm.example.com/customers"
//	  headers:
//	    Authorization: "Bearer {{ .env.CRM_TOKEN }}"
//	    Content-Type: "application/json"
//
// with:
//
//	customer_data:
//	  name: "{{ .workflow.input.customer.name }}"
//	  email: "{{ .workflow.input.customer.email }}"
//	  sentiment_analysis: "{{ .tasks.analyze_feedback.outputs.sentiment }}"
//	  risk_score: "{{ .tasks.analyze_feedback.outputs.confidence }}"
//
// outputs:
//
//	customer_id: "{{ .task.output.id }}"
//	created_at: "{{ .task.output.created_at }}"
//	profile_url: "{{ .task.output.profile_url }}"
//
// ```
//
// ### 🧠 Direct LLM (model_config + prompt)
//
// **Best for:** Simple, ad-hoc LLM calls without defining an agent
//
// ```yaml
// type: basic
// model_config:
//
//	provider: anthropic
//	model: claude-3-5-haiku-latest
//
// prompt: |
//
//	Analyze this customer message and return JSON with:
//	- sentiment: positive|neutral|negative
//	- confidence: 0..1
//	- themes: string[]
//
// with:
//
//	message: "{{ .workflow.input.customer_message }}"
//
// outputs:
//
//	sentiment: "{{ .task.output.sentiment }}"
//	confidence: "{{ .task.output.confidence }}"
//	themes: "{{ .task.output.themes }}"
//
// ```
//
// ## 🏷️ Action Field Benefits
//
// The `action` field (for agents and direct LLM tasks) provides multiple advantages. It is not used with tools:
//
// | Benefit | Description | Example |
// |---------|-------------|---------|
// | **Identification** | Unique operation identifier | `analyze_sentiment`, `send_email` |
// | **Logging** | Structured log entries | `[analyze_sentiment] Processing customer feedback` |
// | **Metrics** | Performance tracking by action | Action execution times, success rates |
// | **Templates** | Action-specific prompts | Different prompts for different analysis types |
// | **Debugging** | Clear error context | `[create_customer] API call failed: 401 Unauthorized` |
//
// ## 💡 Best Practices
//
// ### **Action Naming**
// - Use descriptive verbs: `analyze_`, `create_`, `validate_`, `send_`
// - Include domain context: `process_payment`, `analyze_sentiment`
// - Maintain consistency: `create_user`, `update_user`, `delete_user`
//
// ### **Input/Output Design**
// - Define clear schemas for validation
// - Use meaningful output keys for chaining
// - Include metadata for debugging
//
// ### **Error Handling**
// - Set appropriate retry counts
// - Define fallback tasks for critical operations
// - Use exponential backoff for external APIs
type BasicTask struct {
	// Embed LLMProperties with inline tags for backward compatibility
	// This allows fields to be accessed directly on Config in YAML/JSON
	agent.LLMProperties `json:",inline" yaml:",inline" mapstructure:",squash"`

	// LLM provider configuration defining which AI model to use and its parameters.
	// Supports multiple providers including OpenAI, Anthropic, Google, Groq, and local models.
	//
	// **Required fields:** provider, model
	// **Optional fields:** api_key, api_url, params (temperature, max_tokens, etc.)
	ModelConfig core.ProviderConfig `json:"model_config" yaml:"model_config,omitempty" mapstructure:"model_config,omitempty"`

	// Action identifier that describes what this task does
	// Used for logging and debugging purposes
	// - **Example**: "process-user-data", "send-notification"
	Action string `json:"action,omitempty" yaml:"action,omitempty" mapstructure:"action,omitempty"`

	// Prompt provides direct instruction to agents when no specific action is needed
	// Used for ad-hoc agent interactions without predefined action definitions
	// - **Example**: "Analyze this code for security issues", "Summarize the following text"
	Prompt string `json:"prompt,omitempty" yaml:"prompt,omitempty" mapstructure:"prompt,omitempty"`
}

// -----------------------------------------------------------------------------
// Router Task
// -----------------------------------------------------------------------------

// RouterTask implements conditional branching in workflows.
//
// Router tasks are the **decision engine** of Compozy workflows, enabling dynamic
// routing based on runtime conditions. They evaluate CEL (Common Expression Language)
// expressions and direct workflow execution down different paths, making workflows
// intelligent and adaptive to varying data conditions.
//
// ## 🎯 When to Use Router Tasks
//
// Router tasks excel in scenarios requiring conditional logic:
//
// | Scenario | Use Case | Example |
// |----------|----------|---------|
// | **Content Classification** | Route by document type | Invoice → OCR, Image → Vision API |
// | **Approval Workflows** | Route by amount/priority | High value → Manager, Low → Auto-approve |
// | **Error Handling** | Route by status/error type | Success → Continue, Error → Retry logic |
// | **Feature Flags** | Route by configuration | Beta users → New flow, Others → Legacy |
// | **Business Rules** | Route by complex conditions | Premium → Fast lane, Standard → Queue |
//
// ## ⚙️ How Router Tasks Work
//
// Router tasks follow a simple but powerful pattern:
//
// ```
// Input Data → CEL Condition → Route Key → Target Task
// ```
//
// 1. **📊 Condition Evaluation**: CEL expression processes input data
// 2. **🔑 Route Resolution**: Result maps to a route key
// 3. **➡️ Task Execution**: Matched route determines next task
// 4. **🔄 Flow Continuation**: Workflow continues with selected path
//
// ## 📝 Practical Examples
//
// ### **Document Processing Router**
//
// ```yaml
// id: document-router
// type: router
// condition: "input.document.type"
// routes:
//
//	# Different processors for different document types
//	invoice:
//	  type: basic
//	  agent:
//	    id: invoice-processor
//	    model: claude-4-opus
//	    instructions: "Extract invoice data: vendor, amount, date, line items"
//	  with:
//	    document_content: "{{ .workflow.input.document.content }}"
//
//	receipt:
//	  type: basic
//	  tool:
//	    id: ocr-tool
//	    config:
//	      engine: tesseract
//	      language: en
//	  with:
//	    image_data: "{{ .workflow.input.document.image }}"
//
//	contract:
//	  type: composite
//	  tasks:
//	    - id: extract-terms
//	      type: basic
//	      agent: { id: contract-analyzer }
//	    - id: legal-review
//	      type: basic
//	      agent: { id: legal-reviewer }
//
//	# Fallback for unknown types
//	default: manual-review-task
//
// ```
//
// ### **Approval Workflow Router**
//
// ```yaml
// id: approval-router
// type: router
// condition: |
//
//	input.request.amount > 10000 ? 'executive' :
//	input.request.amount > 1000 ? 'manager' :
//	input.request.category == 'restricted' ? 'compliance' :
//	'auto_approve'
//
// routes:
//
//	executive:
//	  type: signal
//	  signal:
//	    id: executive-approval-required
//	    payload:
//	      request_id: "{{ .workflow.input.request.id }}"
//	      amount: "{{ .workflow.input.request.amount }}"
//	      requester: "{{ .workflow.input.request.user }}"
//
//	manager: manager-approval-task
//	compliance: compliance-review-task
//	auto_approve: auto-approval-task
//
// ```
//
// ### **Error Handling Router**
//
// ```yaml
// id: error-handler
// type: router
// condition: |
//
//	input.error.type == 'timeout' ? 'retry' :
//	input.error.type == 'auth' ? 'reauth' :
//	input.error.severity == 'critical' ? 'escalate' :
//	'log_and_continue'
//
// routes:
//
//	retry:
//	  type: basic
//	  tool: { id: retry-handler }
//	  with:
//	    original_task: "{{ .workflow.input.failed_task }}"
//	    attempt: "{{ .workflow.input.retry_count + 1 }}"
//
//	reauth: authentication-flow
//	escalate: alert-on-call-team
//	log_and_continue: error-logger
//
// ```
//
// ## 🔀 Route Configuration Options
//
// Routes support two powerful configuration patterns:
//
// ### **1. Task ID References** (Simple & Clean)
// ```yaml
// routes:
//
//	approved: "send-confirmation"
//	pending: "wait-for-approval"
//	rejected: "send-rejection-notice"
//
// ```
//
// ### **2. Inline Task Definitions** (Powerful & Flexible)
// ```yaml
// routes:
//
//	high_priority:
//	  type: parallel
//	  strategy: fail_fast
//	  tasks:
//	    - id: immediate-notification
//	      type: basic
//	      tool: { id: sms-sender }
//	    - id: priority-processing
//	      type: basic
//	      agent: { id: priority-processor }
//
//	standard:
//	  type: basic
//	  agent: { id: standard-processor }
//
// ```
//
// ## 🎨 Advanced Routing Patterns
//
// ### **Multi-Condition Routing**
// ```yaml
// condition: |
//
//	input.user.tier == 'premium' && input.region == 'us' ? 'premium_us' :
//	input.user.tier == 'premium' ? 'premium_intl' :
//	input.urgent ? 'priority' : 'standard'
//
// ```
//
// ### **Data-Driven Routing**
// ```yaml
// condition: "input.config.routing_strategy"  # Dynamic routing from data
// routes:
//
//	fast_path: quick-processor
//	thorough_path: detailed-processor
//	custom_path: custom-handler
//
// ```
//
// ### **Feature Flag Routing**
// ```yaml
// condition: |
//
//	env.FEATURE_NEW_PROCESSOR == 'true' ? 'new_processor' : 'legacy_processor'
//
// routes:
//
//	new_processor: ai-enhanced-processor
//	legacy_processor: traditional-processor
//
// ```
//
// ## 💡 Best Practices
//
// ### **Condition Design**
// - **Always include a default**: Handle unexpected values gracefully
// - **Keep conditions readable**: Break complex logic into helper variables
// - **Test edge cases**: Ensure all possible inputs have routes
//
// ### **Route Organization**
// - **Use descriptive keys**: `high_value` vs `route_a`
// - **Group related routes**: Keep similar logic together
// - **Document business rules**: Comment complex routing logic
type RouterTask struct {
	// Routes maps condition values to task IDs or inline task configurations
	// The condition field in BaseConfig is evaluated, and its result is used
	// as the key to select the appropriate route
	// Values can be:
	//   - Task ID (string): References an existing task
	//   - Inline task config (object): Defines task configuration directly
	// - **Example**:
	//   routes:
	//     approved: "process-payment"  # Task ID reference
	//     rejected:                    # Inline task config
	//       type: basic
	//       agent: { id: rejection-handler }
	//     pending: "wait-for-approval"
	Routes map[string]any `json:"routes,omitempty" yaml:"routes,omitempty" mapstructure:"routes,omitempty"`
}

// -----------------------------------------------------------------------------
// Parallel Task
// -----------------------------------------------------------------------------

// ParallelStrategy defines how parallel tasks handle completion and failures.
//
// Parallel execution strategies are **critical decision points** that determine workflow
// behavior under concurrent execution. Each strategy optimizes for different reliability,
// performance, and error handling requirements, making them essential for robust
// workflow design.
//
// ## 📊 Comprehensive Strategy Comparison
//
// | Strategy | Waits For | Success Criteria | Failure Behavior | Best For |
// |----------|-----------|------------------|-------------------|----------|
// | **`wait_all`** | All tasks finish | All succeed | Fails if any fails (after all complete) | **Default**: Balanced reliability |
// | **`fail_fast`** | First failure/all success | All succeed | **Immediately** cancels on first failure | **Critical**: Must-succeed operations |
// | **`best_effort`** | All tasks finish | Succeeds with partial results | **Never fails**, logs errors | **Batch**: Non-critical operations |
// | **`race`** | First success | Any one succeeds | Fails only if **all** fail | **Speed**: Redundant/fallback systems |
//
// ## 🎯 Strategy Selection Guide
//
// ### **`wait_all` - The Reliable Default**
// **✅ Use when:** You need all results but can tolerate some delay
// ```yaml
// # Data enrichment - need all sources
// strategy: wait_all
// tasks:
//   - fetch-user-profile      # Must have
//   - fetch-purchase-history  # Must have
//   - fetch-preferences       # Must have
//
// ```
//
// ### **`fail_fast` - The Critical Path**
// **✅ Use when:** Any failure makes continuation pointless
// ```yaml
// # Payment processing - all validations must pass
// strategy: fail_fast
// tasks:
//   - validate-payment-method
//   - check-fraud-score
//   - verify-inventory
//   - confirm-shipping-address
//
// ```
//
// ### **`best_effort` - The Resilient Collector**
// **✅ Use when:** Partial success is valuable
// ```yaml
// # Multi-source data collection - get what you can
// strategy: best_effort
// tasks:
//   - fetch-social-media-data    # Nice to have
//   - fetch-external-reviews     # Nice to have
//   - fetch-competitor-data      # Nice to have
//
// ```
//
// ### **`race` - The Speed Champion**
// **✅ Use when:** First result wins, others are redundant
// ```yaml
// # Redundant data sources - fastest wins
// strategy: race
// tasks:
//   - fetch-from-primary-api
//   - fetch-from-backup-api
//   - fetch-from-cache
//
// ```
type ParallelStrategy string

const (
	// StrategyWaitAll waits for all tasks to complete before proceeding
	// Default strategy for parallel execution
	// All tasks must finish (success or failure) before the parallel task completes
	StrategyWaitAll ParallelStrategy = "wait_all"
	// StrategyFailFast stops execution immediately when any task fails
	// Useful when all tasks must succeed for the workflow to continue
	// Running tasks are canceled when one fails
	StrategyFailFast ParallelStrategy = "fail_fast"
	// StrategyBestEffort continues execution even if some tasks fail
	// Failed tasks are recorded but don't stop other tasks
	// Completes when all tasks have finished regardless of success/failure
	StrategyBestEffort ParallelStrategy = "best_effort"
	// StrategyRace returns as soon as the first task completes successfully
	// Other tasks are canceled once one succeeds
	// Fails only if all tasks fail
	StrategyRace ParallelStrategy = "race"
)

// ValidateStrategy checks if the given string is a valid ParallelStrategy.
// Used for configuration validation to ensure only valid strategies are used.
func ValidateStrategy(strategy string) bool {
	switch ParallelStrategy(strategy) {
	case StrategyWaitAll, StrategyFailFast, StrategyBestEffort, StrategyRace:
		return true
	default:
		return false
	}
}

// ParallelTask executes multiple tasks concurrently with configurable behavior.
//
// Parallel tasks are **performance multipliers** in Compozy workflows, enabling
// concurrent execution of independent operations. They transform sequential bottlenecks
// into high-throughput parallel processing, dramatically improving workflow performance
// while providing sophisticated control over execution behavior and resource usage.
//
// ## 🚀 Performance Impact
//
// | Sequential vs Parallel | Execution Time | Throughput | Resource Usage |
// |------------------------|----------------|------------|----------------|
// | **3 Sequential Tasks** | 30s (10s each) | 1 task/10s | Low CPU usage |
// | **3 Parallel Tasks** | 10s (max of all) | 3 tasks/10s | High CPU usage |
// | **10 Parallel Tasks** | 10s (if independent) | 10 tasks/10s | Resource-dependent |
//
// ## 🎯 When to Use Parallel Tasks
//
// Parallel tasks excel in these scenarios:
//
// | Use Case | Performance Gain | Example |
// |----------|------------------|---------|
// | **Independent Data Sources** | 3-10x faster | API calls, database queries |
// | **Validation Checks** | 2-5x faster | Schema, business rules, permissions |
// | **File Processing** | 5-20x faster | Image processing, document parsing |
// | **Redundant Operations** | Latency reduction | Multiple API endpoints |
// | **Batch Operations** | Linear scaling | Email sending, notification dispatch |
//
// ## 📋 Comprehensive Examples
//
// ### **Data Enrichment Pipeline**
//
// ```yaml
// id: enrich-customer-profile
// type: parallel
// strategy: wait_all  # Need all data sources
// max_workers: 5      # Limit concurrent API calls
// timeout: 30s        # Prevent hung requests
//
// tasks:
//
//   - id: fetch-basic-profile
//     type: basic
//     tool:
//     id: api-client
//     config:
//     method: GET
//     url: "https://api.users.com/profile/{{ .workflow.input.user_id }}"
//     headers:
//     Authorization: "Bearer {{ .env.USER_API_TOKEN }}"
//     outputs:
//     profile: "{{ .task.output }}"
//
//   - id: fetch-purchase-history
//     type: basic
//     tool:
//     id: api-client
//     config:
//     method: GET
//     url: "https://api.orders.com/history/{{ .workflow.input.user_id }}"
//     outputs:
//     orders: "{{ .task.output.orders }}"
//     total_spent: "{{ .task.output.total_amount }}"
//
//   - id: fetch-preferences
//     type: basic
//     tool:
//     id: database-client
//     config:
//     query: "SELECT * FROM user_preferences WHERE user_id = ?"
//     params: ["{{ .workflow.input.user_id }}"]
//     outputs:
//     preferences: "{{ .task.output }}"
//
//   - id: fetch-social-data
//     type: basic
//     agent:
//     id: social-analyzer
//     model: claude-3-5-haiku-latest
//     instructions: "Extract insights from social media data"
//     with:
//     user_handle: "{{ .workflow.input.social_handle }}"
//     outputs:
//     social_insights: "{{ .task.output.insights }}"
//
// # Combine all results
// outputs:
//
//	enriched_profile:
//	  basic_info: "{{ .tasks.fetch-basic-profile.outputs.profile }}"
//	  purchase_history: "{{ .tasks.fetch-purchase-history.outputs.orders }}"
//	  total_spent: "{{ .tasks.fetch-purchase-history.outputs.total_spent }}"
//	  preferences: "{{ .tasks.fetch-preferences.outputs.preferences }}"
//	  social_insights: "{{ .tasks.fetch-social-data.outputs.social_insights }}"
//
// ```
//
// ### **Critical Validation Chain**
//
// ```yaml
// id: payment-validation
// type: parallel
// strategy: fail_fast  # Any failure should stop processing
// max_workers: 0       # No limit - all validations must run
//
// tasks:
//
//   - id: validate-payment-method
//     type: basic
//     tool:
//     id: payment-validator
//     config:
//     provider: stripe
//     with:
//     payment_method_id: "{{ .workflow.input.payment_method }}"
//     outputs:
//     is_valid: "{{ .task.output.valid }}"
//     expires_at: "{{ .task.output.expires_at }}"
//
//   - id: check-fraud-score
//     type: basic
//     agent:
//     id: fraud-detector
//     model: claude-4-opus
//     instructions: |
//     Analyze transaction for fraud indicators:
//
//   - Unusual spending patterns
//
//   - Geographic anomalies
//
//   - Device fingerprinting
//     with:
//     transaction: "{{ .workflow.input.transaction }}"
//     user_history: "{{ .workflow.input.user_history }}"
//     outputs:
//     fraud_score: "{{ .task.output.fraud_probability }}"
//     risk_factors: "{{ .task.output.risk_factors }}"
//
//   - id: verify-inventory
//     type: basic
//     tool:
//     id: inventory-checker
//     with:
//     items: "{{ .workflow.input.cart_items }}"
//     outputs:
//     available: "{{ .task.output.all_available }}"
//     out_of_stock: "{{ .task.output.unavailable_items }}"
//
//   - id: check-shipping-zones
//     type: basic
//     tool:
//     id: shipping-validator
//     with:
//     destination: "{{ .workflow.input.shipping_address }}"
//     items: "{{ .workflow.input.cart_items }}"
//     outputs:
//     can_ship: "{{ .task.output.deliverable }}"
//     shipping_options: "{{ .task.output.available_methods }}"
//
// ```
//
// ### **Best-Effort Data Collection**
//
// ```yaml
// id: gather-market-intelligence
// type: parallel
// strategy: best_effort  # Collect what we can, don't fail on errors
// max_workers: 8         # Limit to avoid overwhelming external APIs
// timeout: 45s           # Some sources may be slow
//
// tasks:
//
//   - id: competitor-analysis
//     type: basic
//     agent: { id: competitor-analyzer }
//     with:
//     industry: "{{ .workflow.input.industry }}"
//     competitors: "{{ .workflow.input.competitor_list }}"
//
//   - id: social-sentiment
//     type: basic
//     tool: { id: social-media-scraper }
//     with:
//     keywords: "{{ .workflow.input.brand_keywords }}"
//     timeframe: "7d"
//
//   - id: price-monitoring
//     type: basic
//     tool: { id: price-tracker }
//     with:
//     products: "{{ .workflow.input.product_list }}"
//
//   - id: news-analysis
//     type: basic
//     agent: { id: news-analyzer }
//     with:
//     topics: "{{ .workflow.input.industry_topics }}"
//     sources: ["reuters", "bloomberg", "techcrunch"]
//
// # Outputs available even if some tasks fail
// outputs:
//
//	market_intelligence:
//	  competitor_data: "{{ .tasks.competitor-analysis.outputs | default({}) }}"
//	  social_sentiment: "{{ .tasks.social-sentiment.outputs | default({}) }}"
//	  pricing_data: "{{ .tasks.price-monitoring.outputs | default({}) }}"
//	  news_insights: "{{ .tasks.news-analysis.outputs | default({}) }}"
//	  collection_success_rate: "{{ .parallel.success_count / .parallel.total_count }}"
//
// ```
//
// ### **Race for Speed**
//
// ```yaml
// id: fast-weather-lookup
// type: parallel
// strategy: race         # First successful response wins
// max_workers: 0         # All sources compete simultaneously
// timeout: 10s           # Quick timeout for speed
//
// tasks:
//
//   - id: primary-weather-api
//     type: basic
//     tool:
//     id: weather-api
//     config:
//     provider: openweather
//     endpoint: "https://api.openweathermap.org/data/2.5/weather"
//     with:
//     location: "{{ .workflow.input.location }}"
//
//   - id: backup-weather-api
//     type: basic
//     tool:
//     id: weather-api
//     config:
//     provider: weatherapi
//     endpoint: "https://api.weatherapi.com/v1/current.json"
//     with:
//     location: "{{ .workflow.input.location }}"
//
//   - id: cached-weather-data
//     type: basic
//     tool: { id: cache-lookup }
//     with:
//     cache_key: "weather:{{ .workflow.input.location }}"
//     max_age: 3600  # 1 hour cache
//
// outputs:
//
//	weather: "{{ .race.winner.output }}"
//	data_source: "{{ .race.winner.task_id }}"
//	response_time: "{{ .race.winner.duration }}"
//
// ```
//
// ## ⚙️ Configuration Deep Dive
//
// ### **Worker Limits (`max_workers`)**
//
// | Setting | Behavior | Best For |
// |---------|----------|----------|
// | `0` (default) | No limit, all tasks run | Small task counts (< 10) |
// | `2-5` | Conservative limiting | External API calls |
// | `5-20` | Moderate parallelism | CPU-bound operations |
// | `20+` | High parallelism | I/O-bound operations |
//
// ### **Timeout Management**
//
// ```yaml
// timeout: 30s           # Global timeout for all tasks
// tasks:
//   - id: slow-task
//     timeout: 60s       # Override for specific task
//   - id: fast-task
//     timeout: 5s        # Quick timeout for fast operations
//
// ```
//
// ## 💡 Performance Best Practices
//
// ### **Task Independence**
// - ✅ Ensure tasks don't depend on each other's outputs
// - ✅ Use shared inputs from workflow context
// - ❌ Avoid task-to-task dependencies within parallel blocks
//
// ### **Resource Management**
// - ✅ Set `max_workers` for external API calls
// - ✅ Use timeouts to prevent hung tasks
// - ✅ Monitor resource usage in production
//
// ### **Error Handling**
// - ✅ Choose appropriate strategy for your use case
// - ✅ Implement fallbacks for critical operations
// - ✅ Log partial results in `best_effort` mode
type ParallelTask struct {
	// Strategy determines how the parallel execution handles task completion
	// Defaults to "wait_all" if not specified
	// Options: wait_all, fail_fast, best_effort, race
	Strategy ParallelStrategy `json:"strategy,omitempty"    yaml:"strategy,omitempty"    mapstructure:"strategy,omitempty"`
	// MaxWorkers limits the number of concurrent task executions
	// 0 means no limit (all tasks run concurrently)
	// - **Example**: 5 means at most 5 tasks run at the same time
	MaxWorkers int `json:"max_workers,omitempty" yaml:"max_workers,omitempty" mapstructure:"max_workers,omitempty"`
}

func (pt *ParallelTask) GetStrategy() ParallelStrategy {
	if pt.Strategy == "" {
		return StrategyWaitAll
	}
	return pt.Strategy
}

// -----------------------------------------------------------------------------
// Collection Task
// -----------------------------------------------------------------------------

// CollectionMode determines how collection items are processed
type CollectionMode string

const (
	// CollectionModeParallel processes all items concurrently
	// Subject to MaxWorkers limit if specified
	// Default mode for collection processing
	CollectionModeParallel CollectionMode = "parallel"
	// CollectionModeSequential processes items one at a time in order
	// Useful when items have dependencies or order matters
	// Items are processed in array order
	CollectionModeSequential CollectionMode = "sequential"
)

// ValidateCollectionMode checks if the given string is a valid CollectionMode.
// Used for configuration validation to ensure only valid modes are specified.
func ValidateCollectionMode(mode string) bool {
	switch CollectionMode(mode) {
	case CollectionModeParallel, CollectionModeSequential:
		return true
	default:
		return false
	}
}

// CollectionConfig defines how to iterate over a collection of items.
//
// Collection tasks are **dynamic task generators** that transform arrays into individual
// task executions. They're the powerhouse of batch processing in Compozy, enabling
// sophisticated iteration patterns with filtering, variable injection, and flexible
// execution modes. Think of them as intelligent for-loops that scale from simple
// transformations to complex parallel processing workflows.
//
// ## 🎯 When to Use Collection Tasks
//
// Collection tasks excel in scenarios involving array processing:
//
// | Use Case | Scale | Mode | Example |
// |----------|-------|------|---------|
// | **User Processing** | 100-10K users | Parallel | Account activation, data migration |
// | **Document Processing** | 10-1K docs | Parallel | PDF parsing, content extraction |
// | **Database Migrations** | 5-50 migrations | Sequential | Schema updates, data transformations |
// | **File Processing** | 100-5K files | Parallel | Image resizing, format conversion |
// | **API Synchronization** | 50-1K records | Batched | Third-party system updates |
// | **Report Generation** | 10-100 reports | Parallel | Multi-tenant report creation |
//
// ## ⚙️ How Collection Tasks Work
//
// Collection tasks follow a sophisticated execution pipeline:
//
// ```
// Array Input → Filter → Item Generation → Task Execution → Result Collection
//
//	  ↓            ↓            ↓              ↓               ↓
//	Original    Filtered     Task per        Parallel/       Aggregated
//	Items       Items        Item            Sequential      Results
//
// ```
//
// ### **Execution Flow Details**
// 1. **📋 Items Resolution**: Template expression evaluates to array
// 2. **🔍 Optional Filtering**: CEL expressions filter items
// 3. **⚙️ Task Generation**: One task instance per filtered item
// 4. **💉 Variable Injection**: Each task gets `item`, `index`, and custom variables
// 5. **🚀 Execution**: Run according to mode (parallel/sequential) and batching
// 6. **📊 Result Collection**: Aggregate outputs from all task instances
//
// ## 📝 Comprehensive Examples
//
// ### **User Account Processing**
//
// ```yaml
// id: process-user-accounts
// type: collection
// items: "{{ .workflow.input.users }}"
// filter: "item.status == 'pending' && item.created_at > '2024-01-01'"
// item_var: user          # Custom variable name
// index_var: position     # Custom index name
// mode: parallel
// batch: 20               # Process 20 users at a time
// max_workers: 5          # Limit concurrent operations
// strategy: best_effort   # Continue even if some fail
//
// task:
//
//	id: "process-user-{{ .position }}"
//	type: basic
//	agent:
//	  id: user-processor
//	  model: claude-3-5-haiku-latest
//	  instructions: |
//	    Process user account activation:
//	    1. Validate user data
//	    2. Generate welcome email
//	    3. Create user profile
//	    4. Set up default preferences
//	with:
//	  user_data:
//	    id: "{{ .user.id }}"
//	    email: "{{ .user.email }}"
//	    name: "{{ .user.full_name }}"
//	    signup_date: "{{ .user.created_at }}"
//	  processing_context:
//	    batch_position: "{{ .position }}"
//	    total_users: "{{ len(.workflow.input.users) }}"
//	    is_priority: "{{ .user.tier == 'premium' }}"
//	outputs:
//	  user_id: "{{ .task.output.user_id }}"
//	  welcome_sent: "{{ .task.output.email_sent }}"
//	  profile_created: "{{ .task.output.profile_id }}"
//
// # Aggregate results
// outputs:
//
//	processed_users: "{{ .collection.results }}"
//	success_count: "{{ .collection.success_count }}"
//	failure_count: "{{ .collection.failure_count }}"
//	processing_summary:
//	  total_processed: "{{ .collection.total_count }}"
//	  success_rate: "{{ .collection.success_count / .collection.total_count }}"
//	  failed_users: "{{ .collection.failed_items }}"
//
// ```
//
// ### **Document Processing Pipeline**
//
// ```yaml
// id: process-documents
// type: collection
// items: "{{ .workflow.input.document_queue }}"
// filter: "item.size_mb < 50 && item.type in ['pdf', 'docx', 'txt']"
// mode: parallel
// batch: 10
// strategy: fail_fast  # Stop if critical document fails
//
// task:
//
//	type: composite  # Multi-step processing per document
//	tasks:
//	  - id: extract-text
//	    type: basic
//	    tool:
//	      id: document-parser
//	      config:
//	        parser_type: "{{ .item.type }}"
//	    with:
//	      document_path: "{{ .item.file_path }}"
//	      options:
//	        extract_images: true
//	        preserve_formatting: true
//
//	  - id: analyze-content
//	    type: basic
//	    agent:
//	      id: content-analyzer
//	      model: claude-4-opus
//	      instructions: "Analyze document content and extract key information"
//	    with:
//	      text_content: "{{ .tasks.extract-text.output.text }}"
//	      document_metadata:
//	        filename: "{{ .item.filename }}"
//	        source: "{{ .item.source }}"
//
//	  - id: store-results
//	    type: basic
//	    tool:
//	      id: database-client
//	    with:
//	      table: "processed_documents"
//	      data:
//	        document_id: "{{ .item.id }}"
//	        extracted_text: "{{ .tasks.extract-text.output.text }}"
//	        analysis: "{{ .tasks.analyze-content.output }}"
//	        processed_at: "{{ now }}"
//
// ```
//
// ### **Sequential Database Migration**
//
// ```yaml
// id: database-migrations
// type: collection
// items: "{{ .workflow.input.pending_migrations }}"
// mode: sequential     # Must run in order
// strategy: fail_fast  # Stop on first failure
//
// task:
//
//	id: "migration-{{ .item.version }}"
//	type: basic
//	tool:
//	  id: database-migrator
//	  config:
//	    connection: "{{ .env.DATABASE_URL }}"
//	    backup_before: true
//	with:
//	  migration_sql: "{{ .item.sql_content }}"
//	  version: "{{ .item.version }}"
//	  description: "{{ .item.description }}"
//	  dependencies: "{{ .item.required_versions }}"
//	timeout: 300s  # 5 minutes per migration
//	on_error:
//	  next: rollback-migration
//	  retry: 0  # No retries for migrations
//
// outputs:
//
//	applied_migrations: "{{ .collection.results }}"
//	current_version: "{{ .collection.results[-1].version }}"
//	migration_log: "{{ .collection.execution_log }}"
//
// ```
//
// ### **Batch API Synchronization**
//
// ```yaml
// id: sync-customer-data
// type: collection
// items: "{{ .workflow.input.customers_to_sync }}"
// filter: "item.last_updated > workflow.input.since_timestamp"
// mode: parallel
// batch: 25           # API rate limiting
// max_workers: 3      # Conservative for external API
// strategy: best_effort  # Don't fail entire sync for one record
//
// task:
//
//	type: basic
//	tool:
//	  id: crm-api-client
//	  config:
//	    endpoint: "{{ .env.CRM_API_BASE }}/customers"
//	    timeout: 30s
//	    retry_attempts: 3
//	with:
//	  customer_id: "{{ .item.id }}"
//	  update_data:
//	    email: "{{ .item.email }}"
//	    phone: "{{ .item.phone }}"
//	    address: "{{ .item.address }}"
//	    preferences: "{{ .item.preferences }}"
//	    last_purchase: "{{ .item.last_order_date }}"
//	on_error:
//	  next: log-sync-failure
//	  retry: 2
//
// outputs:
//
//	sync_results:
//	  successful_syncs: "{{ .collection.success_count }}"
//	  failed_syncs: "{{ .collection.failure_count }}"
//	  sync_rate: "{{ .collection.success_count / .collection.total_count }}"
//	  failed_customer_ids: "{{ .collection.failed_items | map(.id) }}"
//
// ```
//
// ## 🔧 Advanced Configuration Options
//
// ### **Variable Customization**
// ```yaml
// item_var: customer      # Access as {{ .customer }}
// index_var: position     # Access as {{ .position }}
// ```
//
// ### **Filtering Expressions**
// ```yaml
// # Basic filtering
// filter: "item.active == true"
//
// # Complex conditions
// filter: |
//
//	item.priority == 'high' &&
//	item.created_at > workflow.input.cutoff_date &&
//	item.size_mb < 100
//
// # Multiple criteria
// filter: "item.status in ['pending', 'processing'] && item.retry_count < 3"
// ```
//
// ### **Batching Strategies**
//
// | Configuration | Behavior | Use Case |
// |---------------|----------|----------|
// | `batch: 0` | Process all items simultaneously | Small datasets (< 100 items) |
// | `batch: 10` | Process 10 items at a time | Medium datasets with rate limits |
// | `batch: 1` | Process one item at a time | Sequential processing |
// | `batch: 100` | Large batches | High-throughput scenarios |
//
// ### **Performance Tuning**
// ```yaml
// # Conservative (external APIs)
// batch: 5
// max_workers: 2
// strategy: best_effort
//
// # Aggressive (internal processing)
// batch: 50
// max_workers: 10
// strategy: fail_fast
//
// # Sequential (dependencies)
// mode: sequential
// batch: 1
// strategy: fail_fast
// ```
//
// ## 📊 Collection Context Variables
//
// Within task templates, access these special variables:
//
// | Variable | Description | Example |
// |----------|-------------|---------|
// | `{{ .item }}` | Current item (or custom var) | `{{ .item.id }}` |
// | `{{ .index }}` | Zero-based position (or custom var) | `{{ .index }}` |
// | `{{ .workflow.* }}` | Workflow context | `{{ .workflow.input.config }}` |
// | `{{ .collection.total }}` | Total items count | `{{ .collection.total }}` |
// | `{{ .collection.filtered }}` | Filtered items count | `{{ .collection.filtered }}` |
//
// ## 💡 Best Practices
//
// ### **Performance Optimization**
// - ✅ Use `batch` to control resource usage
// - ✅ Set `max_workers` for external API calls
// - ✅ Choose appropriate `strategy` for your use case
// - ✅ Use `filter` to reduce unnecessary processing
//
// ### **Error Handling**
// - ✅ Use `best_effort` for non-critical batch operations
// - ✅ Use `fail_fast` for critical sequential operations
// - ✅ Implement proper timeout and retry logic
// - ✅ Log failed items for later reprocessing
//
// ### **Data Management**
// - ✅ Use meaningful variable names (`item_var`, `index_var`)
// - ✅ Structure outputs for easy aggregation
// - ✅ Include metadata for debugging and monitoring
type CollectionConfig struct {
	// Items is a template expression that evaluates to an array
	// The expression should resolve to a list of items to iterate over
	// - **Example**: "{{ .workflow.input.users }}" or "{{ range(1, 10) }}"
	Items string `json:"items"               yaml:"items"               mapstructure:"items"`
	// Filter is an optional CEL expression to filter items before processing
	// Each item is available as 'item' in the expression
	// - **Example**: "item.status != 'inactive'" or "item.age > 18"
	Filter string `json:"filter,omitempty"    yaml:"filter,omitempty"    mapstructure:"filter,omitempty"`
	// ItemVar is the variable name for the current item (default: "item")
	// Available in task templates as {{ .item }} or custom name
	// - **Example**: Set to "user" to access as {{ .user }} in templates
	ItemVar string `json:"item_var,omitempty"  yaml:"item_var,omitempty"  mapstructure:"item_var,omitempty"`
	// IndexVar is the variable name for the current index (default: "index")
	// Available in task templates as {{ .index }} or custom name
	// Zero-based index of the current item
	IndexVar string `json:"index_var,omitempty" yaml:"index_var,omitempty" mapstructure:"index_var,omitempty"`
	// Mode determines if items are processed in parallel or sequentially
	// Defaults to "parallel"
	// Options: parallel, sequential
	Mode CollectionMode `json:"mode,omitempty"      yaml:"mode,omitempty"      mapstructure:"mode,omitempty"`
	// Batch size for processing items in groups (0 = no batching)
	// Useful for rate limiting or managing resource usage
	// - **Example**: 10 means process 10 items at a time
	Batch int `json:"batch,omitempty"     yaml:"batch,omitempty"     mapstructure:"batch,omitempty"`
}

// Default sets sensible defaults for collection configuration.
// Mode defaults to parallel, item variable to "item", and index variable to "index".
func (cc *CollectionConfig) Default() {
	if cc.Mode == "" {
		cc.Mode = CollectionModeParallel
	}
	if cc.ItemVar == "" {
		cc.ItemVar = "item"
	}
	if cc.IndexVar == "" {
		cc.IndexVar = "index"
	}
	if cc.Batch == 0 {
		cc.Batch = 0 // Keep 0 as default (no batching)
	}
}

// GetItemVar returns the item variable name
func (cc *CollectionConfig) GetItemVar() string {
	return cc.ItemVar
}

// GetIndexVar returns the index variable name
func (cc *CollectionConfig) GetIndexVar() string {
	return cc.IndexVar
}

// GetMode returns the collection mode
func (cc *CollectionConfig) GetMode() CollectionMode {
	return cc.Mode
}

// -----------------------------------------------------------------------------
// Signal Task
// -----------------------------------------------------------------------------

// SignalTask sends signals to coordinate between tasks and workflows.
//
// Signal tasks are the **event broadcasters** of Compozy's event-driven architecture,
// enabling sophisticated coordination patterns across workflows, tasks, and external
// systems. They transform simple task completion into powerful pub/sub messaging,
// allowing workflows to communicate, synchronize, and orchestrate complex multi-stage
// processes with loose coupling and high resilience.
//
// ## 🎯 When to Use Signal Tasks
//
// Signal tasks excel in event-driven coordination scenarios:
//
// | Use Case | Pattern | Example |
// |----------|---------|---------|
// | **Process Completion** | Notify → Wait | Order processing → Email dispatch |
// | **Cross-Workflow Sync** | Signal → Multiple Waiters | Data ready → Multiple processors |
// | **Human-in-Loop** | Process → Signal → Approval | Document ready → Manager approval |
// | **External Integration** | Internal → Signal → Webhook | Task done → Third-party notification |
// | **Pipeline Stages** | Stage Complete → Signal → Next Stage | ETL phases coordination |
// | **Error Escalation** | Failure → Signal → Alert | Critical error → On-call notification |
//
// ## ⚙️ How Signal Broadcasting Works
//
// Signal tasks follow a reliable broadcast pattern:
//
// ```
// Signal Task → Event Bus → Multiple Waiters → Parallel Processing
//
//	   ↓              ↓           ↓                    ↓
//	Broadcast      Delivery   Condition         Continued
//	Event          Queue      Evaluation        Execution
//
// ```
//
// ## 📝 Practical Examples
//
// ### **Order Processing Coordination**
//
// ```yaml
// id: process-order-completion
// type: signal
// signal:
//
//	id: order-processed-{{ .workflow.input.order.id }}
//	payload:
//	  order_id: "{{ .workflow.input.order.id }}"
//	  customer_id: "{{ .workflow.input.order.customer_id }}"
//	  status: completed
//	  processed_at: "{{ now }}"
//	  total_amount: "{{ .tasks.process-payment.output.charged_amount }}"
//	  shipping_required: "{{ .workflow.input.order.requires_shipping }}"
//	  items_processed: "{{ .tasks.update-inventory.output.updated_items }}"
//	  payment_reference: "{{ .tasks.process-payment.output.transaction_id }}"
//	  estimated_delivery: "{{ .tasks.update-inventory.output.delivery_estimate }}"
//
// ```
//
// ### **Multi-Service Data Pipeline**
//
// ```yaml
// id: signal-data-ready
// type: signal
// signal:
//
//	id: customer-data-ready-{{ .workflow.input.batch_id }}
//	payload:
//	  batch_id: "{{ .workflow.input.batch_id }}"
//	  data_location: "{{ .tasks.extract-data.output.file_path }}"
//	  record_count: "{{ .tasks.extract-data.output.total_records }}"
//	  data_quality_score: "{{ .tasks.validate-data.output.quality_score }}"
//	  schema_version: "{{ .tasks.validate-data.output.schema_version }}"
//	  processing_timestamp: "{{ now }}"
//	  data_hash: "{{ .tasks.extract-data.output.data_checksum }}"
//	  contains_pii: "{{ .tasks.validate-data.output.has_personal_data }}"
//	  region: "{{ .workflow.input.data_region }}"
//
// ```
//
// ### **Human Approval Workflow**
//
// ```yaml
// id: signal-approval-required
// type: signal
// condition: "tasks.analyze-document.output.requires_approval == true"
// signal:
//
//	id: document-approval-required-{{ .workflow.input.document_id }}
//	payload:
//	  document_id: "{{ .workflow.input.document_id }}"
//	  document_type: "{{ .tasks.analyze-document.output.classification }}"
//	  sensitivity_level: "{{ .tasks.analyze-document.output.sensitivity }}"
//	  required_approver_level: "{{ .tasks.analyze-document.output.approver_level }}"
//	  analysis_summary: "{{ .tasks.analyze-document.output.summary }}"
//	  compliance_flags: "{{ .tasks.analyze-document.output.compliance_issues }}"
//	  submitted_by: "{{ .workflow.input.submitter_id }}"
//	  submitted_at: "{{ .workflow.input.submission_time }}"
//	  review_deadline: "{{ .workflow.input.submission_time | date_add('72h') }}"
//
// ```
//
// ### **Error Escalation System**
//
// ```yaml
// id: signal-critical-failure
// type: signal
// signal:
//
//	id: critical-process-failed
//	payload:
//	  process_id: "{{ .workflow.id }}"
//	  success: false
//	  failed_at: "{{ now }}"
//	  error_type: "{{ .tasks.process-critical-data.error.type }}"
//	  error_message: "{{ .tasks.process-critical-data.error.message }}"
//	  retry_count: "{{ .tasks.process-critical-data.retry_count }}"
//	  escalation_required: true
//	  on_call_team: data-engineering
//	  severity: critical
//
// ```
//
// ## 📡 Signal Naming Best Practices
//
// ### **Naming Conventions**
// | Pattern | Example | Use Case |
// |---------|---------|----------|
// | **Descriptive Actions** | `user-activated`, `payment-completed` | Clear event identification |
// | **Entity-Specific** | `order-{{ .order_id }}-shipped` | Targeted notifications |
// | **Hierarchical** | `billing.invoice.generated` | System organization |
// | **Status-Based** | `process-failed`, `validation-passed` | State transitions |
//
// ### **Signal ID Examples**
// ```yaml
// # ✅ Good signal IDs
// id: user-registration-completed
// id: order-{{ .order_id }}-payment-processed
// id: document-{{ .doc_id }}-approval-required
// id: billing.subscription.cancelled.{{ .customer_id }}
// id: system.backup.completed.{{ .backup_type }}
//
// # ❌ Avoid these patterns
// id: signal1                    # Not descriptive
// id: done                       # Too generic
// id: event                      # Meaningless
// ```
type SignalTask struct {
	// Signal configuration containing the signal ID and payload
	Signal *SignalConfig `json:"signal,omitempty" yaml:"signal,omitempty" mapstructure:"signal,omitempty"`
}

// SignalConfig defines the signal to be sent
type SignalConfig struct {
	// ID is the unique identifier for the signal
	// Wait tasks with matching wait_for values will receive this signal
	// - **Example**: "user-approved", "payment-completed", "data-ready"
	ID string `json:"id"                yaml:"id"                mapstructure:"id"`
	// Payload contains data to send with the signal
	// This data is available to the receiving wait task for processing
	// Can be any JSON-serializable data structure
	// - **Example**: { "user_id": "123", "status": "approved", "timestamp": "2024-01-01T00:00:00Z" }
	Payload map[string]any `json:"payload,omitempty" yaml:"payload,omitempty" mapstructure:"payload,omitempty"`
}

// -----------------------------------------------------------------------------
// Wait Task
// -----------------------------------------------------------------------------

// WaitTask pauses workflow execution until a specific condition is met.
//
// Wait tasks are the receiving end of Compozy's event-driven architecture. They pause
// workflow execution until they receive a signal that matches their criteria, enabling
// synchronization between independent processes, human-in-the-loop patterns, and
// event-driven orchestration.
//
// ## When to Use Wait Tasks
//
// Use wait tasks when you need to:
// - **Wait for external events** (webhooks, user approvals, system notifications)
// - **Synchronize parallel processes** that complete at different times
// - **Implement timeouts** for time-sensitive operations
// - **Create approval workflows** requiring human intervention
//
// ## How Wait Tasks Work
//
// 1. **Task pauses** execution and listens for the specified signal
// 2. **Condition evaluation** checks each received signal
// 3. **Optional processing** transforms signal data before continuing
// 4. **Timeout handling** provides fallback for missing signals
//
// ## Example: Approval Workflow
//
// ```yaml
// type: wait
// wait_for: user-approval-{{ .workflow.input.request_id }}
// condition: |
//
//	signal.payload.approved == true &&
//	signal.payload.approver_level >= input.required_level
//
// timeout: 24h
// on_timeout: escalate-approval
// ```
//
// ## Example: Multi-Service Coordination
//
// ```yaml
// type: wait
// wait_for: service-ready
// condition: |
//
//	signal.payload.service_name == input.expected_service &&
//	signal.payload.status == 'healthy'
//
// processor:
//
//	type: basic
//	agent:
//	  id: config-updater
//	with:
//	  endpoint: "{{ signal.payload.endpoint }}"
//	  credentials: "{{ signal.payload.auth }}"
//
// ```
//
// ## Condition Expressions
//
// Wait conditions use CEL with access to:
// - `signal`: The received signal data
// - `input`: The wait task's input parameters
// - `workflow`: The workflow context
//
// ## Timeout Strategies
//
// ```yaml
// timeout: 5m
// on_timeout: handle-timeout  # Task to execute on timeout
// # OR
// on_error:
//
//	next: timeout-handler
//	retry: 0  # Don't retry timeouts
//
// ```
type WaitTask struct {
	// WaitFor specifies the signal ID to wait for
	// The task will pause until a signal with this ID is received
	// Must match the ID used in a SignalTask
	// - **Example**: "user-approved", "payment-completed"
	WaitFor string `json:"wait_for,omitempty"   yaml:"wait_for,omitempty"   mapstructure:"wait_for,omitempty"`
	// Processor is an optional task configuration to process received signals
	// Allows custom handling of signal data before continuing
	// The processor receives the signal payload as input
	// $ref: inline:#
	Processor *Config `json:"processor,omitempty"  yaml:"processor,omitempty"  mapstructure:"processor,omitempty"`
	// OnTimeout specifies the next task to execute if the wait times out
	// Uses the timeout value from BaseConfig
	// If not specified, the task fails on timeout
	OnTimeout string `json:"on_timeout,omitempty" yaml:"on_timeout,omitempty" mapstructure:"on_timeout,omitempty"`
}

// -----------------------------------------------------------------------------
// Memory Task
// -----------------------------------------------------------------------------

// MemoryOpType defines the type of operation to perform on memory.
//
// Memory operations provide persistent state management across tasks and workflows.
// Each operation type serves a specific purpose in managing shared data.
//
// ## Operation Types
//
// | Operation | Purpose | Requires Payload |
// |-----------|---------|------------------|
// | `read` | Retrieve stored data | No |
// | `write` | Store/overwrite data | Yes |
// | `append` | Add to existing data | Yes |
// | `delete` | Remove a key | No |
// | `flush` | Clean up old data | No |
// | `health` | Check memory health | No |
// | `stats` | Get usage statistics | No |
// | `clear` | Remove multiple keys | No |
type MemoryOpType string

const (
	// MemoryOpRead retrieves data from a memory key
	// Returns the stored value or null if key doesn't exist
	MemoryOpRead MemoryOpType = "read"
	// MemoryOpWrite stores data to a memory key (overwrites existing)
	// Creates the key if it doesn't exist
	MemoryOpWrite MemoryOpType = "write"
	// MemoryOpAppend adds data to existing memory content
	// For arrays: adds elements, for strings: concatenates, for objects: merges
	MemoryOpAppend MemoryOpType = "append"
	// MemoryOpDelete removes a specific memory key
	// No error if key doesn't exist
	MemoryOpDelete MemoryOpType = "delete"
	// MemoryOpFlush triggers memory cleanup based on strategy
	// Removes old or excess data according to configured rules
	MemoryOpFlush MemoryOpType = "flush"
	// MemoryOpHealth checks memory system health and connectivity
	// Returns status and diagnostic information
	MemoryOpHealth MemoryOpType = "health"
	// MemoryOpClear removes all data matching a pattern (requires confirmation)
	// DANGEROUS: Can delete large amounts of data
	MemoryOpClear MemoryOpType = "clear"
	// MemoryOpStats retrieves memory usage statistics
	// Returns counts, sizes, and other metrics
	MemoryOpStats MemoryOpType = "stats"
)

// MemoryTask performs operations on shared memory stores.
//
// Memory tasks provide persistent state management for Compozy workflows. They enable
// data sharing between tasks, state persistence across workflow executions, and
// implementation of stateful patterns like caching, session management, and counters.
// Memory operations work with backend stores (Redis, DynamoDB, etc.) configured at
// the project level.
//
// ## When to Use Memory Tasks
//
// Use memory tasks when you need to:
// - **Share state** between tasks or workflows
// - **Cache results** of expensive operations
// - **Track progress** across long-running processes
// - **Implement counters** or accumulative operations
// - **Store session data** for multi-step interactions
//
// ## Example: User Session Management
//
// ```yaml
// # Write session data
// type: memory
// operation: write
// memory_ref: user-sessions
// key_template: "session:{{ .workflow.input.user_id }}"
// payload:
//
//	last_activity: "{{ now }}"
//	preferences: "{{ .tasks.get_preferences.output }}"
//	cart_items: "{{ .workflow.input.cart }}"
//
// ```
//
// ## Example: Distributed Counter
//
// ```yaml
// # Increment counter
// type: memory
// operation: append
// memory_ref: counters
// key_template: "processed:{{ .workflow.input.date }}"
// payload: 1  # Increment by 1
// ```
//
// ## Example: Cache Management
//
// ```yaml
// # Read from cache
//   - id: check-cache
//     type: memory
//     operation: read
//     memory_ref: api-cache
//     key_template: "weather:{{ .input.city }}:{{ .input.date }}"
//
// # Write to cache if miss
//   - id: update-cache
//     type: memory
//     operation: write
//     memory_ref: api-cache
//     key_template: "weather:{{ .input.city }}:{{ .input.date }}"
//     payload: "{{ .tasks.fetch_weather.output }}"
//     condition: "tasks.check_cache.output == null"
//
// ```
//
// ## Key Templates
//
// Key templates support dynamic key generation:
// - Use template variables: `"user:{{ .workflow.input.user_id }}"`
// - Include timestamps: `"backup:{{ .workflow.id }}:{{ now }}"`
// - Create hierarchies: `"org:{{ .org_id }}:dept:{{ .dept_id }}:user:{{ .user_id }}"`
//
// ## Memory References
//
// The `memory_ref` field references a memory configuration defined in your project:
//
// ```yaml
// # project.yaml
// memories:
//   - id: user-sessions
//     provider: redis
//     config:
//     url: redis://localhost:6379
//     ttl: 3600
//
// ```
type MemoryTask struct {
	// Operation type to perform on memory
	// Required field that determines the action to take
	Operation MemoryOpType `json:"operation"     yaml:"operation"     mapstructure:"operation"`
	// MemoryRef identifies which memory store to use
	// References a memory configuration defined at the project level
	// - **Example**: "user-sessions", "workflow-state", "cache"
	MemoryRef string `json:"memory_ref"    yaml:"memory_ref"    mapstructure:"memory_ref"`
	// KeyTemplate is a template expression for the memory key
	// Supports template variables for dynamic key generation
	// - **Example**: "user:{{ .workflow.input.user_id }}:profile"
	KeyTemplate string `json:"key_template"  yaml:"key_template"  mapstructure:"key_template"`
	// Payload data for write/append operations
	// Can be any JSON-serializable data structure
	// Required for write and append operations
	Payload any `json:"payload"       yaml:"payload"       mapstructure:"payload,omitempty"`
	// BatchSize for operations that process multiple keys
	// Controls how many keys are processed in each batch
	// Default: 100, Maximum: 10,000
	BatchSize int `json:"batch_size"    yaml:"batch_size"    mapstructure:"batch_size,omitempty"`
	// MaxKeys limits the number of keys processed
	// Safety limit to prevent runaway operations
	// Default: 1,000, Maximum: 50,000
	MaxKeys int `json:"max_keys"      yaml:"max_keys"      mapstructure:"max_keys,omitempty"`
	// Configuration for flush operations
	// Only used when operation is "flush"
	FlushConfig *FlushConfig `json:"flush_config"  yaml:"flush_config"  mapstructure:"flush_config,omitempty"`
	// Configuration for health check operations
	// Only used when operation is "health"
	HealthConfig *HealthConfig `json:"health_config" yaml:"health_config" mapstructure:"health_config,omitempty"`
	// Configuration for statistics operations
	// Only used when operation is "stats"
	StatsConfig *StatsConfig `json:"stats_config"  yaml:"stats_config"  mapstructure:"stats_config,omitempty"`
	// Configuration for clear operations
	// Only used when operation is "clear"
	ClearConfig *ClearConfig `json:"clear_config"  yaml:"clear_config"  mapstructure:"clear_config,omitempty"`
}

// FlushConfig controls memory flushing behavior
type FlushConfig struct {
	// Strategy for selecting keys to flush
	// Options: "simple_fifo" (oldest first), "lru" (least recently used)
	// Default: "simple_fifo"
	Strategy string `json:"strategy"  yaml:"strategy"  mapstructure:"strategy"`
	// Maximum number of keys to flush in one operation
	// Default: 100
	MaxKeys int `json:"max_keys"  yaml:"max_keys"  mapstructure:"max_keys"`
	// DryRun simulates flush without actually removing data
	// Useful for testing what would be removed
	DryRun bool `json:"dry_run"   yaml:"dry_run"   mapstructure:"dry_run"`
	// Force flush even if below threshold
	// Bypasses normal threshold checks
	Force bool `json:"force"     yaml:"force"     mapstructure:"force"`
	// Threshold (0-1) for triggering flush based on memory usage
	// - **Example**: 0.8 means flush when 80% full
	Threshold float64 `json:"threshold" yaml:"threshold" mapstructure:"threshold"`
}

// HealthConfig controls health check behavior
type HealthConfig struct {
	// IncludeStats adds memory statistics to health check results
	// Provides additional diagnostic information
	IncludeStats bool `json:"include_stats"      yaml:"include_stats"      mapstructure:"include_stats"`
	// CheckConnectivity verifies connection to memory backend
	// Tests actual read/write operations
	CheckConnectivity bool `json:"check_connectivity" yaml:"check_connectivity" mapstructure:"check_connectivity"`
}

// StatsConfig controls statistics gathering
type StatsConfig struct {
	// IncludeContent includes actual memory content in stats
	// WARNING: May return large amounts of data
	IncludeContent bool `json:"include_content" yaml:"include_content" mapstructure:"include_content"`
	// GroupBy field for aggregating statistics
	// - **Example**: "user", "session", "workflow"
	// Groups stats by the specified field in stored data
	GroupBy string `json:"group_by"        yaml:"group_by"        mapstructure:"group_by"`
}

// ClearConfig controls memory clearing behavior
type ClearConfig struct {
	// Confirm must be true to execute clear operation
	// Required safety check to prevent accidental data loss
	Confirm bool `json:"confirm" yaml:"confirm" mapstructure:"confirm"`
	// Backup data before clearing
	// Implementation-dependent, may not be available for all backends
	Backup bool `json:"backup"  yaml:"backup"  mapstructure:"backup"`
}

// -----------------------------------------------------------------------------
// Config
// -----------------------------------------------------------------------------

// Config is the main task configuration structure in Compozy.
//
// Tasks are the **fundamental building blocks** of Compozy's workflow orchestration
// engine, representing atomic units of work that can be composed into sophisticated
// AI-powered automation pipelines. They provide a declarative, type-safe, and highly
// flexible foundation for building everything from simple single-step operations to
// complex multi-stage workflows with parallel processing, event coordination, and
// intelligent decision-making capabilities.
//
// ## 🏗️ Core Architecture Concepts
//
// ### **What Makes Tasks Powerful**
//
// Compozy tasks provide enterprise-grade capabilities:
//
// | Capability | Description | Business Value |
// |------------|-------------|----------------|
// | **🧩 Composable** | Build complex workflows from simple blocks | Rapid development, reusability |
// | **🔒 Type-Safe** | JSON Schema validation for inputs/outputs | Reliable data flow, fewer bugs |
// | **👁️ Observable** | Track execution, monitor performance | Production readiness, debugging |
// | **♻️ Reusable** | Reference tasks across workflows | Code efficiency, consistency |
// | **🧪 Testable** | Execute tasks in isolation with mock data | Quality assurance, CI/CD |
// | **⚡ Scalable** | Parallel execution and resource management | High throughput, efficiency |
// | **🎯 Intelligent** | AI agents with sophisticated reasoning | Smart automation, adaptability |
//
// ### **Task Execution Pipeline**
//
// ```
// 📥 Input Data → ✅ Validation → ⚙️ Execution → 📤 Output → 🔀 Transition
//
//	    ↓              ↓             ↓           ↓          ↓
//	Template        JSON Schema   Agent/Tool   Transform   Next Task
//	Variables       Validation    Processing   Results     On Success/Error
//
// ```
//
// ## 🎯 Task Orchestration Capabilities
//
// | Category | Capability | Implementation | Use Cases |
// |----------|------------|----------------|-----------|
// | **🤖 AI Processing** | LLM operations | Agent tasks | Content analysis, generation, classification |
// | **🔧 Tool Execution** | Deterministic ops | Tool tasks | API calls, file operations, data processing |
// | **🔀 Flow Control** | Conditional routing | Router tasks | Decision trees, approval routing |
// | **⚡ Parallelization** | Concurrent execution | Parallel tasks | Batch processing, data enrichment |
// | **🔄 Iteration** | Collection processing | Collection tasks | Array transformation, bulk operations |
// | **💾 State Management** | Persistent memory | Memory tasks | Session data, caching, counters |
// | **📡 Event Coordination** | Signal/wait patterns | Signal/Wait tasks | Approval workflows, external integration |
// | **🛡️ Error Handling** | Retries and fallbacks | Error transitions | Resilient operations, graceful degradation |
//
// ## 📋 Complete Customer Feedback Analysis Example
//
// This comprehensive example demonstrates advanced task configuration patterns:
//
// ```yaml
// # Advanced task configuration with full feature set
// id: analyze-customer-feedback
// type: basic
// resource: "compozy:task:sentiment-analyzer"
//
// # AI agent configuration
// agent:
//
//	id: advanced-sentiment-analyzer
//	provider: anthropic
//	model: claude-4-opus
//	instructions: |
//	  You are an expert customer feedback analyst. Analyze the provided feedback with these objectives:
//
//	  ## Analysis Framework
//	  1. **Sentiment Analysis**: Determine overall sentiment (positive/neutral/negative) with confidence score
//	  2. **Theme Extraction**: Identify key themes, topics, and pain points mentioned
//	  3. **Urgency Assessment**: Evaluate urgency level (critical/high/medium/low) based on language and content
//	  4. **Actionable Insights**: Provide specific, actionable recommendations for improvement
//	  5. **Customer Intent**: Understand what the customer is trying to achieve
//
//	  ## Output Format
//	  Provide structured analysis with quantifiable metrics and clear recommendations.
//
// # Comprehensive input validation
// input:
//
//	type: object
//	properties:
//	  feedback:
//	    type: string
//	    minLength: 10
//	    maxLength: 5000
//	    description: "Customer feedback text"
//	  customer_id:
//	    type: string
//	    pattern: "^[A-Z0-9]{8}$"
//	    description: "Unique customer identifier"
//	  feedback_metadata:
//	    type: object
//	    properties:
//	      source: { type: string, enum: ["email", "chat", "survey", "phone"] }
//	      timestamp: { type: string, format: "date-time" }
//	      customer_tier: { type: string, enum: ["free", "premium", "enterprise"] }
//	      product_area: { type: string }
//	    required: ["source", "timestamp"]
//	required: [feedback, customer_id, feedback_metadata]
//
// # Rich task parameters with context
// with:
//
//	feedback_content: "{{ .workflow.input.feedback_text }}"
//	customer_profile:
//	  id: "{{ .workflow.input.customer.id }}"
//	  name: "{{ .workflow.input.customer.name }}"
//	  tier: "{{ .workflow.input.customer.tier }}"
//	  account_age: "{{ .workflow.input.customer.account_age_days }}"
//	contextual_data:
//	  source_channel: "{{ .workflow.input.feedback_metadata.source }}"
//	  submission_time: "{{ .workflow.input.feedback_metadata.timestamp }}"
//	  product_context: "{{ .workflow.input.feedback_metadata.product_area }}"
//	  recent_interactions: "{{ .tasks.get_customer_history.output.recent_interactions }}"
//	  account_status: "{{ .tasks.get_customer_status.output.status }}"
//	analysis_config:
//	  include_sentiment_confidence: true
//	  extract_feature_requests: true
//	  identify_competitors: true
//	  suggest_followup_actions: true
//
// # Comprehensive output mapping
// outputs:
//
//	analysis_results:
//	  overall_sentiment: "{{ .task.output.sentiment.classification }}"
//	  sentiment_confidence: "{{ .task.output.sentiment.confidence_score }}"
//	  urgency_level: "{{ .task.output.urgency.level }}"
//	  urgency_justification: "{{ .task.output.urgency.reasoning }}"
//	extracted_insights:
//	  key_themes: "{{ .task.output.themes }}"
//	  pain_points: "{{ .task.output.pain_points }}"
//	  feature_requests: "{{ .task.output.feature_requests }}"
//	  competitive_mentions: "{{ .task.output.competitor_analysis }}"
//	recommendations:
//	  immediate_actions: "{{ .task.output.recommendations.immediate }}"
//	  followup_required: "{{ .task.output.recommendations.followup_needed }}"
//	  escalation_required: "{{ .task.output.urgency.level == 'critical' }}"
//	  suggested_response: "{{ .task.output.recommendations.response_template }}"
//	metadata:
//	  analysis_timestamp: "{{ now }}"
//	  processing_duration: "{{ .task.execution_time }}"
//	  model_version: "{{ .task.agent.model }}"
//	  confidence_metrics: "{{ .task.output.confidence_breakdown }}"
//
// # Advanced flow control
// on_success:
//
//	next: route-feedback-response
//	with:
//	  analysis_data: "{{ .task.outputs.analysis_results }}"
//	  recommendations: "{{ .task.outputs.recommendations }}"
//	  customer_context: "{{ .workflow.input.customer }}"
//
// # Sophisticated error handling
// on_error:
//
//	next: feedback-analysis-fallback
//	retry: 3
//	backoff: exponential
//	with:
//	  error_context:
//	    original_input: "{{ .workflow.input }}"
//	    failure_reason: "{{ .task.error.message }}"
//	    retry_attempt: "{{ .task.retry_count }}"
//
// # Additional controls
// timeout: 60s
// condition: "input.feedback_metadata.source != 'spam'"
// sleep: 2s
// env:
//
//	ANALYSIS_MODE: detailed
//	LOG_LEVEL: info
//
// ```
//
// ## 📊 Task Types Architecture
//
// Compozy provides a comprehensive suite of task types for different orchestration patterns:
//
// ### **🔧 Execution Task Types**
// - **`basic`** (default): Single agent or tool operation - the foundation of all workflows
// - **`composite`**: Sequential task grouping - reusable workflow components
// - **`aggregate`**: Result combination - consolidate outputs from multiple sources
//
// ### **🔀 Control Flow Task Types**
// - **`router`**: Conditional branching - intelligent decision-making based on data
// - **`parallel`**: Concurrent execution - high-performance batch processing
// - **`collection`**: Array iteration - scalable batch operations with filtering
//
// ### **📡 Coordination Task Types**
// - **`signal`**: Event emission - publish notifications and trigger coordination
// - **`wait`**: Event listening - pause execution until conditions are met
// - **`memory`**: State management - persistent data storage and retrieval
//
// ### **Task Type Selection Matrix**
//
// | Need | Recommended Type | Alternative Options |
// |------|------------------|-------------------|
// | **Single operation** | `basic` | - |
// | **Sequential steps** | `composite` | Multiple `basic` tasks |
// | **Conditional logic** | `router` | `basic` with conditions |
// | **Parallel processing** | `parallel` | `collection` (parallel mode) |
// | **Array processing** | `collection` | `parallel` with manual splitting |
// | **Result consolidation** | `aggregate` | `basic` with multiple inputs |
// | **Event coordination** | `signal` + `wait` | External messaging systems |
// | **State persistence** | `memory` | External databases |
//
// The task type determines which embedded configuration fields are active and how
// the task executor processes the configuration. Each type optimizes for specific
// orchestration patterns while maintaining consistent interfaces and behaviors.
type Config struct {
	// Embedded task-specific configurations
	// Only the fields relevant to the task type are used
	BasicTask        `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	RouterTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	ParallelTask     `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	CollectionConfig `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	SignalTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	WaitTask         `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	MemoryTask       `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	// Common configuration fields for all task types
	BaseConfig `         json:",inline"        yaml:",inline"        mapstructure:",squash"`
	// Tasks array for parallel, composite, and collection tasks
	// Contains the list of sub-tasks to execute
	// For parallel: tasks run concurrently
	// For composite: tasks run sequentially
	// For collection: not used (use Task field instead)
	// $ref: inline:#
	Tasks []Config `json:"tasks"          yaml:"tasks"          mapstructure:"tasks"          swaggerignore:"true"`
	// Task template for collection tasks
	// This configuration is replicated for each item in the collection
	// The item and index are available as template variables
	// $ref: inline:#
	Task *Config `json:"task,omitempty" yaml:"task,omitempty" mapstructure:"task,omitempty" swaggerignore:"true"`
}

func (t *Config) GetEnv() core.EnvMap {
	if t.Env == nil {
		t.Env = &core.EnvMap{}
		return *t.Env
	}
	return *t.Env
}

func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		t.With = &core.Input{}
	}
	return t.With
}

func (t *Config) GetAgent() *agent.Config {
	return t.Agent
}

func (t *Config) GetTool() *tool.Config {
	return t.Tool
}

func (t *Config) GetOutputs() *core.Input {
	return t.Outputs
}

// GetMaxWorkers returns the maximum number of workers for parallel execution.
// Used by parallel and collection tasks to limit concurrent operations.
// A value of 0 means no limit.
func (t *Config) GetMaxWorkers() int {
	return t.MaxWorkers
}

func (t *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, t.InputSchema, t.ID).Validate(ctx)
}

func (t *Config) ValidateOutput(ctx context.Context, output *core.Output) error {
	if t.OutputSchema == nil || output == nil {
		return nil
	}
	return schema.NewParamsValidator(output, t.OutputSchema, t.ID).Validate(ctx)
}

func (t *Config) HasSchema() bool {
	return t.InputSchema != nil || t.OutputSchema != nil
}

func (t *Config) Component() core.ConfigType {
	return core.ConfigTask
}

func (t *Config) GetFilePath() string {
	return t.FilePath
}

func (t *Config) SetFilePath(path string) {
	t.FilePath = path
}

func (t *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.CWD = CWD
	return nil
}

func (t *Config) GetCWD() *core.PathCWD {
	return t.CWD
}

func (t *Config) GetGlobalOpts() *core.GlobalOpts {
	return &t.Config
}

// Validate performs comprehensive validation of the task configuration.
// Checks task type validity, cycles in parallel tasks, and type-specific requirements.
func (t *Config) Validate(ctx context.Context) error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.CWD, t.ID),
		NewTaskTypeValidator(t),
	)
	if err := v.Validate(ctx); err != nil {
		return err
	}
	if len(t.Knowledge) > 1 {
		return fmt.Errorf("task configuration error: only one knowledge binding is supported in MVP")
	}
	if len(t.Knowledge) == 1 && strings.TrimSpace(t.Knowledge[0].ID) == "" {
		return fmt.Errorf("task configuration error: knowledge binding requires an id reference")
	}
	if t.Type == TaskTypeParallel {
		cycleValidator := NewCycleValidator()
		if err := cycleValidator.ValidateConfig(t); err != nil {
			return err
		}
	}
	if t.Type == TaskTypeWait {
		if err := t.validateWaitTask(ctx); err != nil {
			return fmt.Errorf("invalid wait task '%s': %w", t.ID, err)
		}
	}
	if err := t.validateBasicSelectorRules(ctx); err != nil {
		return err
	}
	if t.Type == TaskTypeMemory {
		if err := t.validateMemoryTask(ctx); err != nil {
			return fmt.Errorf("invalid memory task '%s': %w", t.ID, err)
		}
	}
	return nil
}

// validateBasicSelectorRules enforces mutual exclusivity and presence rules
// for basic task selectors across inline and explicit ref fields.
func (t *Config) validateBasicSelectorRules(_ context.Context) error {
	if t.Type != TaskTypeBasic {
		return nil
	}
	hasAgent := t.Agent != nil
	hasTool := t.Tool != nil
	if hasAgent && hasTool {
		return fmt.Errorf("basic task '%s': agent and tool are mutually exclusive", t.ID)
	}
	return nil
}

// validateWaitTask performs comprehensive validation for wait task configuration.
// Ensures required fields are present and expressions are valid.
func (t *Config) validateWaitTask(ctx context.Context) error {
	if t.WaitFor == "" {
		return fmt.Errorf("wait_for field is required")
	}
	if t.Condition == "" {
		return fmt.Errorf("condition field is required")
	}
	if err := t.validateWaitCondition(ctx); err != nil {
		return fmt.Errorf("invalid condition: %w", err)
	}
	if err := t.validateWaitTimeout(ctx); err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	if t.Processor != nil {
		if err := t.validateWaitProcessor(ctx); err != nil {
			return fmt.Errorf("invalid processor configuration: %w", err)
		}
	}
	return nil
}

// validateWaitCondition validates the CEL expression syntax
func (t *Config) validateWaitCondition(_ context.Context) error {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		return fmt.Errorf("failed to create CEL evaluator: %w", err)
	}
	return evaluator.ValidateExpression(t.Condition)
}

// validateWaitTimeout validates the timeout value if specified
func (t *Config) validateWaitTimeout(_ context.Context) error {
	if t.Timeout == "" {
		return nil // Timeout is optional
	}
	duration, err := core.ParseHumanDuration(t.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout format '%s': %w", t.Timeout, err)
	}
	if duration <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", duration)
	}
	return nil
}

// validateWaitProcessor validates the processor configuration
func (t *Config) validateWaitProcessor(ctx context.Context) error {
	if t.Processor.ID == "" {
		return fmt.Errorf("processor ID is required")
	}
	if t.Processor.Type == "" {
		return fmt.Errorf("processor type is required")
	}
	if err := t.Processor.Validate(ctx); err != nil {
		return fmt.Errorf("processor validation failed: %w", err)
	}
	return nil
}

// validateMemoryTask performs comprehensive validation for memory task configuration.
// Validates operation type, required fields, and operation-specific constraints.
func (t *Config) validateMemoryTask(ctx context.Context) error {
	if err := t.validateMemoryRequiredFields(ctx); err != nil {
		return err
	}
	if err := t.validateMemoryOperation(ctx); err != nil {
		return err
	}
	if err := t.validateMemoryLimits(ctx); err != nil {
		return err
	}
	return t.validateMemoryOperationSpecific(ctx)
}

func (t *Config) validateMemoryRequiredFields(_ context.Context) error {
	if t.Operation == "" {
		return fmt.Errorf("operation field is required")
	}
	if t.MemoryRef == "" {
		return fmt.Errorf("memory_ref field is required")
	}
	if t.KeyTemplate == "" {
		return fmt.Errorf("key_template field is required")
	}
	return nil
}

func (t *Config) validateMemoryOperation(_ context.Context) error {
	switch t.Operation {
	case MemoryOpRead, MemoryOpWrite, MemoryOpAppend, MemoryOpDelete,
		MemoryOpFlush, MemoryOpHealth, MemoryOpClear, MemoryOpStats:
		return nil
	default:
		return fmt.Errorf(
			"invalid operation '%s', must be one of: read, write, append, delete, flush, health, clear, stats",
			t.Operation,
		)
	}
}

func (t *Config) validateMemoryLimits(_ context.Context) error {
	if t.MaxKeys < 0 {
		return fmt.Errorf("max_keys cannot be negative")
	}
	if t.MaxKeys > 50000 {
		return fmt.Errorf("max_keys cannot exceed 50,000 for safety")
	}
	if t.BatchSize < 0 {
		return fmt.Errorf("batch_size cannot be negative")
	}
	if t.BatchSize > 10000 {
		return fmt.Errorf("batch_size cannot exceed 10,000 for safety")
	}
	return nil
}

func (t *Config) validateMemoryOperationSpecific(_ context.Context) error {
	switch t.Operation {
	case MemoryOpWrite, MemoryOpAppend:
		if t.Payload == nil {
			return fmt.Errorf("%s operation requires payload", t.Operation)
		}
	case MemoryOpFlush:
		if t.FlushConfig != nil {
			if t.FlushConfig.MaxKeys < 0 {
				return fmt.Errorf("flush max_keys cannot be negative")
			}
			if t.FlushConfig.Threshold < 0 || t.FlushConfig.Threshold > 1 {
				return fmt.Errorf("flush threshold must be between 0 and 1")
			}
		}
	case MemoryOpClear:
		if t.ClearConfig == nil || !t.ClearConfig.Confirm {
			return fmt.Errorf("clear operation requires confirm flag to be true")
		}
	}
	return nil
}

func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge task configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

func (t *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(t)
}

// FromMap updates the provider configuration from a normalized map
func (t *Config) FromMap(data any) error {
	var tmp Config
	decoder, err := newConfigDecoder(&tmp)
	if err != nil {
		return err
	}
	if err := decoder.Decode(data); err != nil {
		return err
	}
	return t.Merge(&tmp)
}

// newConfigDecoder builds the mapstructure decoder with the required hooks.
func newConfigDecoder(result *Config) (*mapstructure.Decoder, error) {
	hook := mapstructure.ComposeDecodeHookFunc(
		extendedConfigDecodeHook(),
		core.StringToMapAliasPtrHook,
	)
	return mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           result,
		TagName:          "mapstructure",
		DecodeHook:       hook,
	})
}

// extendedConfigDecodeHook prepares decode hooks for attachments and selectors.
func extendedConfigDecodeHook() mapstructure.DecodeHookFunc {
	attachmentHook := attachmentsDecodeHook()
	agentPtr := reflect.TypeOf(&agent.Config{})
	toolPtr := reflect.TypeOf(&tool.Config{})
	return func(from reflect.Type, to reflect.Type, v any) (any, error) {
		switch to {
		case agentPtr:
			if s, ok := v.(string); ok {
				return &agent.Config{ID: s}, nil
			}
		case toolPtr:
			if s, ok := v.(string); ok {
				return &tool.Config{ID: s}, nil
			}
		}
		if fn, ok := attachmentHook.(mapstructure.DecodeHookFuncType); ok {
			return fn(from, to, v)
		}
		return v, nil
	}
}

// attachmentsDecodeHook converts slices into attachment collections.
func attachmentsDecodeHook() mapstructure.DecodeHookFunc {
	attSliceType := reflect.TypeOf(attachment.Attachments{})
	return mapstructure.DecodeHookFuncType(func(_ reflect.Type, to reflect.Type, v any) (any, error) {
		if to != attSliceType {
			return v, nil
		}
		rv := reflect.ValueOf(v)
		if !rv.IsValid() || rv.Kind() != reflect.Slice {
			return v, nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var atts attachment.Attachments
		if err := json.Unmarshal(b, &atts); err != nil {
			return nil, err
		}
		return atts, nil
	})
}

func (t *Config) GetSleepDuration() (time.Duration, error) {
	if t.Sleep == "" {
		return 0, nil
	}
	return core.ParseHumanDuration(t.Sleep)
}

// GetStrategy returns the execution strategy for the task based on its type.
// For parallel and collection tasks, returns the configured strategy.
// For composite tasks, always returns WaitAll (sequential execution).
// For other task types, defaults to WaitAll.
func (t *Config) GetStrategy() ParallelStrategy {
	switch t.Type {
	case TaskTypeParallel:
		return t.ParallelTask.GetStrategy()
	case TaskTypeCollection:
		return t.ParallelTask.GetStrategy()
	case TaskTypeComposite:
		return StrategyWaitAll
	default:
		return StrategyWaitAll
	}
}

// GetExecType determines the execution type based on the task type.
// This is used internally by the task executor to select the appropriate
// execution logic for each task type.
func (t *Config) GetExecType() ExecutionType {
	taskType := t.Type
	if taskType == "" {
		taskType = TaskTypeBasic
	}
	var executionType ExecutionType
	switch taskType {
	case TaskTypeRouter:
		executionType = ExecutionRouter
	case TaskTypeParallel:
		executionType = ExecutionParallel
	case TaskTypeCollection:
		executionType = ExecutionCollection
	case TaskTypeComposite:
		executionType = ExecutionComposite
	case TaskTypeAggregate:
		executionType = ExecutionBasic
	case TaskTypeSignal:
		executionType = ExecutionBasic
	case TaskTypeWait:
		executionType = ExecutionWait
	case TaskTypeMemory:
		executionType = ExecutionBasic
	default:
		executionType = ExecutionBasic
	}
	return executionType
}

func (t *Config) Clone() (*Config, error) {
	if t == nil {
		return nil, nil
	}
	return core.DeepCopy(t)
}

// FindConfig searches for a task configuration by ID within a slice of tasks.
// Returns an error with available task IDs if the requested task is not found.
func FindConfig(tasks []Config, taskID string) (*Config, error) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return &tasks[i], nil
		}
	}
	availableIDs := make([]string, len(tasks))
	for i := range tasks {
		availableIDs[i] = tasks[i].ID
	}
	return nil, fmt.Errorf("task not found: searched for '%s', available tasks: %v", taskID, availableIDs)
}

// applyDefaults sets default values for task configurations.
// Called during task loading to ensure all required fields have sensible defaults.
func applyDefaults(config *Config) {
	if config.Type == TaskTypeCollection {
		config.Default()
	}
	hasSubTasks := config.Type == TaskTypeParallel ||
		config.Type == TaskTypeComposite ||
		config.Type == TaskTypeCollection
	if hasSubTasks && len(config.Tasks) > 0 {
		for i := range config.Tasks {
			applyDefaults(&config.Tasks[i])
		}
	}
	if config.Type == TaskTypeCollection && config.Task != nil {
		applyDefaults(config.Task)
	}
}

// setCWDForTask sets the CWD for a single task if needed
func setCWDForTask(task *Config, parentCWD *core.PathCWD, taskType string) error {
	if task.CWD != nil || parentCWD == nil {
		return nil
	}
	if err := task.SetCWD(parentCWD.PathStr()); err != nil {
		return fmt.Errorf("failed to set CWD for %s %s: %w", taskType, task.ID, err)
	}
	return nil
}

// PropagateTaskListCWD propagates the current working directory to a list of tasks.
// This ensures that all sub-tasks inherit the correct working directory for file operations.
func PropagateTaskListCWD(tasks []Config, parentCWD *core.PathCWD, taskType string) error {
	for i := range tasks {
		if err := setCWDForTask(&tasks[i], parentCWD, taskType); err != nil {
			return err
		}
		if err := propagateCWDToSubTasks(&tasks[i]); err != nil {
			return err
		}
	}
	return nil
}

// PropagateSingleTaskCWD propagates the current working directory to a single task.
// Used for collection task templates and other single task configurations.
func PropagateSingleTaskCWD(task *Config, parentCWD *core.PathCWD, taskType string) error {
	if err := setCWDForTask(task, parentCWD, taskType); err != nil {
		return err
	}
	return propagateCWDToSubTasks(task)
}

func propagateCWDToSubTasks(config *Config) error {
	switch config.Type {
	case TaskTypeParallel, TaskTypeComposite:
		if len(config.Tasks) > 0 {
			return PropagateTaskListCWD(config.Tasks, config.CWD, "sub-task")
		}
	case TaskTypeCollection:
		if config.Task != nil {
			if err := PropagateSingleTaskCWD(config.Task, config.CWD, "collection task template"); err != nil {
				return err
			}
		}
		if len(config.Tasks) > 0 {
			return PropagateTaskListCWD(config.Tasks, config.CWD, "collection task")
		}
	}
	return nil
}

// normalizeAttachmentsPhase1 attempts a best-effort Phase1 normalization for task-level attachments.
// It safely ignores missing-key template errors to defer resolution to runtime and recurses into subtasks.
func normalizeAttachmentsPhase1(
	ctx context.Context,
	cfg *Config,
	engine *tplengine.TemplateEngine,
	tplCtx map[string]any,
) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Attachments) > 0 {
		n := attachment.NewContextNormalizer(engine, cfg.CWD)
		res, err := n.Phase1(ctx, cfg.Attachments, tplCtx)
		if err == nil {
			cfg.Attachments = res
		} else if !errors.Is(err, tplengine.ErrMissingKey) {
			return fmt.Errorf("attachments normalization failed for task %s: %w", cfg.ID, err)
		}
	}
	switch cfg.Type {
	case TaskTypeParallel, TaskTypeComposite, TaskTypeRouter, TaskTypeAggregate:
		for i := range cfg.Tasks {
			if err := normalizeAttachmentsPhase1(ctx, &cfg.Tasks[i], engine, tplCtx); err != nil {
				return err
			}
		}
	case TaskTypeCollection:
		if cfg.Task != nil {
			if err := normalizeAttachmentsPhase1(ctx, cfg.Task, engine, tplCtx); err != nil {
				return err
			}
		}
		for i := range cfg.Tasks {
			if err := normalizeAttachmentsPhase1(ctx, &cfg.Tasks[i], engine, tplCtx); err != nil {
				return err
			}
		}
	}
	return nil
}

// Load reads and parses a task configuration from a YAML or JSON file.
// Applies defaults and propagates the working directory to all sub-tasks.
// This is the basic loading function used when no template evaluation is needed.
//
// Parameters:
//   - cwd: Current working directory for resolving relative paths
//   - path: Path to the task configuration file (can be relative or absolute)
//
// Returns:
//   - *Config: Parsed and validated task configuration
//   - error: Any loading or validation errors
func Load(ctx context.Context, cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](ctx, filePath)
	if err != nil {
		return nil, err
	}
	if string(config.Type) == "" {
		config.Type = TaskTypeBasic
	}
	applyDefaults(config)
	if err := propagateCWDToSubTasks(config); err != nil {
		return nil, err
	}
	if err := normalizeAttachmentsPhase1(ctx, config,
		tplengine.NewEngine(tplengine.FormatJSON), nil); err != nil {
		return nil, err
	}
	return config, nil
}
