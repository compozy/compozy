package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	if verb != hostRunStartVerb {
		return nil, NewMethodNotFoundError("host.runs." + verb)
	}

	req, err := decodeHostParams[RunStartRequest]("host.runs."+hostRunStartVerb, params)
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
	tasksDir, title, meta, err := o.prepareTaskCreate(ctx, req)
	if err != nil {
		return nil, err
	}

	number, err := nextTaskNumber(tasksDir)
	if err != nil {
		return nil, err
	}
	var indexUpdate *taskIndexUpdate
	if req.UpdateIndex {
		indexUpdate, err = o.prepareTaskIndexUpdate(req.Workflow, tasksDir, number, title, meta)
		if err != nil {
			return nil, err
		}
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
	if indexUpdate != nil {
		if err := o.writeTaskIndexUpdate(*indexUpdate); err != nil {
			return nil, err
		}
	}

	if _, err := o.submitRuntimeEvent(ctx, events.EventKindTaskFileUpdated, kinds.TaskFileUpdatedPayload{
		TasksDir:  tasksDir,
		TaskName:  taskName,
		FilePath:  taskPath,
		NewStatus: meta.Status,
	}); err != nil {
		return nil, err
	}

	refreshedMeta, err := tasks.SnapshotTaskMeta(tasksDir)
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

func (o *defaultKernelOps) prepareTaskCreate(
	ctx context.Context,
	req TaskCreateRequest,
) (string, string, TaskFrontmatter, error) {
	tasksDir, err := o.tasksDirForWorkflow(req.Workflow)
	if err != nil {
		return "", "", TaskFrontmatter{}, err
	}
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return "", "", TaskFrontmatter{}, fmt.Errorf("create tasks directory: %w", err)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return "", "", TaskFrontmatter{}, subprocess.NewInvalidParams(map[string]any{
			"method": "host.tasks.create",
			"field":  "title",
			"error":  "title is required",
		})
	}
	if strings.TrimSpace(req.Body) == "" {
		return "", "", TaskFrontmatter{}, subprocess.NewInvalidParams(map[string]any{
			"method": "host.tasks.create",
			"field":  "body",
			"error":  "body is required",
		})
	}

	meta, err := o.normalizeTaskFrontmatter(ctx, req.Frontmatter)
	if err != nil {
		return "", "", TaskFrontmatter{}, err
	}
	return tasksDir, title, meta, nil
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

	if o.daemonBridge != nil {
		if strings.TrimSpace(o.daemonBridge.HostCapabilityToken()) == "" {
			return nil, NewHostCapabilityTokenInvalidError("host.runs.start", "missing")
		}
		return o.daemonBridge.StartRun(callCtx, runtimeCfg)
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
	resolvedPathStr, content, err := o.writeArtifactFile(
		ctx,
		"host.artifacts.write",
		resolvedPath.absolute,
		req.Content,
		0o600,
	)
	if err != nil {
		return nil, err
	}
	if _, err := o.submitRuntimeEvent(ctx, events.EventKindArtifactUpdated, kinds.ArtifactUpdatedPayload{
		Path:         o.workspaceRelative(resolvedPathStr),
		BytesWritten: len(content),
	}); err != nil {
		return nil, err
	}
	return &ArtifactWriteResult{
		Path:         o.workspaceRelative(resolvedPathStr),
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

type taskIndexUpdate struct {
	absolutePath string
	relativePath string
	content      []byte
}

func (o *defaultKernelOps) prepareTaskIndexUpdate(
	workflow string,
	tasksDir string,
	number int,
	title string,
	meta TaskFrontmatter,
) (*taskIndexUpdate, error) {
	indexPath := filepath.Join(tasksDir, "_tasks.md")
	scoped, err := o.resolveScopedPath("host.tasks.create", indexPath)
	if err != nil {
		return nil, err
	}

	row := formatTaskIndexRow(number, title, meta)
	content, err := os.ReadFile(scoped.absolute)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &taskIndexUpdate{
				absolutePath: scoped.absolute,
				relativePath: scoped.relative,
				content:      []byte(newTaskIndexContent(workflow, row)),
			}, nil
		}
		return nil, fmt.Errorf("read task index %s: %w", scoped.absolute, err)
	}

	updated, err := appendTaskIndexRow(string(content), row)
	if err != nil {
		return nil, fmt.Errorf("update task index %s: %w", scoped.absolute, err)
	}
	return &taskIndexUpdate{
		absolutePath: scoped.absolute,
		relativePath: scoped.relative,
		content:      []byte(updated),
	}, nil
}

func (o *defaultKernelOps) writeTaskIndexUpdate(update taskIndexUpdate) error {
	root, err := o.openWorkspaceRoot("host.tasks.create")
	if err != nil {
		return err
	}
	defer root.Close()

	if err := writeHostFileAtomically(
		root,
		update.relativePath,
		update.absolutePath,
		update.content,
		0o600,
	); err != nil {
		if isRootEscapeError(err) {
			return o.pathOutOfScopeError("host.tasks.create", update.absolutePath)
		}
		return err
	}
	return nil
}

func newTaskIndexContent(workflow string, row string) string {
	return strings.Join([]string{
		fmt.Sprintf("# %s - Task List", taskIndexWorkflowTitle(workflow)),
		"",
		"## Tasks",
		"",
		"| # | Title | Status | Complexity | Dependencies |",
		"|---|-------|--------|------------|--------------|",
		row,
		"",
	}, "\n")
}

func appendTaskIndexRow(content string, row string) (string, error) {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	headerIdx := -1
	for idx, line := range lines {
		if strings.TrimSpace(line) == "| # | Title | Status | Complexity | Dependencies |" {
			headerIdx = idx
			break
		}
	}
	if headerIdx == -1 {
		return "", errors.New("tasks table header not found")
	}
	separatorIdx := headerIdx + 1
	if separatorIdx >= len(lines) || !isTaskIndexSeparator(lines[separatorIdx]) {
		return "", errors.New("tasks table separator not found")
	}

	insertIdx := separatorIdx + 1
	for insertIdx < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[insertIdx]), "|") {
		insertIdx++
	}

	updated := make([]string, 0, len(lines)+1)
	updated = append(updated, lines[:insertIdx]...)
	updated = append(updated, row)
	updated = append(updated, lines[insertIdx:]...)
	return strings.Join(updated, "\n") + "\n", nil
}

func isTaskIndexSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|---") && strings.HasSuffix(trimmed, "|")
}

func formatTaskIndexRow(number int, title string, meta TaskFrontmatter) string {
	status := strings.TrimSpace(meta.Status)
	if status == "" {
		status = taskStatusPending
	}
	complexity := strings.TrimSpace(meta.Complexity)
	if complexity == "" {
		complexity = "-"
	}
	dependencies := "-"
	if len(meta.Dependencies) > 0 {
		dependencies = strings.Join(meta.Dependencies, ", ")
	}
	return fmt.Sprintf(
		"| %02d | %s | %s | %s | %s |",
		number,
		sanitizeTaskIndexCell(title),
		sanitizeTaskIndexCell(status),
		sanitizeTaskIndexCell(complexity),
		sanitizeTaskIndexCell(dependencies),
	)
}

func sanitizeTaskIndexCell(value string) string {
	compact := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return strings.ReplaceAll(compact, "|", `\|`)
}

func taskIndexWorkflowTitle(workflow string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(workflow), func(r rune) bool {
		return r == '-' || r == '_'
	})
	for idx, part := range parts {
		if part == "" {
			continue
		}
		parts[idx] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return "Workflow"
	}
	return strings.Join(parts, " ")
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
		finalPath = resolvedPath.absolute
	}
	if payload.Content != nil {
		finalContent = []byte(*payload.Content)
	}

	scoped, err := o.resolveScopedPath(method, finalPath)
	if err != nil {
		return "", nil, err
	}

	root, err := o.openWorkspaceRoot(method)
	if err != nil {
		return "", nil, err
	}
	defer root.Close()

	if err := writeHostFileAtomically(root, scoped.relative, scoped.absolute, finalContent, perm); err != nil {
		if isRootEscapeError(err) {
			return "", nil, o.pathOutOfScopeError(method, finalPath)
		}
		return "", nil, err
	}
	finalPath = scoped.absolute

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

func writeHostFileAtomically(
	root *os.Root,
	relativePath string,
	absolutePath string,
	content []byte,
	perm os.FileMode,
) error {
	dir := filepath.Dir(relativePath)
	if dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("create artifact parent dir for %s: %w", absolutePath, err)
		}
	}

	tmpPath, tmpFile, err := createScopedTempFile(root, relativePath, perm)
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", absolutePath, err)
	}

	removeTemp := func() error {
		if err := root.Remove(tmpPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove temp file for %s: %w", absolutePath, err)
		}
		return nil
	}
	cleanup := func() error {
		return errors.Join(tmpFile.Close(), removeTemp())
	}

	if _, err := tmpFile.Write(content); err != nil {
		return errors.Join(
			fmt.Errorf("write temp file for %s: %w", absolutePath, err),
			cleanup(),
		)
	}
	if err := tmpFile.Sync(); err != nil {
		return errors.Join(
			fmt.Errorf("sync temp file for %s: %w", absolutePath, err),
			cleanup(),
		)
	}
	if err := tmpFile.Close(); err != nil {
		return errors.Join(
			fmt.Errorf("close temp file for %s: %w", absolutePath, err),
			removeTemp(),
		)
	}
	if err := root.Rename(tmpPath, relativePath); err != nil {
		return errors.Join(
			fmt.Errorf("replace %s: %w", absolutePath, err),
			removeTemp(),
		)
	}
	return nil
}

func createScopedTempFile(root *os.Root, relativePath string, perm os.FileMode) (string, *os.File, error) {
	dir := filepath.Dir(relativePath)
	if dir == "." {
		dir = ""
	}

	base := "." + filepath.Base(relativePath) + ".tmp"
	for attempt := 0; attempt < 16; attempt++ {
		candidate := fmt.Sprintf("%s-%d-%d", base, time.Now().UTC().UnixNano(), attempt)
		if dir != "" {
			candidate = filepath.Join(dir, candidate)
		}

		file, err := root.OpenFile(candidate, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			return candidate, file, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return "", nil, err
	}

	return "", nil, fmt.Errorf("exhausted temp names for %s", relativePath)
}
