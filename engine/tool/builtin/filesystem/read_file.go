package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

type ReadFileArgs struct {
	Path string `json:"path"`
}

var readFileInputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"path"},
	"properties": map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Path to the file to read relative to the configured sandbox root.",
		},
	},
}

var readFileOutputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"content", "metadata"},
	"properties": map[string]any{
		"content": map[string]any{
			"type":        "string",
			"description": "UTF-8 decoded contents of the requested file.",
		},
		"metadata": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
				"size": map[string]any{"type": "integer"},
				"mode": map[string]any{"type": "integer"},
				"modified": map[string]any{
					"type":   "string",
					"format": "date-time",
				},
			},
		},
	},
}

func ReadFileDefinition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            "cp__read_file",
		Description:   "Read UTF-8 text content from the sandboxed filesystem.",
		InputSchema:   readFileInputSchema,
		OutputSchema:  readFileOutputSchema,
		ArgsPrototype: ReadFileArgs{},
		Handler:       readFileHandler,
	}
}

func readFileHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	var (
		respBytes int
		success   bool
	)
	defer recordReadFileInvocation(ctx, start, &success, &respBytes)
	output, bytesRead, err := performReadFile(ctx, payload)
	if err != nil {
		return nil, err
	}
	respBytes = bytesRead
	success = true
	return output, nil
}

// recordReadFileInvocation tracks runtime metrics for cp__read_file
func recordReadFileInvocation(
	ctx context.Context,
	start time.Time,
	success *bool,
	respBytes *int,
) {
	status := builtin.StatusFailure
	if success != nil && *success {
		status = builtin.StatusSuccess
	}
	bytesWritten := 0
	if respBytes != nil {
		bytesWritten = *respBytes
	}
	builtin.RecordInvocation(
		ctx,
		"cp__read_file",
		builtin.RequestIDFromContext(ctx),
		status,
		time.Since(start),
		bytesWritten,
		"",
	)
}

// performReadFile validates the request, reads file content, and emits metadata
func performReadFile(ctx context.Context, payload map[string]any) (core.Output, int, error) {
	cfg, err := loadToolConfig(ctx)
	if err != nil {
		return nil, 0, err
	}
	args, err := decodeArgs[ReadFileArgs](payload)
	if err != nil {
		return nil, 0, builtin.InvalidArgument(err, nil)
	}
	resolvedPath, rootUsed, info, err := resolveReadFileTarget(cfg, args.Path)
	if err != nil {
		return nil, 0, err
	}
	if err := enforceFileSizeLimit(info, cfg.Limits.MaxFileBytes); err != nil {
		return nil, 0, err
	}
	data, err := readTextContent(resolvedPath, args.Path, cfg.Limits.MaxFileBytes)
	if err != nil {
		return nil, 0, err
	}
	metadata := fileMetadata(info)
	metadata["path"] = relativePath(rootUsed, resolvedPath)
	logReadFile(ctx, metadata["path"], metadata["size"])
	return core.Output{
		"content":  string(data),
		"metadata": metadata,
	}, len(data), nil
}

// resolveReadFileTarget resolves and validates the requested file path
func resolveReadFileTarget(
	cfg toolConfig,
	virtualPath string,
) (string, string, fs.FileInfo, error) {
	resolvedPath, rootUsed, err := resolvePath(cfg, virtualPath)
	if err != nil {
		return "", "", nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", nil, builtin.FileNotFound(err, map[string]any{"path": virtualPath})
		}
		return "", "", nil, builtin.Internal(
			fmt.Errorf("failed to stat file: %w", err),
			map[string]any{"path": virtualPath},
		)
	}
	if err := rejectSymlink(info, map[string]any{"path": virtualPath}); err != nil {
		return "", "", nil, err
	}
	if info.IsDir() {
		return "", "", nil, builtin.InvalidArgument(
			errors.New("path points to a directory"),
			map[string]any{"path": virtualPath},
		)
	}
	return resolvedPath, rootUsed, info, nil
}

// readTextContent reads the file while enforcing size and encoding constraints
func readTextContent(
	resolvedPath string,
	virtualPath string,
	maxFileBytes int64,
) ([]byte, error) {
	sample, err := binarySample(resolvedPath)
	if err != nil {
		return nil, builtin.Internal(
			fmt.Errorf("failed to inspect file: %w", err),
			map[string]any{"path": virtualPath},
		)
	}
	if isBinaryContent(sample) {
		return nil, builtin.InvalidArgument(
			errors.New("binary files are not supported"),
			map[string]any{"path": virtualPath},
		)
	}
	data, err := readFileIntoBuffer(resolvedPath, maxFileBytes)
	if err != nil {
		return nil, builtin.Internal(
			fmt.Errorf("failed to read file: %w", err),
			map[string]any{"path": virtualPath},
		)
	}
	if int64(len(data)) > maxFileBytes {
		return nil, builtin.InvalidArgument(
			errors.New("file exceeds maximum size"),
			map[string]any{"path": virtualPath, "limit": maxFileBytes},
		)
	}
	if !utf8.Valid(data) {
		return nil, builtin.InvalidArgument(
			errors.New("file is not valid UTF-8"),
			map[string]any{"path": virtualPath},
		)
	}
	return data, nil
}

func logReadFile(ctx context.Context, path any, size any) {
	logger.FromContext(ctx).Info(
		"Read file",
		"tool_id", "cp__read_file",
		"request_id", builtin.RequestIDFromContext(ctx),
		"path", path,
		"size", size,
	)
}
