package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/subprocess"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func (s *HostServices) handleTasksCreate(ctx context.Context, params json.RawMessage) (any, error) {
	req, err := decodeHostParams[TaskCreateRequest]("host.tasks.create", params)
	if err != nil {
		return nil, err
	}
	return s.ops.CreateTask(ctx, req)
}

func (s *HostServices) handleRuns(
	ctx context.Context,
	_ *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	if s == nil || s.ops == nil {
		return nil, fmt.Errorf("handle host runs: missing kernel ops")
	}
	if verb != "start" {
		return nil, NewMethodNotFoundError("host.runs." + verb)
	}

	req, err := decodeHostParams[RunStartRequest]("host.runs.start", params)
	if err != nil {
		return nil, err
	}
	return s.ops.StartRun(ctx, req)
}

func (s *HostServices) handleMemoryWrite(ctx context.Context, params json.RawMessage) (any, error) {
	req, err := decodeHostParams[MemoryWriteRequest]("host.memory.write", params)
	if err != nil {
		return nil, err
	}
	return s.ops.WriteMemory(ctx, req)
}

func (s *HostServices) handleArtifactWrite(ctx context.Context, params json.RawMessage) (any, error) {
	req, err := decodeHostParams[ArtifactWriteRequest]("host.artifacts.write", params)
	if err != nil {
		return nil, err
	}
	return s.ops.WriteArtifact(ctx, req)
}

func (o *defaultKernelOps) CreateTask(ctx context.Context, req TaskCreateRequest) (*Task, error) {
	tasksDir, err := o.tasksDirForWorkflow(req.Workflow)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return nil, fmt.Errorf("create tasks directory: %w", err)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, subprocess.NewInvalidParams(map[string]any{
			"method": "host.tasks.create",
			"field":  "title",
			"error":  "title is required",
		})
	}
	if strings.TrimSpace(req.Body) == "" {
		return nil, subprocess.NewInvalidParams(map[string]any{
			"method": "host.tasks.create",
			"field":  "body",
			"error":  "body is required",
		})
	}

	meta, err := o.normalizeTaskFrontmatter(ctx, req.Frontmatter)
	if err != nil {
		return nil, err
	}

	number, err := nextTaskNumber(tasksDir)
	if err != nil {
		return nil, err
	}
	taskName := fmt.Sprintf("task_%02d.md", number)
	taskPath := filepath.Join(tasksDir, taskName)
	taskBody := buildTaskBody(number, title, req.Body)
	content, err := frontmatter.Format(model.TaskFileMeta{
		Status:       meta.Status,
		Title:        title,
		TaskType:     meta.Type,
		Complexity:   meta.Complexity,
		Dependencies: meta.Dependencies,
	}, taskBody)
	if err != nil {
		return nil, fmt.Errorf("format task file %s: %w", taskPath, err)
	}
	taskPath, taskContent, err := o.writeArtifactFile(ctx, "host.tasks.create", taskPath, []byte(content), 0o600)
	if err != nil {
		return nil, err
	}
	taskName = filepath.Base(taskPath)

	if _, err := o.submitRuntimeEvent(ctx, events.EventKindTaskFileUpdated, kinds.TaskFileUpdatedPayload{
		TasksDir:  tasksDir,
		TaskName:  taskName,
		FilePath:  taskPath,
		NewStatus: meta.Status,
	}); err != nil {
		return nil, err
	}

	refreshedMeta, err := tasks.RefreshTaskMeta(tasksDir)
	if err != nil {
		return nil, err
	}
	if _, err := o.submitRuntimeEvent(ctx, events.EventKindTaskMetadataRefreshed, kinds.TaskMetadataRefreshedPayload{
		TasksDir:  tasksDir,
		CreatedAt: refreshedMeta.CreatedAt,
		UpdatedAt: refreshedMeta.UpdatedAt,
		Total:     refreshedMeta.Total,
		Completed: refreshedMeta.Completed,
		Pending:   refreshedMeta.Pending,
	}); err != nil {
		return nil, err
	}

	return o.parseTaskDocument(req.Workflow, tasks.ExtractTaskNumber(taskName), taskPath, string(taskContent))
}

