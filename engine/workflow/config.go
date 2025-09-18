package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/webhook"
	pkgcfg "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// TriggerType defines the type of event that can initiate workflow execution.
//
// Triggers enable workflows to be initiated by external events rather than
// being manually executed or scheduled. This allows for event-driven architectures
// where workflows respond to system events, webhooks, or external signals.
type TriggerType string

const (
	// TriggerTypeSignal represents external signal-based triggers.
	// Signal triggers allow workflows to be initiated by external HTTP requests
	// or system events, enabling webhook-style integrations and event-driven workflows.
	TriggerTypeSignal TriggerType = "signal"
	// TriggerTypeWebhook represents HTTP webhook-based triggers.
	// Webhook triggers enable provider-agnostic ingress with per-event routing and validation.
	TriggerTypeWebhook TriggerType = "webhook"
)

// OverlapPolicy defines how scheduled workflows handle overlapping executions.
//
// When a workflow is scheduled to run at regular intervals, there may be cases
// where a previous execution is still running when the next scheduled time arrives.
// The overlap policy determines how Compozy handles these situations.
//
// **Example usage in schedule:**
//
//	schedule:
//	  cron: "*/5 * * * *"  # Every 5 minutes
//	  overlap_policy: buffer_one
type OverlapPolicy string

const (
	// OverlapSkip skips the new execution if the previous one is still running.
	// This is the **default policy** and ensures only one instance runs at a time.
	// Useful for workflows that should not have concurrent executions.
	OverlapSkip OverlapPolicy = "skip"
	// OverlapAllow allows multiple executions to run concurrently.
	// Use this when workflow executions are independent and can safely run in parallel.
	// Be cautious with resource consumption when using this policy.
	OverlapAllow OverlapPolicy = "allow"
	// OverlapBufferOne buffers one execution to run after the current one completes.
	// If multiple triggers occur while running, only the most recent is queued.
	// Useful for workflows that should process the latest trigger but not skip it entirely.
	OverlapBufferOne OverlapPolicy = "buffer_one"
	// OverlapCancelOther cancels the running execution and starts a new one.
	// Use this for workflows where the latest trigger invalidates previous executions.
	// The canceled execution will be marked as canceled in the execution history.
	OverlapCancelOther OverlapPolicy = "cancel_other"
)

// Trigger defines an external event that can initiate workflow execution.
//
// Triggers provide a way to start workflows based on external events such as
// webhooks, API calls, or system signals. Each trigger has a unique name and
// can optionally validate incoming data against a schema.
//
// **Example YAML configuration:**
//
//	triggers:
//	  - type: signal
//	    name: user-registration
//	    schema:
//	      type: object
//	      properties:
//	        userId:
//	          type: string
//	        email:
//	          type: string
//	          format: email
//	      required: [userId, email]
type Trigger struct {
	// Type of trigger mechanism (e.g., "signal" for external signals)
	Type TriggerType `json:"type"              yaml:"type"              mapstructure:"type"`
	// Unique name for identifying this trigger
	Name string `json:"name"              yaml:"name"              mapstructure:"name"`
	// Schema for validating trigger input data (optional)
	Schema *schema.Schema `json:"schema,omitempty"  yaml:"schema,omitempty"  mapstructure:"schema,omitempty"`
	// Webhook holds configuration when Type==webhook
	Webhook *webhook.Config `json:"webhook,omitempty" yaml:"webhook,omitempty" mapstructure:"webhook,omitempty"`
}

