package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
	coreprompt "github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/subprocess"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	hostPromptTemplateBuild          = "build"
	hostPromptTemplateSystemAddendum = "system_addendum"
	defaultHostAPITimeout            = 30 * time.Second
	taskStatusPending                = "pending"
)

var (
	validTaskStatuses = map[string]struct{}{
		taskStatusPending: {},
		"in_progress":     {},
		"completed":       {},
		"blocked":         {},
	}
	validTaskComplexities = map[string]struct{}{"low": {}, "medium": {}, "high": {}, "critical": {}}
)

type KernelOps interface {
	CreateTask(ctx context.Context, req TaskCreateRequest) (*Task, error)
	ListTasks(ctx context.Context, workflow string) ([]Task, error)
	GetTask(ctx context.Context, workflow string, number int) (*Task, error)
	StartRun(ctx context.Context, req RunStartRequest) (*RunHandle, error)
	ReadArtifact(ctx context.Context, path string) (*ArtifactReadResult, error)
	WriteArtifact(ctx context.Context, req ArtifactWriteRequest) (*ArtifactWriteResult, error)
	RenderPrompt(ctx context.Context, req PromptRenderRequest) (*PromptRenderResult, error)
	ReadMemory(ctx context.Context, req MemoryReadRequest) (*MemoryReadResult, error)
	WriteMemory(ctx context.Context, req MemoryWriteRequest) (*MemoryWriteResult, error)
	PublishEvent(ctx context.Context, extensionName string, req EventPublishRequest) (*EventPublishResult, error)
}

type DefaultKernelOpsConfig struct {
	WorkspaceRoot  string
	RunID          string
	ParentRunID    string
	Dispatcher     *kernel.Dispatcher
	EventBus       *events.Bus[events.Event]
	Journal        *journal.Journal
	RuntimeManager model.RuntimeManager
}

type defaultKernelOps struct {
	workspaceRoot  string
	runID          string
	parentChain    []string
	dispatcher     *kernel.Dispatcher
	eventBus       *events.Bus[events.Event]
	journal        *journal.Journal
	runtimeManager model.RuntimeManager
}

var _ KernelOps = (*defaultKernelOps)(nil)

type HostServices struct {
	ops                   KernelOps
	subscriptionIDCounter atomic.Uint64
}

type TaskFrontmatter struct {
	Status       string   `json:"status"`
	Type         string   `json:"type"`
	Complexity   string   `json:"complexity,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

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

type TaskCreateRequest struct {
	Workflow    string          `json:"workflow"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	Frontmatter TaskFrontmatter `json:"frontmatter"`
}

type RunStartRequest struct {
	Runtime RunConfig `json:"runtime"`
}

type RunConfig struct {
	WorkspaceRoot          string              `json:"workspace_root,omitempty"`
	Name                   string              `json:"name,omitempty"`
	Round                  int                 `json:"round,omitempty"`
	Provider               string              `json:"provider,omitempty"`
	PR                     string              `json:"pr,omitempty"`
	ReviewsDir             string              `json:"reviews_dir,omitempty"`
	TasksDir               string              `json:"tasks_dir,omitempty"`
	AutoCommit             bool                `json:"auto_commit,omitempty"`
	Concurrent             int                 `json:"concurrent,omitempty"`
	BatchSize              int                 `json:"batch_size,omitempty"`
	IDE                    string              `json:"ide,omitempty"`
	Model                  string              `json:"model,omitempty"`
	AddDirs                []string            `json:"add_dirs,omitempty"`
	TailLines              int                 `json:"tail_lines,omitempty"`
	ReasoningEffort        string              `json:"reasoning_effort,omitempty"`
	AccessMode             string              `json:"access_mode,omitempty"`
	Mode                   model.ExecutionMode `json:"mode,omitempty"`
	OutputFormat           model.OutputFormat  `json:"output_format,omitempty"`
	Verbose                bool                `json:"verbose,omitempty"`
	TUI                    bool                `json:"tui,omitempty"`
	Persist                bool                `json:"persist,omitempty"`
	RunID                  string              `json:"run_id,omitempty"`
	PromptText             string              `json:"prompt_text,omitempty"`
	PromptFile             string              `json:"prompt_file,omitempty"`
	ReadPromptStdin        bool                `json:"read_prompt_stdin,omitempty"`
	IncludeCompleted       bool                `json:"include_completed,omitempty"`
	IncludeResolved        bool                `json:"include_resolved,omitempty"`
	TimeoutMS              int64               `json:"timeout_ms,omitempty"`
	MaxRetries             int                 `json:"max_retries,omitempty"`
	RetryBackoffMultiplier float64             `json:"retry_backoff_multiplier,omitempty"`
}

