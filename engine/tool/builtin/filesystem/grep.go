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
	defer recordGrepInvocation(ctx, start, &success)
	result, err := performGrep(ctx, payload)
	if err != nil {
		return nil, err
	}
	success = true
	return result, nil
}

// recordGrepInvocation captures cp__grep execution metrics for analytics
func recordGrepInvocation(ctx context.Context, start time.Time, success *bool) {
	status := builtin.StatusFailure
	if success != nil && *success {
		status = builtin.StatusSuccess
	}
	builtin.RecordInvocation(
		ctx,
		"cp__grep",
		builtin.RequestIDFromContext(ctx),
		status,
		time.Since(start),
		0,
		"",
	)
}

// performGrep decodes input, validates state, executes the search, and logs the outcome
func performGrep(ctx context.Context, payload map[string]any) (core.Output, error) {
	cfg, err := loadToolConfig(ctx)
	if err != nil {
		return nil, err
	}
	args, err := decodeArgs[GrepArgs](payload)
	if err != nil {
		return nil, builtin.InvalidArgument(err, nil)
	}
	if err := validateGrepArgs(args); err != nil {
		return nil, err
	}
	resolvedPath, rootUsed, info, err := resolveGrepTarget(cfg, args)
	if err != nil {
		return nil, err
	}
	pattern, err := compileGrepPattern(args)
	if err != nil {
		return nil, err
	}
	maxResults, maxFilesVisited, maxFileBytes := determineGrepLimits(args, cfg)
	matches, filesVisited, err := executeGrepSearch(
		ctx,
		resolvedPath,
		rootUsed,
		info.IsDir(),
		args,
		pattern,
		maxResults,
		maxFilesVisited,
		maxFileBytes,
	)
	if err != nil {
		return nil, err
	}
	logGrepCompletion(ctx, rootUsed, resolvedPath, len(matches), filesVisited)
	return core.Output{"matches": matches}, nil
}

// validateGrepArgs verifies required Grep parameters before executing
func validateGrepArgs(args GrepArgs) error {
	if strings.TrimSpace(args.Pattern) == "" {
		return builtin.InvalidArgument(
			errors.New("pattern must be provided"),
			map[string]any{"field": "pattern"},
		)
	}
	return nil
}

// resolveGrepTarget resolves the virtual path and performs safety checks
func resolveGrepTarget(
	cfg toolConfig,
	args GrepArgs,
) (string, string, fs.FileInfo, error) {
	resolvedPath, rootUsed, err := resolvePath(cfg, args.Path)
	if err != nil {
		return "", "", nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", nil, builtin.FileNotFound(err, map[string]any{"path": args.Path})
		}
		return "", "", nil, builtin.Internal(
			fmt.Errorf("failed to stat path: %w", err),
			map[string]any{"path": args.Path},
		)
	}
	if rejectErr := builtin.RejectSymlink(info); rejectErr != nil {
		return "", "", nil, builtin.PermissionDenied(rejectErr, map[string]any{"path": args.Path})
	}
	return resolvedPath, rootUsed, info, nil
}

// compileGrepPattern prepares the regular expression with any runtime modifiers
func compileGrepPattern(args GrepArgs) (*regexp.Regexp, error) {
	patternSource := args.Pattern
	if args.IgnoreCase {
		patternSource = "(?i)" + patternSource
	}
	re, err := regexp.Compile(patternSource)
	if err != nil {
		return nil, builtin.InvalidArgument(err, map[string]any{"pattern": args.Pattern})
	}
	return re, nil
}

// determineGrepLimits resolves the effective search limits using configuration overrides
func determineGrepLimits(args GrepArgs, cfg toolConfig) (int, int, int64) {
	maxResults := clampPositive(args.MaxResults, cfg.Limits.MaxResults, cfg.Limits.MaxResults)
	maxFilesVisited := clampPositive(args.MaxFilesVisited, cfg.Limits.MaxFilesVisited, cfg.Limits.MaxFilesVisited)
	maxFileBytes := cfg.Limits.MaxFileBytes
	if args.MaxFileBytes > 0 && int64(args.MaxFileBytes) < maxFileBytes {
		maxFileBytes = int64(args.MaxFileBytes)
	}
	return maxResults, maxFilesVisited, maxFileBytes
}

// logGrepCompletion emits structured logs summarizing the search results
func logGrepCompletion(
	ctx context.Context,
	rootUsed string,
	resolvedPath string,
	matchCount int,
	filesVisited int,
) {
	logger.FromContext(ctx).Info(
		"Grep completed",
		"tool_id", "cp__grep",
		"request_id", builtin.RequestIDFromContext(ctx),
		"path", relativePath(rootUsed, resolvedPath),
		"matches", matchCount,
		"filesVisited", filesVisited,
	)
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
	searcher := directorySearcher{
		ctx:             ctx,
		basePath:        basePath,
		root:            root,
		args:            args,
		re:              re,
		maxResults:      maxResults,
		maxFilesVisited: maxFilesVisited,
		maxFileBytes:    maxFileBytes,
		matches:         matches,
	}
	return searcher.walk()
}

