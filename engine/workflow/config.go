package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/webhook"
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
	Resource string `json:"resource,omitempty"        yaml:"resource,omitempty"        mapstructure:"resource,omitempty"`
	// Unique identifier for the workflow (required)
	// Must be unique within the project scope. Used for referencing and execution.
	// - **Example**: "customer-support", "data-processing", "content-generation"
	ID string `json:"id"                        yaml:"id"                        mapstructure:"id"`
	// Version of the workflow for tracking changes
	// Follows semantic versioning (e.g., "1.0.0", "2.1.3")
	// Useful for managing workflow evolution and backwards compatibility
	Version string `json:"version,omitempty"         yaml:"version,omitempty"         mapstructure:"version,omitempty"`
	// Human-readable description of the workflow's purpose
	// Should clearly explain what the workflow does and when to use it
	Description string `json:"description,omitempty"     yaml:"description,omitempty"     mapstructure:"description,omitempty"`
	// JSON schemas for validating data structures used in the workflow
	// Define reusable schemas that can be referenced throughout the workflow
	// using $ref syntax (e.g., $ref: local::schemas.#(id="user_schema"))
	Schemas []schema.Schema `json:"schemas,omitempty"         yaml:"schemas,omitempty"         mapstructure:"schemas,omitempty"`
	// Configuration options including input schema and environment variables
	// Controls workflow behavior, validation, and runtime environment
	Opts Opts `json:"config"                    yaml:"config"                    mapstructure:"config"`
	// Author information for workflow attribution
	// Helps track ownership and responsibility for workflow maintenance
	Author *core.Author `json:"author,omitempty"          yaml:"author,omitempty"          mapstructure:"author,omitempty"`
	// External tools that can be invoked by agents or tasks
	// Define executable scripts or programs that perform specific operations
	// Tools provide deterministic, non-AI functionality like API calls or data processing
	// $ref: schema://tools
	Tools []tool.Config `json:"tools,omitempty"           yaml:"tools,omitempty"           mapstructure:"tools,omitempty"`
	// AI agents with specific instructions and capabilities
	// Configure LLM-powered agents with custom prompts, tools access, and behavior
	// Agents can be referenced by tasks using $use: agent(...) syntax
	// $ref: schema://agents
	Agents []agent.Config `json:"agents,omitempty"          yaml:"agents,omitempty"          mapstructure:"agents,omitempty"`
	// KnowledgeBases declares workflow-scoped knowledge definitions.
	KnowledgeBases []knowledge.BaseConfig `json:"knowledge_bases,omitempty" yaml:"knowledge_bases,omitempty" mapstructure:"knowledge_bases,omitempty"`
	// Knowledge defines the default knowledge binding for the workflow context.
	Knowledge []core.KnowledgeBinding `json:"knowledge,omitempty"       yaml:"knowledge,omitempty"       mapstructure:"knowledge,omitempty"`
	// Model Context Protocol servers for extending AI capabilities
	// MCP servers provide specialized tools and knowledge to agents
	// Enable integration with external services and domain-specific functionality
	// $ref: schema://mcp
	MCPs []mcp.Config `json:"mcps,omitempty"            yaml:"mcps,omitempty"            mapstructure:"mcps,omitempty"`
	// Event triggers that can initiate workflow execution
	// Define external events (webhooks, signals) that can start the workflow
	// Each trigger can have its own input schema for validation
	Triggers []Trigger `json:"triggers,omitempty"        yaml:"triggers,omitempty"        mapstructure:"triggers,omitempty"`
	// Sequential tasks that define the workflow execution plan (required)
	// Tasks are the core execution units, processed in order with conditional branching
	// Each task uses either an agent or tool to perform its operation
	// $ref: schema://tasks
	Tasks []task.Config `json:"tasks"                     yaml:"tasks"                     mapstructure:"tasks"`
	// Output mappings to structure the final workflow results
	// Use template expressions to extract and transform task outputs
	// - **Example**: ticket_id: "{{ .tasks.create-ticket.output.id }}"
	Outputs *core.Output `json:"outputs,omitempty"         yaml:"outputs,omitempty"         mapstructure:"outputs,omitempty"`
	// Schedule configuration for automated workflow execution
	// Enable cron-based scheduling with timezone support and overlap policies
	Schedule *Schedule `json:"schedule,omitempty"        yaml:"schedule,omitempty"        mapstructure:"schedule,omitempty"`

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

// KnowledgeBaseDefinitions exposes workflow-scoped knowledge bases for aggregation.
func (w *Config) KnowledgeBaseDefinitions() []knowledge.BaseConfig {
	if w == nil {
		return nil
	}
	return w.KnowledgeBases
}

// KnowledgeBaseProviderName identifies the workflow when contributing knowledge bases.
func (w *Config) KnowledgeBaseProviderName() string {
	if w == nil {
		return ""
	}
	if strings.TrimSpace(w.ID) == "" {
		return "workflow"
	}
	return fmt.Sprintf("workflow %q", strings.TrimSpace(w.ID))
}

func (w *Config) HasSchema() bool {
	return w.Opts.InputSchema != nil
}

// Validate validates the workflow configuration using the provided context.
func (w *Config) Validate(ctx context.Context) error {
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
	mergedInput, err := w.Opts.InputSchema.ApplyDefaults(inputMap)
	if err != nil {
		return nil, fmt.Errorf("failed to apply input defaults for workflow %s: %w", w.ID, err)
	}
	result := core.Input(mergedInput)
	return &result, nil
}

