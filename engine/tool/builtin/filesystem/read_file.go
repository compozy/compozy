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
	log := logger.FromContext(ctx)
	start := time.Now()
	var respBytes int
	var success bool
	defer func() {
		status := builtin.StatusFailure
		if success {
			status = builtin.StatusSuccess
		}
		builtin.RecordInvocation(
			ctx,
			"cp__read_file",
			builtin.RequestIDFromContext(ctx),
			status,
			time.Since(start),
			respBytes,
			"",
		)
	}()
	cfg, err := loadToolConfig(ctx)
	if err != nil {
		return nil, err
	}
	args, err := decodeArgs[ReadFileArgs](payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	resolvedPath, rootUsed, err := resolvePath(cfg, args.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, builtin.FileNotFound(err, map[string]any{"path": args.Path})
		}
		return nil, builtin.Internal(fmt.Errorf("failed to stat file: %w", err), map[string]any{"path": args.Path})
	}
	if err := rejectSymlink(info, map[string]any{"path": args.Path}); err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, builtin.InvalidArgument(errors.New("path points to a directory"), map[string]any{"path": args.Path})
	}
	if err := enforceFileSizeLimit(info, cfg.Limits.MaxFileBytes); err != nil {
		return nil, err
	}
	sample, err := binarySample(resolvedPath)
	if err != nil {
		return nil, builtin.Internal(fmt.Errorf("failed to inspect file: %w", err), map[string]any{"path": args.Path})
	}
	if isBinaryContent(sample) {
		return nil, builtin.InvalidArgument(
			errors.New("binary files are not supported"),
			map[string]any{"path": args.Path},
		)
	}
	data, err := readFileIntoBuffer(resolvedPath, cfg.Limits.MaxFileBytes)
	if err != nil {
		return nil, builtin.Internal(fmt.Errorf("failed to read file: %w", err), map[string]any{"path": args.Path})
	}
	if int64(len(data)) > cfg.Limits.MaxFileBytes {
		return nil, builtin.InvalidArgument(
			errors.New("file exceeds maximum size"),
			map[string]any{"path": args.Path, "limit": cfg.Limits.MaxFileBytes},
		)
	}
	if !utf8.Valid(data) {
		return nil, builtin.InvalidArgument(errors.New("file is not valid UTF-8"), map[string]any{"path": args.Path})
	}
	metadata := fileMetadata(info)
	metadata["path"] = relativePath(rootUsed, resolvedPath)
	logReadFile(ctx, log, metadata["path"], metadata["size"])
	respBytes = len(data)
	success = true
	return core.Output{
		"content":  string(data),
		"metadata": metadata,
	}, nil
}

func logReadFile(ctx context.Context, log logger.Logger, path any, size any) {
	log.Info(
		"Read file",
		"tool_id", "cp__read_file",
		"request_id", builtin.RequestIDFromContext(ctx),
		"path", path,
		"size", size,
	)
}