type RunHandle struct {
	RunID       string `json:"run_id"`
	ParentRunID string `json:"parent_run_id,omitempty"`
}

type ArtifactReadRequest struct {
	Path string `json:"path"`
}

type ArtifactReadResult struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

type ArtifactWriteRequest struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

type ArtifactWriteResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

type PromptRenderRequest struct {
	Template string             `json:"template"`
	Params   PromptRenderParams `json:"params"`
}

type PromptRenderParams struct {
	Name        string                      `json:"name,omitempty"`
	Round       int                         `json:"round,omitempty"`
	Provider    string                      `json:"provider,omitempty"`
	PR          string                      `json:"pr,omitempty"`
	ReviewsDir  string                      `json:"reviews_dir,omitempty"`
	BatchGroups map[string][]PromptIssueRef `json:"batch_groups,omitempty"`
	AutoCommit  bool                        `json:"auto_commit,omitempty"`
	Mode        model.ExecutionMode         `json:"mode,omitempty"`
	Memory      *PromptMemoryContext        `json:"memory,omitempty"`
}

type PromptIssueRef struct {
	Name     string `json:"name"`
	AbsPath  string `json:"abs_path,omitempty"`
	Content  string `json:"content,omitempty"`
	CodeFile string `json:"code_file,omitempty"`
}

type PromptMemoryContext struct {
	Directory               string `json:"directory,omitempty"`
	WorkflowPath            string `json:"workflow_path,omitempty"`
	TaskPath                string `json:"task_path,omitempty"`
	WorkflowNeedsCompaction bool   `json:"workflow_needs_compaction,omitempty"`
	TaskNeedsCompaction     bool   `json:"task_needs_compaction,omitempty"`
}

type PromptRenderResult struct {
	Rendered string `json:"rendered"`
}

type MemoryReadRequest struct {
	Workflow string `json:"workflow"`
	TaskFile string `json:"task_file,omitempty"`
}

type MemoryReadResult struct {
	Path            string `json:"path"`
	Content         string `json:"content"`
	Exists          bool   `json:"exists"`
	NeedsCompaction bool   `json:"needs_compaction"`
}

type MemoryWriteRequest struct {
	Workflow string           `json:"workflow"`
	TaskFile string           `json:"task_file,omitempty"`
	Content  string           `json:"content"`
	Mode     memory.WriteMode `json:"mode,omitempty"`
}

type MemoryWriteResult struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

type EventSubscribeRequest struct {
	Kinds []events.EventKind `json:"kinds"`
}

type EventSubscribeResult struct {
	SubscriptionID string `json:"subscription_id"`
}

type EventPublishRequest struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type EventPublishResult struct {
	Seq uint64 `json:"seq,omitempty"`
}

func NewDefaultKernelOps(cfg DefaultKernelOpsConfig) (KernelOps, error) {
	workspaceRoot := strings.TrimSpace(cfg.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil, fmt.Errorf("new default kernel ops: workspace root is required")
	}
	resolvedRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("new default kernel ops: resolve workspace root: %w", err)
	}

	dispatcher := cfg.Dispatcher
	if dispatcher == nil {
		dispatcher = kernel.BuildDefault(kernel.KernelDeps{
			AgentRegistry: agent.DefaultRegistry(),
		})
		if err := kernel.ValidateDefaultRegistry(dispatcher); err != nil {
			return nil, fmt.Errorf("new default kernel ops: %w", err)
		}
	}

	parentRunID := strings.TrimSpace(cfg.ParentRunID)
	if parentRunID == "" {
		parentRunID = strings.TrimSpace(os.Getenv("COMPOZY_PARENT_RUN_ID"))
	}

	return &defaultKernelOps{
		workspaceRoot:  resolvedRoot,
		runID:          strings.TrimSpace(cfg.RunID),
		parentChain:    splitParentChain(parentRunID),
		dispatcher:     dispatcher,
		eventBus:       cfg.EventBus,
		journal:        cfg.Journal,
		runtimeManager: cfg.RuntimeManager,
	}, nil
}

