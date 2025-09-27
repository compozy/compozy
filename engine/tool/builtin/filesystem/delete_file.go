package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

type DeleteFileArgs struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

var deleteFileInputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"path"},
	"properties": map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Path to the file or directory to delete relative to the sandbox root.",
		},
		"recursive": map[string]any{
			"type":        "boolean",
			"description": "Allow recursive deletion when the path is a directory.",
		},
	},
}

var deleteFileOutputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"success"},
	"properties": map[string]any{
		"success": map[string]any{"type": "boolean"},
	},
}

func DeleteFileDefinition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            "cp__delete_file",
		Description:   "Delete a file or directory inside the sandboxed filesystem.",
		InputSchema:   deleteFileInputSchema,
		OutputSchema:  deleteFileOutputSchema,
		ArgsPrototype: DeleteFileArgs{},
		Handler:       deleteFileHandler,
	}
}

func deleteFileHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	var success bool
	defer func() {
		status := builtin.StatusFailure
		if success {
			status = builtin.StatusSuccess
		}
		builtin.RecordInvocation(
			ctx,
			"cp__delete_file",
			builtin.RequestIDFromContext(ctx),
			status,
			time.Since(start),
			0,
			"",
		)
	}()
	cfg, err := loadToolConfig(ctx)
	if err != nil {
		return nil, err
	}
	args, err := decodeArgs[DeleteFileArgs](payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	resolvedPath, rootUsed, err := resolvePath(cfg, args.Path)
	if err != nil {
		return nil, err
	}
	if err := ensureParentsSafe(rootUsed, resolvedPath); err != nil {
		return nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return core.Output{"success": false}, nil
		}
		return nil, builtin.Internal(fmt.Errorf("failed to stat path: %w", err), map[string]any{"path": args.Path})
	}
	if err := rejectSymlink(info, map[string]any{"path": args.Path}); err != nil {
		return nil, err
	}
	if info.IsDir() {
		if !args.Recursive {
			return nil, builtin.InvalidArgument(
				errors.New("recursive flag required for directory deletion"),
				map[string]any{"path": args.Path},
			)
		}
		if err := os.RemoveAll(resolvedPath); err != nil {
			return nil, builtin.Internal(
				fmt.Errorf("failed to delete directory: %w", err),
				map[string]any{"path": args.Path},
			)
		}
		logDelete(ctx, true, relativePath(rootUsed, resolvedPath))
		success = true
		return core.Output{"success": true}, nil
	}
	if err := os.Remove(resolvedPath); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, builtin.PermissionDenied(err, map[string]any{"path": args.Path})
		}
		return nil, builtin.Internal(fmt.Errorf("failed to delete file: %w", err), map[string]any{"path": args.Path})
	}
	logDelete(ctx, false, relativePath(rootUsed, resolvedPath))
	success = true
	return core.Output{"success": true}, nil
}

func logDelete(ctx context.Context, isDir bool, path string) {
	action := "Deleted file"
	if isDir {
		action = "Deleted directory"
	}
	logger.FromContext(ctx).Info(
		action,
		"tool_id", "cp__delete_file",
		"request_id", builtin.RequestIDFromContext(ctx),
		"path", path,
	)
}
