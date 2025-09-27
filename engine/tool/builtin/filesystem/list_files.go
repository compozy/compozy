package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
)

type ListFilesArgs struct {
	Dir     string `json:"dir"`
	Exclude any    `json:"exclude,omitempty"`
}

var listFilesInputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"dir"},
	"properties": map[string]any{
		"dir": map[string]any{
			"type":        "string",
			"description": "Directory to enumerate relative to the sandbox root.",
		},
		"exclude": map[string]any{
			"description": "Optional glob pattern or array of patterns to remove from the result set.",
			"oneOf": []any{
				map[string]any{"type": "string"},
				map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
	},
}

var listFilesOutputSchema = &schema.Schema{
	"type":     "object",
	"required": []string{"files"},
	"properties": map[string]any{
		"files": map[string]any{
			"type":        "array",
			"description": "Sorted list of file names located in the requested directory.",
			"items":       map[string]any{"type": "string"},
		},
	},
}

func ListFilesDefinition() builtin.BuiltinDefinition {
	return builtin.BuiltinDefinition{
		ID:            "cp__list_files",
		Description:   "List files within a sandboxed directory applying optional exclusion globs.",
		InputSchema:   listFilesInputSchema,
		OutputSchema:  listFilesOutputSchema,
		ArgsPrototype: ListFilesArgs{},
		Handler:       listFilesHandler,
	}
}

func listFilesHandler(ctx context.Context, payload map[string]any) (core.Output, error) {
	start := time.Now()
	var success bool
	defer func() {
		status := builtin.StatusFailure
		if success {
			status = builtin.StatusSuccess
		}
		builtin.RecordInvocation(
			ctx,
			"cp__list_files",
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
	args, err := decodeArgs[ListFilesArgs](payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	if strings.TrimSpace(args.Dir) == "" {
		return nil, builtin.InvalidArgument(errors.New("dir must be provided"), map[string]any{"field": "dir"})
	}
	resolvedPath, rootUsed, err := resolvePath(cfg, args.Dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, builtin.FileNotFound(err, map[string]any{"dir": args.Dir})
		}
		return nil, builtin.Internal(fmt.Errorf("failed to stat directory: %w", err), map[string]any{"dir": args.Dir})
	}
	if !info.IsDir() {
		return nil, builtin.InvalidArgument(errors.New("path is not a directory"), map[string]any{"dir": args.Dir})
	}
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, builtin.PermissionDenied(err, map[string]any{"dir": args.Dir})
		}
		return nil, builtin.Internal(fmt.Errorf("failed to read directory: %w", err), map[string]any{"dir": args.Dir})
	}
	patterns, err := normalizeExcludePatterns(args.Exclude)
	if err != nil {
		return nil, builtin.InvalidArgument(err, map[string]any{"field": "exclude"})
	}
	compiled := compileExcludePatterns(patterns)
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsDir() {
			continue
		}
		name := entry.Name()
		if shouldExclude(name, compiled) {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	logListFiles(ctx, relativePath(rootUsed, resolvedPath), len(files))
	success = true
	return core.Output{"files": files}, nil
}

type compiledPattern struct {
	pattern string
	negated bool
}

func normalizeExcludePatterns(raw any) ([]string, error) {
	switch value := raw.(type) {
	case nil:
		return nil, nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, nil
		}
		return []string{trimmed}, nil
	case []any:
		patterns := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("exclude array must contain only strings")
			}
			trimmed := strings.TrimSpace(text)
			if trimmed == "" {
				continue
			}
			patterns = append(patterns, trimmed)
		}
		return patterns, nil
	case []string:
		patterns := make([]string, 0, len(value))
		for _, item := range value {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			patterns = append(patterns, trimmed)
		}
		return patterns, nil
	default:
		return nil, fmt.Errorf("exclude must be a string or array of strings")
	}
}

func compileExcludePatterns(patterns []string) []compiledPattern {
	compiled := make([]compiledPattern, 0, len(patterns))
	for _, raw := range patterns {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		negated := false
		if strings.HasPrefix(trimmed, "!") {
			negated = true
			trimmed = strings.TrimSpace(trimmed[1:])
		}
		expansions := expandBracePatterns(trimmed)
		if len(expansions) == 0 {
			expansions = []string{trimmed}
		}
		for _, expanded := range expansions {
			candidate := strings.TrimSpace(expanded)
			if candidate == "" {
				continue
			}
			compiled = append(compiled, compiledPattern{pattern: candidate, negated: negated})
		}
	}
	return compiled
}

func expandBracePatterns(pattern string) []string {
	start := strings.Index(pattern, "{")
	if start == -1 {
		return nil
	}
	end := findMatchingBrace(pattern, start)
	if end == -1 {
		return nil
	}
	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := splitBraceOptions(pattern[start+1 : end])
	results := make([]string, 0, len(options))
	for _, opt := range options {
		if strings.Contains(opt, "{") {
			nested := expandBracePatterns(opt)
			if len(nested) == 0 {
				nested = []string{opt}
			}
			for _, expanded := range nested {
				results = append(results, prefix+expanded+suffix)
			}
			continue
		}
		results = append(results, prefix+opt+suffix)
	}
	if len(results) == 0 {
		results = append(results, prefix+suffix)
	}
	return results
}

func findMatchingBrace(pattern string, start int) int {
	depth := 0
	for i := start; i < len(pattern); i++ {
		switch pattern[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitBraceOptions(body string) []string {
	if body == "" {
		return []string{""}
	}
	options := []string{}
	depth := 0
	start := 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				options = append(options, body[start:i])
				start = i + 1
			}
		}
	}
	options = append(options, body[start:])
	return options
}

func shouldExclude(name string, patterns []compiledPattern) bool {
	excluded := false
	hasPositive := false
	for _, pattern := range patterns {
		if !pattern.negated {
			hasPositive = true
		}
		matched, err := doublestar.Match(pattern.pattern, name)
		if err != nil {
			continue
		}
		if pattern.negated {
			if matched {
				excluded = false
				continue
			}
			if !hasPositive {
				excluded = true
			}
			continue
		}
		if matched {
			excluded = true
		}
	}
	return excluded
}

func logListFiles(ctx context.Context, dir string, count int) {
	logger.FromContext(ctx).Info(
		"Listed files",
		"tool_id", "cp__list_files",
		"request_id", builtin.RequestIDFromContext(ctx),
		"directory", dir,
		"count", count,
	)
}