func NewHostServices(ops KernelOps) *HostServices {
	return &HostServices{ops: ops}
}

func RegisterHostServices(router *HostAPIRouter, ops KernelOps) error {
	services := NewHostServices(ops)
	for namespace, handler := range map[string]HostAPIService{
		"host.tasks":     HostAPIServiceFunc(services.handleTasks),
		"host.runs":      HostAPIServiceFunc(services.handleRuns),
		"host.memory":    HostAPIServiceFunc(services.handleMemory),
		"host.artifacts": HostAPIServiceFunc(services.handleArtifacts),
		"host.prompts":   HostAPIServiceFunc(services.handlePrompts),
		"host.events":    HostAPIServiceFunc(services.handleEvents),
	} {
		if err := router.RegisterService(namespace, handler); err != nil {
			return err
		}
	}
	return nil
}

func (s *HostServices) handlePrompts(
	ctx context.Context,
	_ *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	if s == nil || s.ops == nil {
		return nil, fmt.Errorf("handle host prompts: missing kernel ops")
	}
	if verb != "render" {
		return nil, NewMethodNotFoundError("host.prompts." + strings.TrimSpace(verb))
	}

	req, err := decodeHostParams[PromptRenderRequest]("host.prompts.render", params)
	if err != nil {
		return nil, err
	}
	return s.ops.RenderPrompt(ctx, req)
}

func (s *HostServices) handleEvents(
	ctx context.Context,
	extension *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	if s == nil || s.ops == nil {
		return nil, fmt.Errorf("handle host events: missing kernel ops")
	}

	switch strings.TrimSpace(verb) {
	case "subscribe":
		req, err := decodeHostParams[EventSubscribeRequest]("host.events.subscribe", params)
		if err != nil {
			return nil, err
		}

		subscriptionID := fmt.Sprintf("sub-%020d", s.subscriptionIDCounter.Add(1))
		if extension != nil {
			extension.SetEventSubscription(subscriptionID, req.Kinds)
		}
		return &EventSubscribeResult{SubscriptionID: subscriptionID}, nil
	case "publish":
		req, err := decodeHostParams[EventPublishRequest]("host.events.publish", params)
		if err != nil {
			return nil, err
		}
		extensionName := ""
		if extension != nil {
			extensionName = extension.normalizedName()
		}
		return s.ops.PublishEvent(ctx, extensionName, req)
	default:
		return nil, NewMethodNotFoundError("host.events." + strings.TrimSpace(verb))
	}
}

func decodeHostParams[T any](method string, params json.RawMessage) (T, error) {
	var zero T

	reader := bytes.NewReader(params)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var payload T
	if err := decoder.Decode(&payload); err != nil {
		return zero, subprocess.NewInvalidParams(map[string]any{
			"method": method,
			"error":  err.Error(),
		})
	}
	if decoder.More() {
		return zero, subprocess.NewInvalidParams(map[string]any{
			"method": method,
			"error":  "unexpected trailing data",
		})
	}
	return payload, nil
}