// directorySearcher iteratively traverses directories during grep operations
type directorySearcher struct {
	ctx             context.Context
	basePath        string
	root            string
	args            GrepArgs
	re              *regexp.Regexp
	maxResults      int
	maxFilesVisited int
	maxFileBytes    int64
	matches         *[]map[string]any
}

// walk performs a breadth-first traversal applying the grep constraints
func (s *directorySearcher) walk() (int, error) {
	queue := []string{s.basePath}
	visited := 0
	for len(queue) > 0 {
		if len(*s.matches) >= s.maxResults {
			return visited, nil
		}
		current := queue[0]
		queue = queue[1:]
		if err := progressContext(s.ctx); err != nil {
			return visited, err
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			details := map[string]any{"path": relativePath(s.root, current)}
			return visited, builtin.PermissionDenied(err, details)
		}
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if err := s.processEntry(current, entry, &queue, &visited); err != nil {
				return visited, err
			}
			if len(*s.matches) >= s.maxResults {
				return visited, nil
			}
		}
	}
	return visited, nil
}

// processEntry processes a single directory entry inside the traversal queue
func (s *directorySearcher) processEntry(
	current string,
	entry fs.DirEntry,
	queue *[]string,
	visited *int,
) error {
	fullPath := filepath.Join(current, entry.Name())
	info, err := os.Lstat(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		details := map[string]any{"path": relativePath(s.root, fullPath)}
		return builtin.Internal(fmt.Errorf("failed to inspect path: %w", err), details)
	}
	if rejectErr := builtin.RejectSymlink(info); rejectErr != nil {
		return nil
	}
	if info.IsDir() {
		if s.args.Recursive {
			*queue = append(*queue, fullPath)
		}
		return nil
	}
	*visited++
	if err := visitLimit(*visited, s.maxFilesVisited); err != nil {
		return err
	}
	return searchFile(s.ctx, s.root, fullPath, s.re, s.maxResults, s.maxFileBytes, s.matches)
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
		return translateSearchStatError(err, rel)
	}
	if rejectErr := builtin.RejectSymlink(info); rejectErr != nil || shouldSkipNonRegular(info, maxFileBytes) {
		return nil
	}
	skipBinary, err := shouldSkipBinaryFile(path, rel)
	if err != nil {
		return err
	}
	if skipBinary {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return translateSearchOpenError(err, rel)
	}
	defer closeSearchFile(ctx, file, rel)
	return scanFileMatches(file, root, path, re, maxResults, maxFileBytes, matches)
}

// translateSearchStatError maps file stat errors to user-facing responses
func translateSearchStatError(err error, rel string) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return builtin.Internal(fmt.Errorf("failed to stat file: %w", err), map[string]any{"path": rel})
}

// shouldSkipNonRegular returns true when the file should be skipped for size or type reasons
func shouldSkipNonRegular(info fs.FileInfo, maxFileBytes int64) bool {
	if !info.Mode().IsRegular() {
		return true
	}
	return info.Size() > maxFileBytes
}

// shouldSkipBinaryFile inspects the file header and reports whether it is binary
func shouldSkipBinaryFile(path string, rel string) (bool, error) {
	sample, err := binarySample(path)
	if err != nil {
		return false, builtin.Internal(
			fmt.Errorf("failed to inspect file: %w", err),
			map[string]any{"path": rel},
		)
	}
	if isBinaryContent(sample) {
		return true, nil
	}
	return false, nil
}

// translateSearchOpenError converts file open errors into domain errors
func translateSearchOpenError(err error, rel string) error {
	if errors.Is(err, fs.ErrPermission) {
		return builtin.PermissionDenied(err, map[string]any{"path": rel})
	}
	return builtin.Internal(fmt.Errorf("failed to open file: %w", err), map[string]any{"path": rel})
}

// closeSearchFile closes the file descriptor while logging non-fatal errors
func closeSearchFile(ctx context.Context, file *os.File, rel string) {
	if cerr := file.Close(); cerr != nil {
		logger.FromContext(ctx).Debug("failed to close file", "path", rel, "error", cerr)
	}
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
	rel := relativePath(root, path)
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
				"file":   rel,
				"line":   lineNumber,
				"column": column,
				"text":   trimmed,
			}
			*matches = append(*matches, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return builtin.Internal(fmt.Errorf("failed to scan file: %w", err), map[string]any{"path": rel})
	}
	return nil
}
