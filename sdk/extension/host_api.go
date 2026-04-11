package extension

import (
	"context"
	"encoding/json"
	"fmt"
)

// HostAPI exposes the public host callback surface available to extension
// handlers after initialization succeeds.
type HostAPI struct {
	Events    *EventsClient
	Tasks     *TasksClient
	Runs      *RunsClient
	Artifacts *ArtifactsClient
	Prompts   *PromptsClient
	Memory    *MemoryClient
}

type hostCaller interface {
	call(ctx context.Context, method string, params any, result any) error
}

// EventsClient wraps the host.events namespace.
type EventsClient struct {
	caller hostCaller
}

// TasksClient wraps the host.tasks namespace.
type TasksClient struct {
	caller hostCaller
}

// RunsClient wraps the host.runs namespace.
type RunsClient struct {
	caller hostCaller
}

// ArtifactsClient wraps the host.artifacts namespace.
type ArtifactsClient struct {
	caller hostCaller
}

// PromptsClient wraps the host.prompts namespace.
type PromptsClient struct {
	caller hostCaller
}

// MemoryClient wraps the host.memory namespace.
type MemoryClient struct {
	caller hostCaller
}

// EventSubscribeRequest narrows the forwarded event kinds for this extension.
type EventSubscribeRequest struct {
	Kinds []EventKind `json:"kinds"`
}

// EventSubscribeResult acknowledges the active event subscription filter.
type EventSubscribeResult struct {
	SubscriptionID string `json:"subscription_id"`
}

// EventPublishRequest publishes a custom extension event through the host.
type EventPublishRequest struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// EventPublishResult reports the journal sequence assigned to a published
// extension event.
type EventPublishResult struct {
	Seq uint64 `json:"seq,omitempty"`
}

// TaskFrontmatter describes the task metadata written by host.tasks.create.
type TaskFrontmatter struct {
	Status       string   `json:"status"`
	Type         string   `json:"type"`
	Complexity   string   `json:"complexity,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// Task identifies one task document returned by the Host API.
type Task struct {
	Workflow     string   `json:"workflow"`
	Number       int      `json:"number"`
	Path         string   `json:"path"`
	Status       string   `json:"status"`
	Title        string   `json:"title,omitempty"`
	Type         string   `json:"type,omitempty"`
	Complexity   string   `json:"complexity,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Body         string   `json:"body,omitempty"`
}

// TaskListRequest lists tasks inside one workflow directory.
type TaskListRequest struct {
	Workflow string `json:"workflow"`
}

// TaskGetRequest fetches one task by workflow and task number.
type TaskGetRequest struct {
	Workflow string `json:"workflow"`
	Number   int    `json:"number"`
}