func (w *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
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

func setupCompileMetrics() (metric.Float64Histogram, metric.Int64Counter) {
	meter := otel.GetMeterProvider().Meter("compozy")
	var hist metric.Float64Histogram
	var cnt metric.Int64Counter
	if meter == nil {
		return nil, nil
	}
	if h, err := meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("compile", "duration_seconds"),
		metric.WithDescription("Duration of workflow compile step"),
	); err == nil {
		hist = h
	}
	if c, err := meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("compile", "total"),
		metric.WithDescription("Count of workflow compile attempts"),
	); err == nil {
		cnt = c
	}
	return hist, cnt
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

func (w *Config) Compile(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
) (compiled *Config, err error) {
	log := logger.FromContext(ctx)
	compileDur, compileCnt := setupCompileMetrics()
	started := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		if compileCnt != nil {
			compileCnt.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
		}
		if compileDur != nil {
			dur := time.Since(started).Seconds()
			compileDur.Record(ctx, dur, metric.WithAttributes(attribute.String("status", status)))
		}
	}()
	if store == nil {
		return nil, fmt.Errorf("compile failed: resource store is required")
	}
	if proj == nil || proj.Name == "" {
		return nil, fmt.Errorf("compile failed: project with valid name is required")
	}
	compiled, err = w.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone workflow '%s': %w", w.ID, err)
	}
	if err = linkWorkflowSchemas(ctx, proj, store, compiled); err == nil {
		for i := range compiled.Tasks {
			if err = compileTaskRecursive(ctx, proj, store, &compiled.Tasks[i]); err != nil {
				break
			}
		}
	}
	if err != nil {
		if te, ok := err.(*compileTaskError); ok {
			return nil, fmt.Errorf("compile failed for task '%s': %w", te.id, te.err)
		}
		return nil, fmt.Errorf("compile failed: %w", err)
	}
	log.Debug("Workflow compiled", "project", proj.Name, "workflow_id", compiled.ID)
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
	if err := linkTaskSchemas(ctx, proj, store, t); err != nil {
		return withCompileTaskErr(t.ID, err)
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
		switch route := rv.(type) {
		case map[string]any:
			var inline task.Config
			if err := inline.FromMap(route); err != nil {
				return fmt.Errorf("router route '%s' decode failed: %w", rk, err)
			}
			if err := compileTaskRecursive(ctx, proj, store, &inline); err != nil {
				return fmt.Errorf("router route '%s' compile failed: %w", rk, err)
			}
			t.Routes[rk] = inline
		case string:
		case task.Config:
			inline := route
			if err := compileTaskRecursive(ctx, proj, store, &inline); err != nil {
				return fmt.Errorf("router route '%s' compile failed: %w", rk, err)
			}
			t.Routes[rk] = inline
		case *task.Config:
			if route == nil {
				return fmt.Errorf("router route '%s' has nil task config", rk)
			}
			if err := compileTaskRecursive(ctx, proj, store, route); err != nil {
				return fmt.Errorf("router route '%s' compile failed: %w", rk, err)
			}
		default:
			return fmt.Errorf("router route '%s' has unsupported type %T", rk, rv)
		}
	}
	return nil
}

func compileSelectors(ctx context.Context, proj *project.Config, store resources.ResourceStore, t *task.Config) error {
	if t.Type == task.TaskTypeBasic {
		hasAgent := t.Agent != nil
		hasTool := t.Tool != nil
		hasDirect := strings.TrimSpace(t.Prompt) != ""
		// Note: provider/model defaults may be injected later; runtime validation ensures
		execCount := 0
		if hasAgent {
			execCount++
		}
		if hasTool {
			execCount++
		}
		if hasDirect {
			execCount++
		}
		if execCount == 0 {
			return fmt.Errorf(
				"task '%s' invalid selectors: exactly one executor required: agent, tool, or direct LLM",
				t.ID,
			)
		}
		if execCount > 1 {
			return fmt.Errorf(
				"task '%s' invalid selectors: cannot specify multiple executor types; use only one",
				t.ID,
			)
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
	Type       resources.ResourceType
	ID         string
	Project    string
	Candidates []string
}

func (e *SelectorNotFoundError) Error() string {
	if len(e.Candidates) == 0 {
		return fmt.Sprintf("%s '%s' not found in project '%s'", string(e.Type), e.ID, e.Project)
	}
	return fmt.Sprintf(
		"%s '%s' not found in project '%s'. Did you mean: %s?",
		string(e.Type),
		e.ID,
		e.Project,
		strings.Join(e.Candidates, ", "),
	)
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

func (e *compileTaskError) Error() string {
	return e.err.Error()
}

func (e *compileTaskError) Unwrap() error {
	return e.err
}

func withCompileTaskErr(id string, err error) error {
	if err == nil {
		return nil
	}
	return &compileTaskError{id: id, err: err}
}

func WorkflowsFromProject(ctx context.Context, projectConfig *project.Config) ([]*Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cwd := projectConfig.GetCWD()
	projectEnv := projectConfig.GetEnv()
	var ws []*Config
	for _, wf := range projectConfig.Workflows {
		config, err := Load(ctx, cwd, wf.Source)
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

func Load(ctx context.Context, cwd *core.PathCWD, path string) (*Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](ctx, filePath)
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