// Schedule defines when and how a workflow should be executed automatically.
//
// Schedules enable workflows to run at predetermined times using cron expressions.
// They support advanced features like timezone awareness, execution windows, jitter
// for load distribution, and policies for handling overlapping runs.
//
// **Example YAML configurations:**
//
// Basic hourly schedule:
//
//	schedule:
//	  cron: "0 * * * *"  # Every hour
//
// Advanced schedule with all options:
//
//	schedule:
//	  cron: "0 9 * * MON-FRI"  # 9 AM on weekdays
//	  timezone: "America/New_York"
//	  enabled: true
//	  jitter: "5m"  # Random delay up to 5 minutes
//	  overlap_policy: skip
//	  start_at: "2024-01-01T00:00:00Z"
//	  end_at: "2024-12-31T23:59:59Z"
//	  input:
//	    mode: "scheduled"
//	    priority: "normal"
type Schedule struct {
	// Cron expression for scheduling (required)
	// Supports standard cron format: "minute hour day month weekday"
	// Special strings: @yearly, @monthly, @weekly, @daily, @hourly
	Cron string `yaml:"cron"                     json:"cron"                     validate:"required,cron"`
	// Timezone for schedule execution (optional, default UTC)
	// Uses IANA timezone names (e.g., "America/New_York", "Europe/London")
	Timezone string `yaml:"timezone,omitempty"       json:"timezone,omitempty"`
	// Whether the schedule is enabled (optional, default true)
	// Set to false to temporarily disable scheduled runs without removing the configuration
	Enabled *bool `yaml:"enabled,omitempty"        json:"enabled,omitempty"`
	// Random delay to add to execution time (optional)
	// Format: "5m", "1h", "30s" - helps distribute load when many workflows run at the same time
	Jitter string `yaml:"jitter,omitempty"         json:"jitter,omitempty"`
	// Policy for handling overlapping executions (optional, default skip)
	// Options: skip, allow, buffer_one, cancel_other
	OverlapPolicy OverlapPolicy `yaml:"overlap_policy,omitempty" json:"overlap_policy,omitempty"`
	// Start date for the schedule (optional)
	// Schedule will not run before this time
	StartAt *time.Time `yaml:"start_at,omitempty"       json:"start_at,omitempty"`
	// End date for the schedule (optional)
	// Schedule will not run after this time
	EndAt *time.Time `yaml:"end_at,omitempty"         json:"end_at,omitempty"`
	// Default input values for scheduled runs (optional)
	// These inputs are merged with any trigger inputs when the workflow executes
	Input map[string]any `yaml:"input,omitempty"          json:"input,omitempty"`
}