func (o *defaultKernelOps) RenderPrompt(
	_ context.Context,
	req PromptRenderRequest,
) (*PromptRenderResult, error) {
	params := coreprompt.BatchParams{
		Name:        strings.TrimSpace(req.Params.Name),
		Round:       req.Params.Round,
		Provider:    strings.TrimSpace(req.Params.Provider),
		PR:          strings.TrimSpace(req.Params.PR),
		ReviewsDir:  strings.TrimSpace(req.Params.ReviewsDir),
		BatchGroups: make(map[string][]model.IssueEntry, len(req.Params.BatchGroups)),
		AutoCommit:  req.Params.AutoCommit,
		Mode:        req.Params.Mode,
	}
	for key, items := range req.Params.BatchGroups {
		group := make([]model.IssueEntry, 0, len(items))
		for _, item := range items {
			group = append(group, model.IssueEntry{
				Name:     item.Name,
				AbsPath:  item.AbsPath,
				Content:  item.Content,
				CodeFile: item.CodeFile,
			})
		}
		params.BatchGroups[key] = group
	}
	if req.Params.Memory != nil {
		params.Memory = &coreprompt.WorkflowMemoryContext{
			Directory:               req.Params.Memory.Directory,
			WorkflowPath:            req.Params.Memory.WorkflowPath,
			TaskPath:                req.Params.Memory.TaskPath,
			WorkflowNeedsCompaction: req.Params.Memory.WorkflowNeedsCompaction,
			TaskNeedsCompaction:     req.Params.Memory.TaskNeedsCompaction,
		}
	}

	var (
		rendered string
		err      error
	)
	switch strings.TrimSpace(req.Template) {
	case hostPromptTemplateBuild:
		rendered, err = coreprompt.Build(params)
	case hostPromptTemplateSystemAddendum:
		rendered, err = coreprompt.BuildSystemPromptAddendum(params)
	default:
		return nil, NewMethodNotFoundError("host.prompts.render." + strings.TrimSpace(req.Template))
	}
	if err != nil {
		return nil, err
	}

	return &PromptRenderResult{Rendered: rendered}, nil
}

func (o *defaultKernelOps) PublishEvent(
	ctx context.Context,
	extensionName string,
	req EventPublishRequest,
) (*EventPublishResult, error) {
	payload := kinds.ExtensionEventPayload{
		Extension: strings.TrimSpace(extensionName),
		Kind:      strings.TrimSpace(req.Kind),
		Payload:   append(json.RawMessage(nil), req.Payload...),
	}
	if payload.Kind == "" {
		return nil, subprocess.NewInvalidParams(map[string]any{
			"method": "host.events.publish",
			"field":  "kind",
			"error":  "kind is required",
		})
	}

	seq, err := o.submitRuntimeEvent(ctx, events.EventKindExtensionEvent, payload)
	if err != nil {
		return nil, err
	}
	return &EventPublishResult{Seq: seq}, nil
}

func (o *defaultKernelOps) submitRuntimeEvent(ctx context.Context, kind events.EventKind, payload any) (uint64, error) {
	if o == nil {
		return 0, fmt.Errorf("submit runtime event: kernel ops is nil")
	}
	event, err := newHostRuntimeEvent(o.runID, kind, payload)
	if err != nil {
		return 0, err
	}
	switch {
	case o.journal != nil:
		return o.journal.SubmitWithSeq(ctx, event)
	case o.eventBus != nil:
		o.eventBus.Publish(ctx, event)
		return 0, nil
	default:
		return 0, nil
	}
}

func newHostRuntimeEvent(runID string, kind events.EventKind, payload any) (events.Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("marshal %s payload: %w", kind, err)
	}
	return events.Event{
		RunID:   strings.TrimSpace(runID),
		Kind:    kind,
		Payload: raw,
	}, nil
}

func (o *defaultKernelOps) tasksDirForWorkflow(workflow string) (string, error) {
	trimmed := strings.TrimSpace(workflow)
	if trimmed == "" {
		return "", subprocess.NewInvalidParams(map[string]any{
			"field": "workflow",
			"error": "workflow is required",
		})
	}
	if trimmed != filepath.Base(trimmed) || !model.IsActiveWorkflowDirName(trimmed) {
		return "", subprocess.NewInvalidParams(map[string]any{
			"field": "workflow",
			"error": "workflow must be a single active workflow directory name",
		})
	}
	return model.TaskDirectoryForWorkspace(o.workspaceRoot, trimmed), nil
}

