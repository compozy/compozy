package filesystem

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

type GrepArgs struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	Recursive       bool   `json:"recursive,omitempty"`
	IgnoreCase      bool   `json:"ignore_case,omitempty"`
	MaxResults      int    `json:"max_results,omitempty"`
	MaxFilesVisited int    `json:"max_files_visited,omitempty"`
	MaxFileBytes    int    `json:"max_file_bytes,omitempty"`
}

var grepInputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"pattern", "path"},
	"properties": map[string]any{
		"pattern": map[string]any{
			"type":        "string",
			"description": "Regular expression used to match file contents.",
		},
		"path": map[string]any{
			"type":        "string",
			"description": "File or directory path relative to the sandbox root.",
		},
		"recursive": map[string]any{
			"type":        "boolean",
			"description": "Traverse directories recursively when the path is a directory.",
		},
		"ignore_case": map[string]any{
			"type":        "boolean",
			"description": "Perform case-insensitive matching.",
		},
		"max_results": map[string]any{
			"type":        "integer",
			"description": "Upper bound on matches to return (defaults to 1,000).",
		},
		"max_files_visited": map[string]any{
			"type":        "integer",
			"description": "Upper bound on files inspected during the search (defaults to 10,000).",
		},
		"max_file_bytes": map[string]any{
			"type":        "integer",
			"description": "Skip files larger than this many bytes (default 1 MiB).",
		},
	},
}

var grepOutputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"matches"},
	"properties": map[string]any{
		"matches": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"file", "line", "column", "text"},
				"properties": map[string]any{
					"file":   map[string]any{"type": "string"},
					"line":   map[string]any{"type": "integer"},
					"column": map[string]any{"type": "integer"},
					"text":   map[string]any{"type": "string"},
				},
			},
		},
	},
}

func GrepDefinition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            "cp__grep",
		Description:   "Search for a regular expression across files in the sandboxed filesystem.",
		InputSchema:   grepInputSchema,
		OutputSchema:  grepOutputSchema,
		ArgsPrototype: GrepArgs{},
		Handler:       grepHandler,
	}
}

func grepHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	var success bool
	defer func() {
		status := builtin.StatusFailure
		if success {
			status = builtin.StatusSuccess
		}
		builtin.RecordInvocation(ctx, "cp__grep", builtin.RequestIDFromContext(ctx), status, time.Since(start), 0, "")
	}()
	cfg, err := loadToolConfig(ctx)
	if err != nil {
		return nil, err
	}
	args, err := decodeArgs[GrepArgs](payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return nil, builtin.InvalidArgument(errors.New("pattern must be provided"), map[string]any{"field": "pattern"})
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
		return nil, builtin.Internal(fmt.Errorf("failed to stat path: %w", err), map[string]any{"path": args.Path})
	}
	if rejectErr := builtin.RejectSymlink(info); rejectErr != nil {
		return nil, builtin.PermissionDenied(rejectErr, map[string]any{"path": args.Path})
	}
	patternSource := args.Pattern
	if args.IgnoreCase {
		patternSource = "(?i)" + patternSource
	}
	re, err := regexp.Compile(patternSource)
	if err != nil {
		return nil, builtin.InvalidArgument(err, map[string]any{"pattern": args.Pattern})
	}
	maxResults := clampPositive(args.MaxResults, cfg.Limits.MaxResults, cfg.Limits.MaxResults)
	maxFilesVisited := clampPositive(args.MaxFilesVisited, cfg.Limits.MaxFilesVisited, cfg.Limits.MaxFilesVisited)
	maxFileBytes := cfg.Limits.MaxFileBytes
	if args.MaxFileBytes > 0 && int64(args.MaxFileBytes) < maxFileBytes {
		maxFileBytes = int64(args.MaxFileBytes)
	}
	matches, filesVisited, err := executeGrepSearch(
		ctx,
		resolvedPath,
		rootUsed,
		info.IsDir(),
		args,
		re,
		maxResults,
		maxFilesVisited,
		maxFileBytes,
	)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Info(
		"Grep completed",
		"tool_id",
		"cp__grep",
		"request_id",
		builtin.RequestIDFromContext(ctx),
		"path",
		relativePath(rootUsed, resolvedPath),
		"matches",
		len(matches),
		"filesVisited",
		filesVisited,
	)
	success = true
	return core.Output{"matches": matches}, nil
}

