package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

type WriteFileArgs struct {
	Path    string  `json:"path"`
	Content string  `json:"content"`
	Append  bool    `json:"append,omitempty"`
	Mode    *uint32 `json:"mode,omitempty"`
}

var writeFileInputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"path", "content"},
	"properties": map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Path to the file to write relative to the configured sandbox root.",
		},
		"content": map[string]any{
			"type":        "string",
			"description": "UTF-8 content to write to the file.",
		},
		"append": map[string]any{
			"type":        "boolean",
			"description": "Append to the existing file instead of truncating it.",
		},
		"mode": map[string]any{
			"type":        "integer",
			"description": "Optional POSIX file mode (e.g., 420 for 0644).",
		},
	},
}

var writeFileOutputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"success", "metadata"},
	"properties": map[string]any{
		"success": map[string]any{"type": "boolean"},
		"metadata": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string"},
				"size":     map[string]any{"type": "integer"},
				"mode":     map[string]any{"type": "integer"},
				"modified": map[string]any{"type": "string", "format": "date-time"},
			},
		},
	},
}

const defaultFileMode = 0o644

func WriteFileDefinition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            "cp__write_file",
		Description:   "Write UTF-8 content to a file inside the sandboxed filesystem.",
		InputSchema:   writeFileInputSchema,
		OutputSchema:  writeFileOutputSchema,
		ArgsPrototype: WriteFileArgs{},
		Handler:       writeFileHandler,
	}
}

func writeFileHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	var success bool
	defer recordWriteFileInvocation(ctx, start, &success)
	output, err := performWriteFile(ctx, payload)
	if err != nil {
		return nil, err
	}
	success = true
	return output, nil
}

// recordWriteFileInvocation tracks runtime metrics for cp__write_file usage
func recordWriteFileInvocation(ctx context.Context, start time.Time, success *bool) {
	status := builtin.StatusFailure
	if success != nil && *success {
		status = builtin.StatusSuccess
	}
	builtin.RecordInvocation(
		ctx,
		"cp__write_file",
		builtin.RequestIDFromContext(ctx),
		status,
		time.Since(start),
		0,
		"",
	)
}

// performWriteFile handles validation, disk writes, and logging
func performWriteFile(ctx context.Context, payload map[string]any) (core.Output, error) {
	cfg, err := loadToolConfig(ctx)
	if err != nil {
		return nil, err
	}
	args, err := decodeArgs[WriteFileArgs](payload)
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
	if err := ensureDirectory(resolvedPath); err != nil {
		return nil, err
	}
	if err := preventSymlinkTarget(resolvedPath, args.Path); err != nil {
		return nil, err
	}
	file, err := openWritable(resolvedPath, args)
	if err != nil {
		return nil, err
	}
	defer closeWrittenFile(ctx, file, args.Path)
	if err := validateWriteSize(args.Content, cfg.Limits.MaxFileBytes); err != nil {
		return nil, err
	}
	if err := writeContent(file, []byte(args.Content), cfg.Limits.MaxFileBytes); err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, builtin.Internal(
			fmt.Errorf("failed to stat written file: %w", err),
			map[string]any{"path": args.Path},
		)
	}
	metadata := fileMetadata(stat)
	metadata["path"] = relativePath(rootUsed, resolvedPath)
	logWriteFile(ctx, metadata["path"], len(args.Content), args.Append)
	return core.Output{
		"success":  true,
		"metadata": metadata,
	}, nil
}

// validateWriteSize ensures the request payload respects configured limits
func validateWriteSize(content string, limit int64) error {
	if int64(len(content)) > limit {
		return builtin.InvalidArgument(
			errors.New("content exceeds maximum size"),
			map[string]any{"limit": limit},
		)
	}
	return nil
}

// logWriteFile emits structured logs summarizing the write operation
func logWriteFile(ctx context.Context, path any, bytes int, appendMode bool) {
	logger.FromContext(ctx).Info(
		"Wrote file",
		"tool_id", "cp__write_file",
		"request_id", builtin.RequestIDFromContext(ctx),
		"path", path,
		"bytes", bytes,
		"append", appendMode,
	)
}

func ensureDirectory(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return builtin.Internal(
			fmt.Errorf("failed to create parent directories: %w", err),
			map[string]any{"path": path},
		)
	}
	return nil
}

func openWritable(path string, args WriteFileArgs) (*os.File, error) {
	flag := os.O_WRONLY | os.O_CREATE
	if args.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	mode := os.FileMode(defaultFileMode)
	if args.Mode != nil {
		mode = os.FileMode(*args.Mode)
	}
	file, err := os.OpenFile(path, flag, mode)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, builtin.PermissionDenied(err, map[string]any{"path": args.Path})
		}
		return nil, builtin.Internal(fmt.Errorf("failed to open file: %w", err), map[string]any{"path": args.Path})
	}
	return file, nil
}

func closeWrittenFile(ctx context.Context, file *os.File, virtualPath string) {
	if err := file.Close(); err != nil {
		logger.FromContext(ctx).Error(
			"failed to close written file",
			"tool_id",
			"cp__write_file",
			"path",
			virtualPath,
			"error",
			err,
		)
	}
}

func preventSymlinkTarget(path string, virtual string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return builtin.Internal(fmt.Errorf("failed to stat path: %w", err), map[string]any{"path": virtual})
	}
	return rejectSymlink(info, map[string]any{"path": virtual})
}

func writeContent(file *os.File, content []byte, limit int64) error {
	if int64(len(content)) > limit {
		return builtin.InvalidArgument(errors.New("content exceeds maximum size"), map[string]any{"limit": limit})
	}
	written, err := file.Write(content)
	if err != nil {
		return builtin.Internal(fmt.Errorf("failed to write file: %w", err), nil)
	}
	if written != len(content) {
		return builtin.Internal(
			errors.New("short write"),
			map[string]any{"expected": len(content), "written": written},
		)
	}
	if err := file.Sync(); err != nil {
		return builtin.Internal(fmt.Errorf("failed to sync file: %w", err), nil)
	}
	return nil
}