func (o *defaultKernelOps) workspaceRelative(path string) string {
	if o == nil {
		return strings.TrimSpace(path)
	}
	rel, err := filepath.Rel(o.workspaceRoot, path)
	if err != nil {
		return filepath.Clean(path)
	}
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func (o *defaultKernelOps) resolveScopedPath(method string, rawPath string) (string, error) {
	if o == nil {
		return "", fmt.Errorf("resolve scoped path: kernel ops is nil")
	}

	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", subprocess.NewInvalidParams(map[string]any{
			"method": method,
			"field":  "path",
			"error":  "path is required",
		})
	}

	var resolved string
	if filepath.IsAbs(trimmed) {
		resolved = filepath.Clean(trimmed)
	} else {
		resolved = filepath.Clean(filepath.Join(o.workspaceRoot, trimmed))
	}

	workspaceRoot := filepath.Clean(o.workspaceRoot)
	compozyRoot := filepath.Clean(model.CompozyDir(o.workspaceRoot))
	if pathWithinRoot(resolved, workspaceRoot) || pathWithinRoot(resolved, compozyRoot) {
		return resolved, nil
	}
	return "", NewPathOutOfScopeError(method, rawPath, []string{".compozy/", workspaceRoot})
}

func (o *defaultKernelOps) resolveTaskTypeRegistry(ctx context.Context) (*tasks.TypeRegistry, error) {
	cfg, _, err := workspace.LoadConfig(ctx, o.workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("load workspace config for task type registry: %w", err)
	}
	if cfg.Tasks.Types == nil {
		return tasks.NewRegistry(nil)
	}
	return tasks.NewRegistry(*cfg.Tasks.Types)
}

func (o *defaultKernelOps) parseTaskDocument(
	workflow string,
	number int,
	path string,
	content string,
) (*Task, error) {
	var meta model.TaskFileMeta
	body, err := frontmatter.Parse(content, &meta)
	if err != nil {
		return nil, fmt.Errorf("parse task document %s: %w", path, err)
	}

	return &Task{
		Workflow:     workflow,
		Number:       number,
		Path:         o.workspaceRelative(path),
		Status:       strings.TrimSpace(meta.Status),
		Title:        strings.TrimSpace(meta.Title),
		Type:         strings.TrimSpace(meta.TaskType),
		Complexity:   strings.TrimSpace(meta.Complexity),
		Dependencies: append([]string(nil), meta.Dependencies...),
		Body:         body,
	}, nil
}

func (o *defaultKernelOps) normalizeTaskFrontmatter(
	ctx context.Context,
	frontmatter TaskFrontmatter,
) (TaskFrontmatter, error) {
	normalized := TaskFrontmatter{
		Status:       strings.TrimSpace(frontmatter.Status),
		Type:         strings.TrimSpace(frontmatter.Type),
		Complexity:   strings.TrimSpace(frontmatter.Complexity),
		Dependencies: normalizeDependencies(frontmatter.Dependencies),
	}
	if normalized.Status == "" {
		normalized.Status = taskStatusPending
	}
	if _, ok := validTaskStatuses[normalized.Status]; !ok {
		return TaskFrontmatter{}, subprocess.NewInvalidParams(map[string]any{
			"method": "host.tasks.create",
			"field":  "frontmatter.status",
			"error":  fmt.Sprintf("unsupported status %q", normalized.Status),
		})
	}
	if normalized.Complexity != "" {
		if _, ok := validTaskComplexities[normalized.Complexity]; !ok {
			return TaskFrontmatter{}, subprocess.NewInvalidParams(map[string]any{
				"method": "host.tasks.create",
				"field":  "frontmatter.complexity",
				"error":  fmt.Sprintf("unsupported complexity %q", normalized.Complexity),
			})
		}
	}
	registry, err := o.resolveTaskTypeRegistry(ctx)
	if err != nil {
		return TaskFrontmatter{}, err
	}
	if !registry.IsAllowed(normalized.Type) {
		return TaskFrontmatter{}, subprocess.NewInvalidParams(map[string]any{
			"method":        "host.tasks.create",
			"field":         "frontmatter.type",
			"error":         fmt.Sprintf("unsupported task type %q", normalized.Type),
			"allowed_types": registry.Values(),
		})
	}
	return normalized, nil
}

func normalizeDependencies(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func splitParentChain(parentRunID string) []string {
	if strings.TrimSpace(parentRunID) == "" {
		return nil
	}
	items := strings.Split(parentRunID, ",")
	chain := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		chain = append(chain, trimmed)
	}
	return chain
}

func pathWithinRoot(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}