// Opts defines workflow-specific configuration options.
//
// These options control the workflow's runtime behavior, input validation,
// and environment configuration. They extend the global options with
// workflow-specific settings.
//
// **Example YAML configuration:**
//
//	config:
//	  # Input schema definition
//	  input:
//	    type: object
//	    properties:
//	      message:
//	        type: string
//	        description: Message to process
//	      priority:
//	        type: string
//	        enum: [low, medium, high]
//	        default: medium
//	    required: [message]
//	  # Environment variables
//	  env:
//	    API_KEY: "{{ .secrets.API_KEY }}"
//	    DEBUG_MODE: "true"
//	    MAX_RETRIES: "3"
type Opts struct {
	// Global options inherited from core configuration
	// Includes provider settings, model configurations, and other global parameters
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`
	// Input schema for validating workflow input parameters
	// Uses JSON Schema format to define expected input structure and validation rules
	InputSchema *schema.Schema `json:"input,omitempty" yaml:"input,omitempty" mapstructure:"input,omitempty"`
	// Environment variables available to the workflow and its components
	// These variables are accessible to all tasks, agents, and tools within the workflow
	Env *core.EnvMap `json:"env,omitempty"   yaml:"env,omitempty"   mapstructure:"env,omitempty"`
}

// Config represents a workflow configuration in Compozy.
//
// Workflows in Compozy are **orchestration units** that define how AI agents, tools, and tasks
// work together to accomplish complex objectives. A workflow acts as a blueprint that:
//
//   - **Defines a sequence of tasks** to be executed with conditional branching
//   - **Coordinates AI agents** with specific instructions and capabilities
//   - **Integrates external tools** and Model Context Protocol (MCP) servers
//   - **Manages data flow** between tasks using Go template expressions
//   - **Handles scheduling** and triggering of automated executions
//   - **Validates inputs and outputs** according to defined JSON schemas
//
// ## Core Concepts
//
// **Tasks**: The building blocks of workflows. Each task performs a specific action
// using either an agent (AI-powered) or tool (deterministic function).
//
// **Agents**: AI models with specific instructions that can use tools and perform
// complex reasoning tasks.
//
// **Tools**: External programs or scripts that perform specific operations, like
// data processing, API calls, or file manipulation.
//
// **MCP Servers**: Model Context Protocol servers that extend AI capabilities with
// specialized tools and knowledge bases.
//
// **Data Flow**: Tasks can access outputs from previous tasks using template
// expressions like `{{ .tasks.previous-task.output.field }}`.
//
// ## Complete Example Workflow
//
//	id: customer-support-workflow
//	version: "1.0.0"
//	description: "Automated customer support with ticket creation"
//	author:
//	  name: "Support Team"
//	  email: "support@example.com"
//
//	# Input validation schema
//	config:
//	  input:
//	    type: object
//	    properties:
//	      customer_email:
//	        type: string
//	        format: email
//	      issue_description:
//	        type: string
//	        minLength: 10
//	    required: [customer_email, issue_description]
//	  env:
//	    SUPPORT_API_KEY: "{{ .secrets.SUPPORT_API_KEY }}"
//	    TICKET_SYSTEM_URL: "https://api.tickets.example.com"
//
//	# Reusable schemas
//	schemas:
//	  - id: ticket_schema
//	    type: object
//	    properties:
//	      ticket_id:
//	        type: string
//	      priority:
//	        type: string
//	        enum: [low, medium, high, urgent]
//
//	# External tools
//	tools:
//	  - id: ticket-creator
//	    description: "Creates support tickets via API"
//	    env:
//	      API_KEY: "{{ .workflow.env.SUPPORT_API_KEY }}"
//
//	# AI agents
//	agents:
//	  - id: support-agent
//	    model: "gpt-4"
//	    instructions: |
//	      You are a helpful customer support agent. Analyze customer issues,
//	      determine priority, and provide clear, empathetic responses.
//	    temperature: 0.7
//
//	# MCP servers for extended capabilities
//	mcps:
//	  - id: knowledge-base
//	    url: "http://localhost:3000/mcp"
//	    proto: "2025-03-26"
//
//	# Event triggers
//	triggers:
//	  - type: signal
//	    name: new-support-request
//	    schema:
//	      $ref: "local::config.input"
//
//	# Workflow tasks
//	tasks:
//	  - id: analyze-issue
//	    type: basic
//	    $use: agent(local::agents.#(id="support-agent"))
//	    with:
//	      prompt: |
//	        Analyze this customer issue and determine:
//	        1. Issue category
//	        2. Priority level
//	        3. Suggested resolution
//
//	        Customer: {{ .workflow.input.customer_email }}
//	        Issue: {{ .workflow.input.issue_description }}
//	    on_success:
//	      next: create-ticket
//	    on_error:
//	      next: escalate-to-human
//
//	  - id: create-ticket
//	    type: basic
//	    $use: tool(local::tools.#(id="ticket-creator"))
//	    with:
//	      email: "{{ .workflow.input.customer_email }}"
//	      description: "{{ .workflow.input.issue_description }}"
//	      priority: "{{ .tasks.analyze-issue.output.priority }}"
//	      category: "{{ .tasks.analyze-issue.output.category }}"
//	    final: true
//
//	  - id: escalate-to-human
//	    type: basic
//	    $use: tool(local::tools.#(id="ticket-creator"))
//	    with:
//	      email: "{{ .workflow.input.customer_email }}"
//	      description: "ESCALATED: {{ .workflow.input.issue_description }}"
//	      priority: "urgent"
//	      assign_to: "human-support-team"
//	    final: true
//
//	# Output mapping
//	outputs:
//	  ticket_id: "{{ .tasks.create-ticket.output.ticket_id }}"
//	  status: "{{ .tasks.create-ticket.output.status }}"
//	  resolution: "{{ .tasks.analyze-issue.output.suggested_resolution }}"
//
//	# Automated schedule
//	schedule:
//	  cron: "0 */4 * * *"  # Every 4 hours
//	  overlap_policy: buffer_one
//	  input:
//	    source: "scheduled_check"
type Config struct {
	// Resource reference for external workflow definitions
	// Format: "compozy:workflow:<name>" - allows referencing pre-built workflows
	Resource string `json:"resource,omitempty"    yaml:"resource,omitempty"    mapstructure:"resource,omitempty"`
	// Unique identifier for the workflow (required)
	// Must be unique within the project scope. Used for referencing and execution.
	// - **Example**: "customer-support", "data-processing", "content-generation"
	ID string `json:"id"                    yaml:"id"                    mapstructure:"id"`
	// Version of the workflow for tracking changes
	// Follows semantic versioning (e.g., "1.0.0", "2.1.3")
	// Useful for managing workflow evolution and backwards compatibility
	Version string `json:"version,omitempty"     yaml:"version,omitempty"     mapstructure:"version,omitempty"`
	// Human-readable description of the workflow's purpose
	// Should clearly explain what the workflow does and when to use it
	Description string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	// JSON schemas for validating data structures used in the workflow
	// Define reusable schemas that can be referenced throughout the workflow
	// using $ref syntax (e.g., $ref: local::schemas.#(id="user_schema"))
	Schemas []schema.Schema `json:"schemas,omitempty"     yaml:"schemas,omitempty"     mapstructure:"schemas,omitempty"`
	// Configuration options including input schema and environment variables
	// Controls workflow behavior, validation, and runtime environment
	Opts Opts `json:"config"                yaml:"config"                mapstructure:"config"`
	// Author information for workflow attribution
	// Helps track ownership and responsibility for workflow maintenance
	Author *core.Author `json:"author,omitempty"      yaml:"author,omitempty"      mapstructure:"author,omitempty"`
	// External tools that can be invoked by agents or tasks
	// Define executable scripts or programs that perform specific operations
	// Tools provide deterministic, non-AI functionality like API calls or data processing
	// $ref: schema://tools
	Tools []tool.Config `json:"tools,omitempty"       yaml:"tools,omitempty"       mapstructure:"tools,omitempty"`
	// AI agents with specific instructions and capabilities
	// Configure LLM-powered agents with custom prompts, tools access, and behavior
	// Agents can be referenced by tasks using $use: agent(...) syntax
	// $ref: schema://agents
	Agents []agent.Config `json:"agents,omitempty"      yaml:"agents,omitempty"      mapstructure:"agents,omitempty"`
	// Model Context Protocol servers for extending AI capabilities
	// MCP servers provide specialized tools and knowledge to agents
	// Enable integration with external services and domain-specific functionality
	// $ref: schema://mcp
	MCPs []mcp.Config `json:"mcps,omitempty"        yaml:"mcps,omitempty"        mapstructure:"mcps,omitempty"`
	// Event triggers that can initiate workflow execution
	// Define external events (webhooks, signals) that can start the workflow
	// Each trigger can have its own input schema for validation
	Triggers []Trigger `json:"triggers,omitempty"    yaml:"triggers,omitempty"    mapstructure:"triggers,omitempty"`
	// Sequential tasks that define the workflow execution plan (required)
	// Tasks are the core execution units, processed in order with conditional branching
	// Each task uses either an agent or tool to perform its operation
	// $ref: schema://tasks
	Tasks []task.Config `json:"tasks"                 yaml:"tasks"                 mapstructure:"tasks"`
	// Output mappings to structure the final workflow results
	// Use template expressions to extract and transform task outputs
	// - **Example**: ticket_id: "{{ .tasks.create-ticket.output.id }}"
	Outputs *core.Output `json:"outputs,omitempty"     yaml:"outputs,omitempty"     mapstructure:"outputs,omitempty"`
	// Schedule configuration for automated workflow execution
	// Enable cron-based scheduling with timezone support and overlap policies
	Schedule *Schedule `json:"schedule,omitempty"    yaml:"schedule,omitempty"    mapstructure:"schedule,omitempty"`

	// Internal field for tracking the source file path
	filePath string
	// Internal field for the current working directory context
	CWD *core.PathCWD
}

func (w *Config) Component() core.ConfigType {
	return core.ConfigWorkflow
}

func (w *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	w.CWD = CWD
	if err := setComponentsCWD(w, w.CWD); err != nil {
		return err
	}
	return nil
}

func (w *Config) GetCWD() *core.PathCWD {
	return w.CWD
}

func (w *Config) GetEnv() core.EnvMap {
	if w.Opts.Env == nil {
		w.Opts.Env = &core.EnvMap{}
		return *w.Opts.Env
	}
	return *w.Opts.Env
}

func (w *Config) GetInput() *core.Input {
	return &core.Input{}
}

func (w *Config) GetOutputs() *core.Output {
	return w.Outputs
}

func (w *Config) GetFilePath() string {
	return w.filePath
}

func (w *Config) SetFilePath(path string) {
	w.filePath = path
}

func (w *Config) HasSchema() bool {
	return w.Opts.InputSchema != nil
}

func (w *Config) Validate() error {
	// Backward-compatible entry point without context
	return w.ValidateWithContext(context.Background())
}

// ValidateWithContext validates the workflow configuration using the provided context.
// Prefer this over Validate() to preserve cancellations and deadlines.
func (w *Config) ValidateWithContext(ctx context.Context) error {
	validator := NewWorkflowValidator(w)
	return validator.Validate(ctx)
}

func (w *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	validator := NewInputValidator(w, input)
	return validator.Validate(ctx)
}

// ApplyInputDefaults merges default values from the input schema with the provided input
func (w *Config) ApplyInputDefaults(input *core.Input) (*core.Input, error) {
	if w.Opts.InputSchema == nil {
		// No schema, return input as-is
		if input == nil {
			input = &core.Input{}
		}
		return input, nil
	}
	var inputMap map[string]any
	if input == nil {
		inputMap = make(map[string]any)
	} else {
		inputMap = *input
	}
	// Apply defaults from schema
	mergedInput, err := w.Opts.InputSchema.ApplyDefaults(inputMap)
	if err != nil {
		return nil, fmt.Errorf("failed to apply input defaults for workflow %s: %w", w.ID, err)
	}
	result := core.Input(mergedInput)
	return &result, nil
}

func (w *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the workflow having a schema
	return nil
}

func (w *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge workflow configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(w, otherConfig, mergo.WithOverride)
}

func (w *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(w)
}

func (w *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return w.Merge(config)
}

func (w *Config) GetID() string {
	return w.ID
}

func (w *Config) SetDefaults() {
	if w.Schedule != nil {
		if w.Schedule.Enabled == nil {
			enabled := true
			w.Schedule.Enabled = &enabled
		}
		if w.Schedule.OverlapPolicy == "" {
			w.Schedule.OverlapPolicy = OverlapSkip
		}
	}
	for i := range w.Triggers {
		t := &w.Triggers[i]
		if t.Type == TriggerTypeWebhook && t.Webhook != nil {
			webhook.ApplyDefaults(t.Webhook)
		}
	}
	for i := range w.MCPs {
		w.MCPs[i].SetDefaults()
	}
}

// GetTasks returns the workflow tasks
func (w *Config) GetTasks() []task.Config {
	return w.Tasks
}

// GetMCPs returns the workflow MCPs
func (w *Config) GetMCPs() []mcp.Config {
	mcps := make([]mcp.Config, len(w.MCPs))
	copy(mcps, w.MCPs)
	return mcps
}

func (w *Config) DetermineNextTask(
	taskConfig *task.Config,
	success bool,
) *task.Config {
	var nextTaskID string
	if success && taskConfig.OnSuccess != nil && taskConfig.OnSuccess.Next != nil {
		nextTaskID = *taskConfig.OnSuccess.Next
	} else if !success && taskConfig.OnError != nil && taskConfig.OnError.Next != nil {
		nextTaskID = *taskConfig.OnError.Next
	}
	if nextTaskID == "" {
		return nil
	}
	// Find the next task config
	nextTask, err := task.FindConfig(w.Tasks, nextTaskID)
	if err != nil {
		return nil
	}
	return nextTask
}

func (w *Config) Clone() (*Config, error) {
	if w == nil {
		return nil, nil
	}
	return core.DeepCopy(w)
}

// Compile materializes a workflow configuration by resolving ID-based selectors
// (agent/tool) and applying precedence rules. It deep-copies resolved configs so
// no mutable state is shared across tasks at runtime.
//
// Rules (MVP per PRD):
// - For basic tasks, exactly one of agent/tool must be set
// - If Agent/Tool contains only an ID (selector), resolve from ResourceStore
// - If agent model is empty, fill from project default model (when available)
// - Deep-copy all resolved configs
//
// NOTE: Schema ID linking and MCP selector support will be completed in later tasks
// (see tasks/prd-refs/_task_6.0.md and _task_7.0.md).
func (w *Config) Compile(ctx context.Context, proj *project.Config, store resources.ResourceStore) (*Config, error) {
	_ = pkgcfg.FromContext(ctx)
	log := logger.FromContext(ctx)
	meter := otel.GetMeterProvider().Meter("compozy")
	compileDur, derr := meter.Float64Histogram(
		"compozy_compile_duration_seconds",
		metric.WithDescription("Duration of workflow compile step"),
	)
	compileCnt, cerr := meter.Int64Counter(
		"compozy_compile_total",
		metric.WithDescription("Count of workflow compile attempts"),
	)
	started := time.Now()
	if store == nil {
		return nil, fmt.Errorf("compile failed: resource store is required")
	}
	if proj == nil || proj.Name == "" {
		return nil, fmt.Errorf("compile failed: project with valid name is required")
	}
	compiled, err := w.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone workflow '%s': %w", w.ID, err)
	}
	for i := range compiled.Tasks {
		if err = compileTaskRecursive(ctx, proj, store, &compiled.Tasks[i]); err != nil {
			break
		}
	}
	dur := time.Since(started).Seconds()
	if err != nil {
		if cerr == nil {
			compileCnt.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		}
		if derr == nil {
			compileDur.Record(ctx, dur, metric.WithAttributes(attribute.String("status", "error")))
		}
		if te, ok := err.(*compileTaskError); ok {
			return nil, fmt.Errorf("compile failed for task '%s': %w", te.id, te.err)
		}
		return nil, fmt.Errorf("compile failed: %w", err)
	}
	if cerr == nil {
		compileCnt.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
	}
	if derr == nil {
		compileDur.Record(ctx, dur, metric.WithAttributes(attribute.String("status", "ok")))
	}
	log.Debug("Workflow compiled", "workflow_id", compiled.ID)
	return compiled, nil
}

func compileTaskRecursive(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	t *task.Config,
) error {
	if t == nil {
		return nil
	}
	if err := compileChildren(ctx, proj, store, t); err != nil {
		return withCompileTaskErr(t.ID, err)
	}
	if err := compileRouterInline(ctx, proj, store, t); err != nil {
		return withCompileTaskErr(t.ID, err)
	}
	if err := compileSelectors(ctx, proj, store, t); err != nil {
		return withCompileTaskErr(t.ID, err)
	}
	return nil
}

func compileChildren(ctx context.Context, proj *project.Config, store resources.ResourceStore, t *task.Config) error {
	if t.Type == task.TaskTypeParallel {
		for i := range t.Tasks {
			if err := compileTaskRecursive(ctx, proj, store, &t.Tasks[i]); err != nil {
				return err
			}
		}
	}
	if t.Task != nil {
		if err := compileTaskRecursive(ctx, proj, store, t.Task); err != nil {
			return err
		}
	}
	return nil
}

func compileRouterInline(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	t *task.Config,
) error {
	if t.Type != task.TaskTypeRouter || t.Routes == nil {
		return nil
	}
	for rk, rv := range t.Routes {
		if m, ok := rv.(map[string]any); ok {
			var inline task.Config
			if err := inline.FromMap(m); err != nil {
				return fmt.Errorf("router route '%s' decode failed: %w", rk, err)
			}
			if err := compileTaskRecursive(ctx, proj, store, &inline); err != nil {
				return fmt.Errorf("router route '%s' compile failed: %w", rk, err)
			}
			t.Routes[rk] = inline
		}
	}
	return nil
}

func compileSelectors(ctx context.Context, proj *project.Config, store resources.ResourceStore, t *task.Config) error {
	// Enforce PRD selector grammar at compile time for basic tasks
	if t.Type == task.TaskTypeBasic {
		hasAgent := t.Agent != nil
		hasTool := t.Tool != nil
		if hasAgent == hasTool { // both true or both false
			return fmt.Errorf("task '%s' invalid selectors: exactly one of agent or tool is required", t.ID)
		}
	}
	if t.Agent != nil {
		resolved, err := resolveAgent(ctx, proj, store, t.Agent)
		if err != nil {
			return err
		}
		t.Agent = resolved
	}
	if t.Tool != nil {
		resolved, err := resolveTool(ctx, proj, store, t.Tool)
		if err != nil {
			return err
		}
		t.Tool = resolved
	}
	return nil
}

// validateBasicSelectors removed: validation is consolidated in task.Config.Validate
// and compileSelectors enforces PRD grammar for presence.

// SelectorNotFoundError signals a missing resource by type+ID
type SelectorNotFoundError struct {
	Type resources.ResourceType
	ID   string
}

func (e *SelectorNotFoundError) Error() string {
	return fmt.Sprintf("%s '%s' not found", string(e.Type), e.ID)
}

// TypeMismatchError signals a resource type conflict in the store
type TypeMismatchError struct {
	Type resources.ResourceType
	ID   string
	Got  any
}

func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf("%s '%s' has incompatible type %T", string(e.Type), e.ID, e.Got)
}

// compileTaskError wraps an inner error with the task ID where it occurred.
type compileTaskError struct {
	id  string
	err error
}

func (e *compileTaskError) Error() string { return e.err.Error() }
func (e *compileTaskError) Unwrap() error { return e.err }

func withCompileTaskErr(id string, err error) error {
	if err == nil {
		return nil
	}
	return &compileTaskError{id: id, err: err}
}

func resolveAgent(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *agent.Config,
) (*agent.Config, error) {
	log := logger.FromContext(ctx)
	// Treat as selector when only ID is provided (no provider/model/instructions)
	if isAgentSelector(in) {
		key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceAgent, ID: in.ID}
		val, _, err := store.Get(ctx, key)
		if err != nil {
			if errors.Is(err, resources.ErrNotFound) {
				return nil, &SelectorNotFoundError{Type: resources.ResourceAgent, ID: in.ID}
			}
			return nil, fmt.Errorf("agent lookup failed for '%s': %w", in.ID, err)
		}
		got, ok := val.(*agent.Config)
		if !ok {
			return nil, &TypeMismatchError{Type: resources.ResourceAgent, ID: in.ID, Got: val}
		}
		clone, err := got.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone agent '%s': %w", in.ID, err)
		}
		// Apply agent-level model selector if provided on the incoming selector
		if in.Model != "" {
			if err := applyAgentModelSelector(ctx, proj, store, clone, in.Model); err != nil {
				return nil, err
			}
		}
		fillAgentModelDefault(clone, proj)
		log.Debug("Resolved agent selector", "agent_id", in.ID)
		return clone, nil
	}
	// Inline agent; deep-copy and apply default model if missing
	clone, err := in.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone inline agent '%s': %w", in.ID, err)
	}
	// If inline agent specifies a model selector, resolve it into the provider config
	if in.Model != "" {
		if err := applyAgentModelSelector(ctx, proj, store, clone, in.Model); err != nil {
			return nil, err
		}
	}
	fillAgentModelDefault(clone, proj)
	return clone, nil
}

func resolveTool(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *tool.Config,
) (*tool.Config, error) {
	log := logger.FromContext(ctx)
	if isToolSelector(in) {
		key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceTool, ID: in.ID}
		val, _, err := store.Get(ctx, key)
		if err != nil {
			if errors.Is(err, resources.ErrNotFound) {
				return nil, &SelectorNotFoundError{Type: resources.ResourceTool, ID: in.ID}
			}
			return nil, fmt.Errorf("tool lookup failed for '%s': %w", in.ID, err)
		}
		got, ok := val.(*tool.Config)
		if !ok {
			return nil, &TypeMismatchError{Type: resources.ResourceTool, ID: in.ID, Got: val}
		}
		clone, err := got.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone tool '%s': %w", in.ID, err)
		}
		log.Debug("Resolved tool selector", "tool_id", in.ID)
		return clone, nil
	}
	clone, err := in.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone inline tool '%s': %w", in.ID, err)
	}
	return clone, nil
}

func isAgentSelector(a *agent.Config) bool {
	if a == nil {
		return false
	}
	hasID := a.ID != ""
	noProvider := a.Config.Provider == ""
	noModel := a.Config.Model == ""
	noInstr := a.Instructions == ""
	return hasID && noProvider && noModel && noInstr && len(a.Tools) == 0 && len(a.MCPs) == 0
}

func isToolSelector(t *tool.Config) bool {
	if t == nil {
		return false
	}
	return t.ID != "" && t.Description == "" && t.Timeout == "" && t.InputSchema == nil && t.OutputSchema == nil
}

func fillAgentModelDefault(a *agent.Config, proj *project.Config) {
	if a == nil || proj == nil {
		return
	}
	if a.Config.Provider == "" || a.Config.Model == "" {
		if def := proj.GetDefaultModel(); def != nil {
			if a.Config.Provider == "" {
				a.Config.Provider = def.Provider
			}
			if a.Config.Model == "" {
				a.Config.Model = def.Model
			}
			if a.Config.APIKey == "" {
				a.Config.APIKey = def.APIKey
			}
			if a.Config.APIURL == "" {
				a.Config.APIURL = def.APIURL
			}
		}
	}
}

// applyAgentModelSelector resolves a model resource by ID and merges it into the
// agent's ProviderConfig, preserving any explicitly set fields on the agent.
func applyAgentModelSelector(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	a *agent.Config,
	modelID string,
) error {
	key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceModel, ID: modelID}
	val, _, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return &SelectorNotFoundError{Type: resources.ResourceModel, ID: modelID}
		}
		return fmt.Errorf("model lookup failed for '%s': %w", modelID, err)
	}
	// Models are stored as *core.ProviderConfig
	pc, ok := val.(*core.ProviderConfig)
	if !ok {
		return &TypeMismatchError{Type: resources.ResourceModel, ID: modelID, Got: val}
	}
	// Merge resolved model defaults into agent config (agent fields win)
	mergeProviderDefaults(&a.Config, pc)
	return nil
}

// mergeProviderDefaults copies non-set fields from src into dst.
// Explicit values already present in dst take precedence.
func mergeProviderDefaults(dst *core.ProviderConfig, src *core.ProviderConfig) {
	if dst == nil || src == nil {
		return
	}
	mergeProviderIdentity(dst, src)
	mergeProviderParams(&dst.Params, &src.Params)
	if dst.Organization == "" {
		dst.Organization = src.Organization
	}
	if dst.MaxToolIterations == 0 {
		dst.MaxToolIterations = src.MaxToolIterations
	}
}

func mergeProviderIdentity(dst *core.ProviderConfig, src *core.ProviderConfig) {
	if dst.Provider == "" {
		dst.Provider = src.Provider
	}
	if dst.Model == "" {
		dst.Model = src.Model
	}
	if dst.APIKey == "" {
		dst.APIKey = src.APIKey
	}
	if dst.APIURL == "" {
		dst.APIURL = src.APIURL
	}
}

func mergeProviderParams(dst *core.PromptParams, src *core.PromptParams) {
	if dst == nil || src == nil {
		return
	}
	type copier struct {
		dstSet bool
		srcSet bool
		do     func()
	}
	ops := []copier{
		{dst.IsSetMaxTokens(), src.IsSetMaxTokens(), func() { dst.MaxTokens = src.MaxTokens }},
		{dst.IsSetTemperature(), src.IsSetTemperature(), func() { dst.Temperature = src.Temperature }},
		{dst.IsSetStopWords(), src.IsSetStopWords(), func() {
			if len(src.StopWords) > 0 {
				dst.StopWords = append([]string(nil), src.StopWords...)
			}
		}},
		{dst.IsSetTopK(), src.IsSetTopK(), func() { dst.TopK = src.TopK }},
		{dst.IsSetTopP(), src.IsSetTopP(), func() { dst.TopP = src.TopP }},
		{dst.IsSetSeed(), src.IsSetSeed(), func() { dst.Seed = src.Seed }},
		{dst.IsSetMinLength(), src.IsSetMinLength(), func() { dst.MinLength = src.MinLength }},
		{
			dst.IsSetRepetitionPenalty(),
			src.IsSetRepetitionPenalty(),
			func() { dst.RepetitionPenalty = src.RepetitionPenalty },
		},
	}
	for i := range ops {
		c := ops[i]
		if !c.dstSet && c.srcSet {
			c.do()
		}
	}
}

func WorkflowsFromProject(projectConfig *project.Config) ([]*Config, error) {
	cwd := projectConfig.GetCWD()
	projectEnv := projectConfig.GetEnv()
	var ws []*Config
	for _, wf := range projectConfig.Workflows {
		config, err := Load(cwd, wf.Source)
		if err != nil {
			return nil, err
		}
		if config != nil {
			config.Opts.Env = &projectEnv
		}
		ws = append(ws, config)
	}
	return ws, nil
}

func setComponentsCWD(wc *Config, cwd *core.PathCWD) error {
	if err := setTasksCWD(wc, cwd); err != nil {
		return err
	}
	if err := setToolsCWD(wc, cwd); err != nil {
		return err
	}
	if err := setAgentsCWD(wc, cwd); err != nil {
		return err
	}
	return nil
}

func setTasksCWD(wc *Config, cwd *core.PathCWD) error {
	for i := range wc.Tasks {
		if err := wc.Tasks[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
	}
	return nil
}

func setToolsCWD(wc *Config, cwd *core.PathCWD) error {
	for i := range wc.Tools {
		if err := wc.Tools[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
	}
	return nil
}

func setAgentsCWD(wc *Config, cwd *core.PathCWD) error {
	for i := range wc.Agents {
		if err := wc.Agents[i].SetCWD(cwd.PathStr()); err != nil {
			return err
		}
	}
	return nil
}

func Load(cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	config.SetDefaults()
	return config, nil
}

func FindConfig(workflows []*Config, workflowID string) (*Config, error) {
	for _, wf := range workflows {
		if wf.ID == workflowID {
			return wf, nil
		}
	}
	return nil, fmt.Errorf("workflow not found: %s", workflowID)
}

func FindAgentConfig[C core.Config](workflows []*Config, agentID string) (C, error) {
	var cfg C
	for _, wf := range workflows {
		for i := range wf.Agents {
			if wf.Agents[i].ID == agentID {
				cfg, ok := any(&wf.Agents[i]).(C)
				if !ok {
					return cfg, fmt.Errorf("agent config is not of type %T", cfg)
				}
				return cfg, nil
			}
		}
	}
	return cfg, fmt.Errorf("agent not found: %s", agentID)
}

// SlugsFromList returns all webhook slugs from a list of workflows.
// Empty slugs are ignored.
func SlugsFromList(workflows []*Config) []string {
	slugs := make([]string, 0, len(workflows))
	for _, wf := range workflows {
		for i := range wf.Triggers {
			t := &wf.Triggers[i]
			if t.Type == TriggerTypeWebhook && t.Webhook != nil && t.Webhook.Slug != "" {
				slugs = append(slugs, t.Webhook.Slug)
			}
		}
	}
	return slugs
}
