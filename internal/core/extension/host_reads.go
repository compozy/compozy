package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/subprocess"
	"github.com/compozy/compozy/internal/core/tasks"
)

func (s *HostServices) handleTasks(
	ctx context.Context,
	_ *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	if s == nil || s.ops == nil {
		return nil, fmt.Errorf("handle host tasks: missing kernel ops")
	}

	switch verb {
	case "list":
		req, err := decodeHostParams[struct {
			Workflow string `json:"workflow"`
		}]("host.tasks.list", params)
		if err != nil {
			return nil, err
		}
		return s.ops.ListTasks(ctx, req.Workflow)
	case "get":
		req, err := decodeHostParams[struct {
			Workflow string `json:"workflow"`
			Number   int    `json:"number"`
		}]("host.tasks.get", params)
		if err != nil {
			return nil, err
		}
		return s.ops.GetTask(ctx, req.Workflow, req.Number)
	case "create":
		return s.handleTasksCreate(ctx, params)
	default:
		return nil, NewMethodNotFoundError("host.tasks." + verb)
	}
}

func (s *HostServices) handleMemory(
	ctx context.Context,
	_ *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	if s == nil || s.ops == nil {
		return nil, fmt.Errorf("handle host memory: missing kernel ops")
	}

	switch verb {
	case "read":
		req, err := decodeHostParams[MemoryReadRequest]("host.memory.read", params)
		if err != nil {
			return nil, err
		}
		return s.ops.ReadMemory(ctx, req)
	case "write":
		return s.handleMemoryWrite(ctx, params)
	default:
		return nil, NewMethodNotFoundError("host.memory." + verb)
	}
}

func (s *HostServices) handleArtifacts(
	ctx context.Context,
	_ *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	if s == nil || s.ops == nil {
		return nil, fmt.Errorf("handle host artifacts: missing kernel ops")
	}

	switch verb {
	case "read":
		req, err := decodeHostParams[ArtifactReadRequest]("host.artifacts.read", params)
		if err != nil {
			return nil, err
		}
		return s.ops.ReadArtifact(ctx, req.Path)
	case "write":
		return s.handleArtifactWrite(ctx, params)
	default:
		return nil, NewMethodNotFoundError("host.artifacts." + verb)
	}
}

func (o *defaultKernelOps) ListTasks(_ context.Context, workflow string) ([]Task, error) {
	tasksDir, err := o.tasksDirForWorkflow(workflow)
	if err != nil {
		return nil, err
	}

	entries, err := tasks.ReadTaskEntries(tasksDir, true)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, err
	}

	result := make([]Task, 0, len(entries))
	for _, entry := range entries {
		task, err := o.parseTaskDocument(workflow, tasks.ExtractTaskNumber(entry.Name), entry.AbsPath, entry.Content)
		if err != nil {
			return nil, err
		}
		result = append(result, *task)
	}
	return result, nil
}

func (o *defaultKernelOps) GetTask(_ context.Context, workflow string, number int) (*Task, error) {
	if number <= 0 {
		return nil, subprocess.NewInvalidParams(map[string]any{
			"method": "host.tasks.get",
			"field":  "number",
			"error":  "number must be positive",
		})
	}

	tasksDir, err := o.tasksDirForWorkflow(workflow)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(tasksDir, fmt.Sprintf("task_%02d.md", number))
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file %s: %w", path, err)
	}
	return o.parseTaskDocument(workflow, number, path, string(content))
}

func (o *defaultKernelOps) ReadMemory(_ context.Context, req MemoryReadRequest) (*MemoryReadResult, error) {
	tasksDir, err := o.tasksDirForWorkflow(req.Workflow)
	if err != nil {
		return nil, err
	}

	document, err := memory.ReadDocument(tasksDir, req.TaskFile)
	if err != nil {
		return nil, err
	}

	return &MemoryReadResult{
		Path:            o.workspaceRelative(document.Path),
		Content:         document.Content,
		Exists:          document.Exists,
		NeedsCompaction: document.NeedsCompaction,
	}, nil
}

func (o *defaultKernelOps) ReadArtifact(_ context.Context, path string) (*ArtifactReadResult, error) {
	scoped, err := o.resolveScopedPath("host.artifacts.read", path)
	if err != nil {
		return nil, err
	}

	root, err := o.openWorkspaceRoot("host.artifacts.read")
	if err != nil {
		return nil, err
	}
	defer root.Close()

	content, err := root.ReadFile(scoped.relative)
	if err != nil {
		if isRootEscapeError(err) {
			return nil, o.pathOutOfScopeError("host.artifacts.read", path)
		}
		return nil, fmt.Errorf("read artifact %s: %w", scoped.absolute, err)
	}
	return &ArtifactReadResult{
		Path:    o.workspaceRelative(scoped.absolute),
		Content: content,
	}, nil
}