func (o *defaultKernelOps) StartRun(ctx context.Context, req RunStartRequest) (*RunHandle, error) {
	parentChain := append([]string(nil), o.parentChain...)
	if len(parentChain) >= 3 {
		return nil, NewRecursionDepthExceededError("host.runs.start", strings.Join(parentChain, ","), len(parentChain))
	}

	parentRunID := strings.Join(append(parentChain, strings.TrimSpace(o.runID)), ",")
	runtimeCfg := req.Runtime.toRuntimeConfig(o.workspaceRoot, parentRunID)
	if strings.TrimSpace(runtimeCfg.RunID) == "" {
		runtimeCfg.RunID = hostGeneratedRunID(runtimeCfg.Mode)
	}

	callCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, defaultHostAPITimeout)
		defer cancel()
	}

	result, err := kernel.Dispatch[commands.RunStartCommand, commands.RunStartResult](
		callCtx,
		o.dispatcher,
		commands.RunStartCommand{Runtime: *runtimeCfg},
	)
	if err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(result.RunID)
	if runID == "" {
		runID = runtimeCfg.RunID
	}
	return &RunHandle{
		RunID:       runID,
		ParentRunID: strings.Trim(parentRunID, ","),
	}, nil
}

func (o *defaultKernelOps) WriteMemory(ctx context.Context, req MemoryWriteRequest) (*MemoryWriteResult, error) {
	tasksDir, err := o.tasksDirForWorkflow(req.Workflow)
	if err != nil {
		return nil, err
	}

	document, bytesWritten, err := memory.WriteDocument(tasksDir, req.TaskFile, req.Content, req.Mode)
	if err != nil {
		return nil, err
	}
	if _, err := o.submitRuntimeEvent(ctx, events.EventKindTaskMemoryUpdated, kinds.TaskMemoryUpdatedPayload{
		Workflow:     strings.TrimSpace(req.Workflow),
		TaskFile:     strings.TrimSpace(req.TaskFile),
		Path:         o.workspaceRelative(document.Path),
		Mode:         string(req.Mode),
		BytesWritten: bytesWritten,
	}); err != nil {
		return nil, err
	}
	return &MemoryWriteResult{
		Path:         o.workspaceRelative(document.Path),
		BytesWritten: bytesWritten,
	}, nil
}

func (o *defaultKernelOps) WriteArtifact(ctx context.Context, req ArtifactWriteRequest) (*ArtifactWriteResult, error) {
	resolvedPath, err := o.resolveScopedPath("host.artifacts.write", req.Path)
	if err != nil {
		return nil, err
	}
	resolvedPath, content, err := o.writeArtifactFile(ctx, "host.artifacts.write", resolvedPath, req.Content, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := o.submitRuntimeEvent(ctx, events.EventKindArtifactUpdated, kinds.ArtifactUpdatedPayload{
		Path:         o.workspaceRelative(resolvedPath),
		BytesWritten: len(content),
	}); err != nil {
		return nil, err
	}
	return &ArtifactWriteResult{
		Path:         o.workspaceRelative(resolvedPath),
		BytesWritten: len(content),
	}, nil
}

func (cfg RunConfig) toRuntimeConfig(workspaceRoot string, parentRunID string) *model.RuntimeConfig {
	runtimeCfg := &model.RuntimeConfig{
		WorkspaceRoot:          strings.TrimSpace(cfg.WorkspaceRoot),
		Name:                   strings.TrimSpace(cfg.Name),
		Round:                  cfg.Round,
		Provider:               strings.TrimSpace(cfg.Provider),
		PR:                     strings.TrimSpace(cfg.PR),
		ReviewsDir:             strings.TrimSpace(cfg.ReviewsDir),
		TasksDir:               strings.TrimSpace(cfg.TasksDir),
		AutoCommit:             cfg.AutoCommit,
		Concurrent:             cfg.Concurrent,
		BatchSize:              cfg.BatchSize,
		IDE:                    strings.TrimSpace(cfg.IDE),
		Model:                  strings.TrimSpace(cfg.Model),
		AddDirs:                append([]string(nil), cfg.AddDirs...),
		TailLines:              cfg.TailLines,
		ReasoningEffort:        strings.TrimSpace(cfg.ReasoningEffort),
		AccessMode:             strings.TrimSpace(cfg.AccessMode),
		Mode:                   cfg.Mode,
		OutputFormat:           cfg.OutputFormat,
		Verbose:                cfg.Verbose,
		TUI:                    cfg.TUI,
		Persist:                cfg.Persist,
		RunID:                  strings.TrimSpace(cfg.RunID),
		ParentRunID:            strings.TrimSpace(parentRunID),
		PromptText:             cfg.PromptText,
		PromptFile:             strings.TrimSpace(cfg.PromptFile),
		ReadPromptStdin:        cfg.ReadPromptStdin,
		IncludeCompleted:       cfg.IncludeCompleted,
		IncludeResolved:        cfg.IncludeResolved,
		MaxRetries:             cfg.MaxRetries,
		RetryBackoffMultiplier: cfg.RetryBackoffMultiplier,
	}
	if runtimeCfg.WorkspaceRoot == "" {
		runtimeCfg.WorkspaceRoot = workspaceRoot
	}
	if cfg.TimeoutMS > 0 {
		runtimeCfg.Timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
	}
	runtimeCfg.ApplyDefaults()
	return runtimeCfg
}

func nextTaskNumber(tasksDir string) (int, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return 0, fmt.Errorf("read tasks directory %s: %w", tasksDir, err)
	}

	maxNumber := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		number := tasks.ExtractTaskNumber(entry.Name())
		if number > maxNumber {
			maxNumber = number
		}
	}
	return maxNumber + 1, nil
}