func executeGrepSearch(
	ctx context.Context,
	basePath string,
	root string,
	isDir bool,
	args GrepArgs,
	re *regexp.Regexp,
	maxResults int,
	maxFilesVisited int,
	maxFileBytes int64,
) ([]map[string]any, int, error) {
	matches := make([]map[string]any, 0, maxResults)
	if isDir {
		visited, err := searchDirectory(
			ctx,
			basePath,
			root,
			args,
			re,
			maxResults,
			maxFilesVisited,
			maxFileBytes,
			&matches,
		)
		if err != nil {
			return nil, 0, err
		}
		return matches, visited, nil
	}
	visited := 1
	if err := visitLimit(visited, maxFilesVisited); err != nil {
		return nil, 0, err
	}
	if err := searchFile(ctx, root, basePath, re, maxResults, maxFileBytes, &matches); err != nil {
		return nil, 0, err
	}
	return matches, visited, nil
}

func searchDirectory(
	ctx context.Context,
	basePath string,
	root string,
	args GrepArgs,
	re *regexp.Regexp,
	maxResults int,
	maxFilesVisited int,
	maxFileBytes int64,
	matches *[]map[string]any,
) (int, error) {
	queue := []string{basePath}
	visited := 0
	for len(queue) > 0 {
		if len(*matches) >= maxResults {
			return visited, nil
		}
		current := queue[0]
		queue = queue[1:]
		if err := progressContext(ctx); err != nil {
			return visited, err
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			details := map[string]any{"path": relativePath(root, current)}
			return visited, builtin.PermissionDenied(err, details)
		}
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			fullPath := filepath.Join(current, entry.Name())
			info, err := os.Lstat(fullPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				details := map[string]any{"path": relativePath(root, fullPath)}
				return visited, builtin.Internal(fmt.Errorf("failed to inspect path: %w", err), details)
			}
			if rejectErr := builtin.RejectSymlink(info); rejectErr != nil {
				continue
			}
			if info.IsDir() {
				if args.Recursive {
					queue = append(queue, fullPath)
				}
				continue
			}
			visited++
			if err := visitLimit(visited, maxFilesVisited); err != nil {
				return visited, err
			}
			if err := searchFile(ctx, root, fullPath, re, maxResults, maxFileBytes, matches); err != nil {
				return visited, err
			}
			if len(*matches) >= maxResults {
				return visited, nil
			}
		}
	}
	return visited, nil
}

func searchFile(
	ctx context.Context,
	root string,
	path string,
	re *regexp.Regexp,
	maxResults int,
	maxFileBytes int64,
	matches *[]map[string]any,
) error {
	if len(*matches) >= maxResults {
		return nil
	}
	if err := progressContext(ctx); err != nil {
		return err
	}
	rel := relativePath(root, path)
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return builtin.Internal(fmt.Errorf("failed to stat file: %w", err), map[string]any{"path": rel})
	}
	if rejectErr := builtin.RejectSymlink(info); rejectErr != nil {
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if info.Size() > maxFileBytes {
		return nil
	}
	sample, err := binarySample(path)
	if err != nil {
		return builtin.Internal(fmt.Errorf("failed to inspect file: %w", err), map[string]any{"path": rel})
	}
	if isBinaryContent(sample) {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return builtin.PermissionDenied(err, map[string]any{"path": rel})
		}
		return builtin.Internal(fmt.Errorf("failed to open file: %w", err), map[string]any{"path": rel})
	}
	defer file.Close()
	return scanFileMatches(file, root, path, re, maxResults, maxFileBytes, matches)
}

func scanFileMatches(
	file *os.File,
	root string,
	path string,
	re *regexp.Regexp,
	maxResults int,
	maxFileBytes int64,
	matches *[]map[string]any,
) error {
	reader := io.LimitReader(file, maxFileBytes+1)
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 0, 64*1024)
	maxBuf := int(maxFileBytes) + 1
	if maxBuf > 0 {
		scanner.Buffer(buffer, maxBuf)
	}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		indices := re.FindAllStringIndex(line, -1)
		for _, loc := range indices {
			if len(*matches) >= maxResults {
				return nil
			}
			column := loc[0] + 1
			trimmed := strings.TrimSpace(line)
			entry := map[string]any{
				"file":   relativePath(root, path),
				"line":   lineNumber,
				"column": column,
				"text":   trimmed,
			}
			*matches = append(*matches, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return builtin.Internal(fmt.Errorf("failed to scan file: %w", err), map[string]any{"path": path})
	}
	return nil
}
