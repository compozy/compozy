package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

type ListDirArgs struct {
	Path         string `json:"path"`
	Pattern      string `json:"pattern,omitempty"`
	Recursive    bool   `json:"recursive,omitempty"`
	IncludeFiles *bool  `json:"include_files,omitempty"`
	IncludeDirs  *bool  `json:"include_dirs,omitempty"`
	PageSize     int    `json:"page_size,omitempty"`
	PageToken    string `json:"page_token,omitempty"`
}

var listDirInputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"path"},
	"properties": map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Directory path relative to the sandbox root to enumerate.",
		},
		"pattern": map[string]any{
			"type":        "string",
			"description": "Optional doublestar-compatible glob applied to paths relative to the requested directory.",
		},
		"recursive": map[string]any{
			"type":        "boolean",
			"description": "Whether to traverse the directory tree recursively.",
		},
		"include_files": map[string]any{
			"type":        "boolean",
			"description": "Include files in the response (defaults to true).",
		},
		"include_dirs": map[string]any{
			"type":        "boolean",
			"description": "Include directories in the response (defaults to true).",
		},
		"page_size": map[string]any{
			"type":        "integer",
			"description": "Number of entries to return in this page (defaults to 200, max 10,000).",
		},
		"page_token": map[string]any{
			"type":        "string",
			"description": "Continuation token from a previous response.",
		},
	},
}

var listDirOutputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"entries"},
	"properties": map[string]any{
		"entries": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"name", "path", "type", "modified"},
				"properties": map[string]any{
					"name":     map[string]any{"type": "string"},
					"path":     map[string]any{"type": "string"},
					"type":     map[string]any{"type": "string", "enum": []string{"file", "dir"}},
					"size":     map[string]any{"type": "integer"},
					"modified": map[string]any{"type": "string", "format": "date-time"},
				},
			},
		},
		"next_page_token": map[string]any{
			"type":        "string",
			"description": "Token to request the next page of results when present.",
		},
	},
}

func ListDirDefinition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            "cp__list_dir",
		Description:   "List directory contents inside the sandboxed filesystem.",
		InputSchema:   listDirInputSchema,
		OutputSchema:  listDirOutputSchema,
		ArgsPrototype: ListDirArgs{},
		Handler:       listDirHandler,
	}
}

func listDirHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	log := logger.FromContext(ctx)
	start := time.Now()
	var success bool
	defer func() {
		status := "failure"
		if success {
			status = "success"
		}
		builtin.RecordInvocation(
			ctx,
			"cp__list_dir",
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
	args, err := decodeArgs[ListDirArgs](payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	resolvedPath, err := resolvePath(cfg.Root, args.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, builtin.FileNotFound(err, map[string]any{"path": args.Path})
		}
		return nil, builtin.Internal(fmt.Errorf("failed to stat directory: %w", err), map[string]any{"path": args.Path})
	}
	if !info.IsDir() {
		return nil, builtin.InvalidArgument(errors.New("path is not a directory"), map[string]any{"path": args.Path})
	}
	pagination, err := newPagination(args.PageToken, args.PageSize, cfg.Limits.MaxListEntries)
	if err != nil {
		return nil, err
	}
	includeFiles := true
	if args.IncludeFiles != nil {
		includeFiles = *args.IncludeFiles
	}
	includeDirs := true
	if args.IncludeDirs != nil {
		includeDirs = *args.IncludeDirs
	}
	if !includeFiles && !includeDirs {
		return nil, builtin.InvalidArgument(
			errors.New("at least one of include_files or include_dirs must be true"),
			nil,
		)
	}
	entries, nextToken, err := traverseDirectory(ctx, cfg, resolvedPath, &args, pagination, includeFiles, includeDirs)
	if err != nil {
		return nil, err
	}
	output := core.Output{"entries": entries}
	if nextToken != "" {
		output["next_page_token"] = nextToken
	}
	logListDir(ctx, log, relativePath(cfg.Root, resolvedPath), len(entries), nextToken != "")
	success = true
	return output, nil
}

func logListDir(ctx context.Context, log logger.Logger, path string, count int, hasNext bool) {
	log.Info(
		"Listed directory",
		"tool_id", "cp__list_dir",
		"request_id", builtin.RequestIDFromContext(ctx),
		"path", path,
		"count", count,
		"next", hasNext,
	)
}

func traverseDirectory(
	ctx context.Context,
	cfg toolConfig,
	basePath string,
	args *ListDirArgs,
	pagination paginationParams,
	includeFiles bool,
	includeDirs bool,
) ([]map[string]any, string, error) {
	collector := &listCollector{
		ctx:          ctx,
		cfg:          cfg,
		base:         basePath,
		args:         args,
		pagination:   pagination,
		includeFiles: includeFiles,
		includeDirs:  includeDirs,
		results:      make([]map[string]any, 0, pagination.pageSize),
	}
	if err := collector.collect(); err != nil {
		return nil, "", err
	}
	return collector.results, collector.nextToken, nil
}

type listCollector struct {
	ctx          context.Context
	cfg          toolConfig
	base         string
	args         *ListDirArgs
	pagination   paginationParams
	includeFiles bool
	includeDirs  bool
	matched      int
	visited      int
	results      []map[string]any
	nextToken    string
}

func (c *listCollector) collect() error {
	queue := []string{c.base}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if err := progressContext(c.ctx); err != nil {
			return err
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			return builtin.PermissionDenied(err, map[string]any{"path": relativePath(c.cfg.Root, current)})
		}
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.Mode()&fs.ModeSymlink != 0 {
				continue
			}
			fullPath := filepath.Join(current, entry.Name())
			if info.IsDir() && c.args.Recursive {
				queue = append(queue, fullPath)
			}
			if err := c.process(fullPath, info); err != nil {
				return err
			}
			if c.nextToken != "" {
				return nil
			}
		}
	}
	return nil
}

func (c *listCollector) process(fullPath string, info fs.FileInfo) error {
	c.visited++
	if err := visitLimit(c.visited, c.cfg.Limits.MaxFilesVisited); err != nil {
		return err
	}
	if !c.shouldInclude(info.IsDir()) {
		return nil
	}
	candidate, err := c.relativeCandidate(fullPath)
	if err != nil {
		return err
	}
	if c.args.Pattern != "" {
		match, matchErr := doublestar.Match(c.args.Pattern, candidate)
		if matchErr != nil {
			return builtin.InvalidArgument(matchErr, map[string]any{"pattern": c.args.Pattern})
		}
		if !match {
			return nil
		}
	}
	c.matched++
	if c.matched <= c.pagination.offset {
		return nil
	}
	if len(c.results) < c.pagination.pageSize {
		c.results = append(c.results, c.entryPayload(fullPath, info))
		return nil
	}
	c.nextToken = fmt.Sprintf("%d", c.pagination.offset+len(c.results))
	return nil
}

func (c *listCollector) shouldInclude(isDir bool) bool {
	if isDir {
		return c.includeDirs
	}
	return c.includeFiles
}

func (c *listCollector) relativeCandidate(fullPath string) (string, error) {
	relative, err := filepath.Rel(c.base, fullPath)
	if err != nil {
		return "", builtin.Internal(err, map[string]any{"path": fullPath})
	}
	return filepath.ToSlash(relative), nil
}

func (c *listCollector) entryPayload(fullPath string, info fs.FileInfo) map[string]any {
	entryType := "file"
	size := info.Size()
	if info.IsDir() {
		entryType = "dir"
		size = 0
	}
	return map[string]any{
		"name":     info.Name(),
		"path":     relativePath(c.cfg.Root, fullPath),
		"type":     entryType,
		"size":     size,
		"modified": info.ModTime().UTC().Format(time.RFC3339Nano),
	}
}