func buildTaskBody(number int, title string, body string) string {
	return fmt.Sprintf(
		"# Task %02d: %s\n\n%s\n",
		number,
		strings.TrimSpace(title),
		strings.TrimSpace(body),
	)
}

func hostGeneratedRunID(mode model.ExecutionMode) string {
	label := "run"
	switch mode {
	case model.ExecutionModeExec:
		label = executionModeLabelExec
	case model.ExecutionModePRDTasks:
		label = "tasks"
	case model.ExecutionModePRReview:
		label = "reviews"
	}
	return fmt.Sprintf("%s-host-%s", label, time.Now().UTC().Format("20060102-150405-000000000"))
}

func (o *defaultKernelOps) writeArtifactFile(
	ctx context.Context,
	method string,
	path string,
	content []byte,
	perm os.FileMode,
) (string, []byte, error) {
	finalPath := path
	finalContent := append([]byte(nil), content...)

	payload, err := model.DispatchMutableHook(
		ctx,
		o.runtimeManager,
		"artifact.pre_write",
		artifactPreWritePayload{
			RunID:          o.runID,
			Path:           o.workspaceRelative(path),
			ContentPreview: contentPreview(finalContent),
		},
	)
	if err != nil {
		return "", nil, err
	}
	if payload.Cancel {
		return "", nil, NewCancelledByExtensionError(method, payload.Path)
	}
	if trimmed := strings.TrimSpace(payload.Path); trimmed != "" {
		resolvedPath, err := o.resolveScopedPath(method, trimmed)
		if err != nil {
			return "", nil, err
		}
		finalPath = resolvedPath
	}
	if payload.Content != nil {
		finalContent = []byte(*payload.Content)
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return "", nil, fmt.Errorf("create artifact parent dir: %w", err)
	}
	if err := writeHostFileAtomically(finalPath, finalContent, perm); err != nil {
		return "", nil, err
	}

	model.DispatchObserverHook(
		ctx,
		o.runtimeManager,
		"artifact.post_write",
		artifactPostWritePayload{
			RunID:        o.runID,
			Path:         o.workspaceRelative(finalPath),
			BytesWritten: len(finalContent),
		},
	)
	return finalPath, finalContent, nil
}

type artifactPreWritePayload struct {
	RunID          string  `json:"run_id"`
	Path           string  `json:"path"`
	ContentPreview string  `json:"content_preview,omitempty"`
	Content        *string `json:"content,omitempty"`
	Cancel         bool    `json:"cancel,omitempty"`
}

type artifactPostWritePayload struct {
	RunID        string `json:"run_id"`
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

func contentPreview(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	preview := string(content)
	const limit = 256
	if len(preview) <= limit {
		return preview
	}
	return preview[:limit]
}

func writeHostFileAtomically(path string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(content); err != nil {
		cleanup()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