// TaskCreateRequest creates a new task through the host task service.
type TaskCreateRequest struct {
	Workflow    string          `json:"workflow"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	Frontmatter TaskFrontmatter `json:"frontmatter"`
}

// RunStartRequest launches a new child run through the host.
type RunStartRequest struct {
	Runtime RunConfig `json:"runtime"`
}

// RunConfig is the Host API run-start payload.
type RunConfig struct {
	WorkspaceRoot          string        `json:"workspace_root,omitempty"`
	Name                   string        `json:"name,omitempty"`
	Round                  int           `json:"round,omitempty"`
	Provider               string        `json:"provider,omitempty"`
	PR                     string        `json:"pr,omitempty"`
	ReviewsDir             string        `json:"reviews_dir,omitempty"`
	TasksDir               string        `json:"tasks_dir,omitempty"`
	AutoCommit             bool          `json:"auto_commit,omitempty"`
	Concurrent             int           `json:"concurrent,omitempty"`
	BatchSize              int           `json:"batch_size,omitempty"`
	IDE                    string        `json:"ide,omitempty"`
	Model                  string        `json:"model,omitempty"`
	AddDirs                []string      `json:"add_dirs,omitempty"`
	TailLines              int           `json:"tail_lines,omitempty"`
	ReasoningEffort        string        `json:"reasoning_effort,omitempty"`
	AccessMode             string        `json:"access_mode,omitempty"`
	Mode                   ExecutionMode `json:"mode,omitempty"`
	OutputFormat           OutputFormat  `json:"output_format,omitempty"`
	Verbose                bool          `json:"verbose,omitempty"`
	TUI                    bool          `json:"tui,omitempty"`
	Persist                bool          `json:"persist,omitempty"`
	RunID                  string        `json:"run_id,omitempty"`
	PromptText             string        `json:"prompt_text,omitempty"`
	PromptFile             string        `json:"prompt_file,omitempty"`
	ReadPromptStdin        bool          `json:"read_prompt_stdin,omitempty"`
	IncludeCompleted       bool          `json:"include_completed,omitempty"`
	IncludeResolved        bool          `json:"include_resolved,omitempty"`
	TimeoutMS              int64         `json:"timeout_ms,omitempty"`
	MaxRetries             int           `json:"max_retries,omitempty"`
	RetryBackoffMultiplier float64       `json:"retry_backoff_multiplier,omitempty"`
}

// RunHandle identifies a host-started run.
type RunHandle struct {
	RunID       string `json:"run_id"`
	ParentRunID string `json:"parent_run_id,omitempty"`
}

// ArtifactReadRequest reads one artifact path through the host.
type ArtifactReadRequest struct {
	Path string `json:"path"`
}

// ArtifactReadResult returns artifact content scoped by the host.
type ArtifactReadResult struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

// ArtifactWriteRequest writes one artifact path through the host.
type ArtifactWriteRequest struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

// ArtifactWriteResult acknowledges one artifact write.
type ArtifactWriteResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

// PromptRenderRequest renders one host-managed prompt template.
type PromptRenderRequest struct {
	Template string             `json:"template"`
	Params   PromptRenderParams `json:"params"`
}

// PromptRenderParams describes the prompt render input snapshot.
type PromptRenderParams struct {
	Name        string                      `json:"name,omitempty"`
	Round       int                         `json:"round,omitempty"`
	Provider    string                      `json:"provider,omitempty"`
	PR          string                      `json:"pr,omitempty"`
	ReviewsDir  string                      `json:"reviews_dir,omitempty"`
	BatchGroups map[string][]PromptIssueRef `json:"batch_groups,omitempty"`
	AutoCommit  bool                        `json:"auto_commit,omitempty"`
	Mode        ExecutionMode               `json:"mode,omitempty"`
	Memory      *WorkflowMemoryContext      `json:"memory,omitempty"`
}

// PromptIssueRef mirrors the prompt render issue input shape.
type PromptIssueRef struct {
	Name     string `json:"name"`
	AbsPath  string `json:"abs_path,omitempty"`
	Content  string `json:"content,omitempty"`
	CodeFile string `json:"code_file,omitempty"`
}

// PromptRenderResult returns one rendered prompt body.
type PromptRenderResult struct {
	Rendered string `json:"rendered"`
}

// MemoryReadRequest reads one workflow or task memory document.
type MemoryReadRequest struct {
	Workflow string `json:"workflow"`
	TaskFile string `json:"task_file,omitempty"`
}

// MemoryReadResult returns a workflow or task memory document.
type MemoryReadResult struct {
	Path            string `json:"path"`
	Content         string `json:"content"`
	Exists          bool   `json:"exists"`
	NeedsCompaction bool   `json:"needs_compaction"`
}

// MemoryWriteRequest writes one workflow or task memory document.
type MemoryWriteRequest struct {
	Workflow string          `json:"workflow"`
	TaskFile string          `json:"task_file,omitempty"`
	Content  string          `json:"content"`
	Mode     MemoryWriteMode `json:"mode,omitempty"`
}

// MemoryWriteResult acknowledges one workflow or task memory write.
type MemoryWriteResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

func newHostAPI(caller hostCaller) *HostAPI {
	return &HostAPI{
		Events:    &EventsClient{caller: caller},
		Tasks:     &TasksClient{caller: caller},
		Runs:      &RunsClient{caller: caller},
		Artifacts: &ArtifactsClient{caller: caller},
		Prompts:   &PromptsClient{caller: caller},
		Memory:    &MemoryClient{caller: caller},
	}
}

// Subscribe registers the current event kind filter.
func (c *EventsClient) Subscribe(
	ctx context.Context,
	req EventSubscribeRequest,
) (*EventSubscribeResult, error) {
	return callHostMethod[EventSubscribeResult](ctx, c.caller, "host.events.subscribe", req)
}

// Publish forwards one extension event through the host event bus.
func (c *EventsClient) Publish(
	ctx context.Context,
	req EventPublishRequest,
) (*EventPublishResult, error) {
	return callHostMethod[EventPublishResult](ctx, c.caller, "host.events.publish", req)
}

// List enumerates the tasks inside one workflow directory.
func (c *TasksClient) List(ctx context.Context, req TaskListRequest) ([]Task, error) {
	if c == nil || c.caller == nil {
		return nil, fmt.Errorf("host.tasks.list: missing caller")
	}

	var out []Task
	if err := c.caller.call(ctx, "host.tasks.list", req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get reads one task document by workflow and number.
func (c *TasksClient) Get(ctx context.Context, req TaskGetRequest) (*Task, error) {
	return callHostMethod[Task](ctx, c.caller, "host.tasks.get", req)
}

// Create writes one new task document through the host task service.
func (c *TasksClient) Create(ctx context.Context, req TaskCreateRequest) (*Task, error) {
	return callHostMethod[Task](ctx, c.caller, "host.tasks.create", req)
}

// Start launches one child run through the host.
func (c *RunsClient) Start(ctx context.Context, req RunStartRequest) (*RunHandle, error) {
	return callHostMethod[RunHandle](ctx, c.caller, "host.runs.start", req)
}

// Read reads one artifact path through the host.
func (c *ArtifactsClient) Read(
	ctx context.Context,
	req ArtifactReadRequest,
) (*ArtifactReadResult, error) {
	return callHostMethod[ArtifactReadResult](ctx, c.caller, "host.artifacts.read", req)
}

// Write writes one artifact path through the host.
func (c *ArtifactsClient) Write(
	ctx context.Context,
	req ArtifactWriteRequest,
) (*ArtifactWriteResult, error) {
	return callHostMethod[ArtifactWriteResult](ctx, c.caller, "host.artifacts.write", req)
}

// Render renders one host-managed prompt template.
func (c *PromptsClient) Render(
	ctx context.Context,
	req PromptRenderRequest,
) (*PromptRenderResult, error) {
	return callHostMethod[PromptRenderResult](ctx, c.caller, "host.prompts.render", req)
}

// Read reads one workflow or task memory document.
func (c *MemoryClient) Read(ctx context.Context, req MemoryReadRequest) (*MemoryReadResult, error) {
	return callHostMethod[MemoryReadResult](ctx, c.caller, "host.memory.read", req)
}

// Write writes one workflow or task memory document.
func (c *MemoryClient) Write(
	ctx context.Context,
	req MemoryWriteRequest,
) (*MemoryWriteResult, error) {
	return callHostMethod[MemoryWriteResult](ctx, c.caller, "host.memory.write", req)
}

func callHostMethod[T any](ctx context.Context, caller hostCaller, method string, params any) (*T, error) {
	if caller == nil {
		return nil, fmt.Errorf("%s: missing caller", method)
	}

	var out T
	if err := caller.call(ctx, method, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
