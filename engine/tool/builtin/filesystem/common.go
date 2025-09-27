package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/config"
)

const (
	sampleBinaryBytes      = 8 * 1024
	defaultMaxResults      = 1000
	defaultMaxFilesVisited = 10000
	defaultMaxFileBytes    = 1 << 20 // 1 MiB
	defaultMaxListEntries  = 10000
	defaultPageSize        = 200
)

var allowedControlBytes = map[byte]struct{}{
	'\n': {},
	'\r': {},
	'\t': {},
	'\f': {},
	'\b': {},
}

type limitsConfig struct {
	MaxResults      int
	MaxFilesVisited int
	MaxFileBytes    int64
	MaxListEntries  int
}

type toolConfig struct {
	Roots  []string
	Limits limitsConfig
}

func loadToolConfig(ctx context.Context) (toolConfig, error) {
	nativeCfg := config.DefaultNativeToolsConfig()
	if cfg := config.FromContext(ctx); cfg != nil {
		nativeCfg = cfg.Runtime.NativeTools
	}
	roots := make([]string, 0, 1+len(nativeCfg.AdditionalRoots))
	root, err := builtin.NormalizeRoot(nativeCfg.RootDir)
	if err != nil {
		return toolConfig{}, builtin.Internal(fmt.Errorf("failed to normalize native tools root: %w", err), nil)
	}
	roots = append(roots, root)
	for _, extra := range nativeCfg.AdditionalRoots {
		normalized, normErr := builtin.NormalizeRoot(extra)
		if normErr != nil {
			return toolConfig{}, builtin.Internal(
				fmt.Errorf("failed to normalize additional native tools root: %w", normErr),
				map[string]any{"root": extra},
			)
		}
		if !slices.Contains(roots, normalized) {
			roots = append(roots, normalized)
		}
	}
	if len(roots) == 0 {
		return toolConfig{}, builtin.Internal(errors.New("no native tools roots configured"), nil)
	}
	return toolConfig{
		Roots: roots,
		Limits: limitsConfig{
			MaxResults:      defaultMaxResults,
			MaxFilesVisited: defaultMaxFilesVisited,
			MaxFileBytes:    defaultMaxFileBytes,
			MaxListEntries:  defaultMaxListEntries,
		},
	}, nil
}

func decodeArgs[T any](payload map[string]any) (T, error) {
	var value T
	if payload == nil {
		return value, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return value, fmt.Errorf("failed to marshal args: %w", err)
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, fmt.Errorf("failed to unmarshal args: %w", err)
	}
	return value, nil
}

func resolvePath(cfg toolConfig, pathValue string) (string, string, error) {
	if strings.TrimSpace(pathValue) == "" {
		return "", "", builtin.InvalidArgument(errors.New("path must be provided"), map[string]any{"field": "path"})
	}
	var lastErr error
	for _, root := range cfg.Roots {
		if filepath.IsAbs(pathValue) {
			cleaned := filepath.Clean(pathValue)
			ensureErr := builtin.EnsureWithinRoot(root, cleaned)
			if ensureErr == nil {
				return cleaned, root, nil
			}
			lastErr = builtin.PermissionDenied(ensureErr, map[string]any{"path": pathValue, "root": root})
			continue
		}
		resolved, err := builtin.ResolvePath(root, pathValue)
		if err == nil {
			return resolved, root, nil
		}
		lastErr = err
	}
	details := map[string]any{"path": pathValue, "roots": cfg.Roots}
	if lastErr == nil {
		lastErr = errors.New("path rejected for all configured roots")
	}
	return "", "", builtin.PermissionDenied(lastErr, details)
}

func ensureParentsSafe(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return builtin.PermissionDenied(
			fmt.Errorf("failed to compute relative path: %w", err),
			map[string]any{"path": target},
		)
	}
	segments := strings.Split(rel, string(filepath.Separator))
	current := root
	for idx, segment := range segments[:len(segments)-1] {
		if segment == "" || segment == "." {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return builtin.Internal(
				fmt.Errorf("failed to inspect path component: %w", err),
				map[string]any{"component": current},
			)
		}
		if err := builtin.RejectSymlink(info); err != nil {
			return builtin.PermissionDenied(
				fmt.Errorf("path component is symlink: %s", current),
				map[string]any{"component": current, "index": idx},
			)
		}
	}
	return nil
}

func rejectSymlink(info fs.FileInfo, details map[string]any) error {
	if err := builtin.RejectSymlink(info); err != nil {
		return builtin.PermissionDenied(err, details)
	}
	return nil
}

func relativePath(root, candidate string) string {
	if rel := normalizedRelative(root, candidate); rel != "" {
		return rel
	}
	rootReal, rootErr := filepath.EvalSymlinks(root)
	candidateReal, candErr := filepath.EvalSymlinks(candidate)
	if rootErr == nil && candErr == nil {
		if rel := normalizedRelative(rootReal, candidateReal); rel != "" {
			return rel
		}
	}
	return candidate
}

func normalizedRelative(root, candidate string) string {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return ""
	}
	if rel == "." {
		return string(filepath.Separator)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return string(filepath.Separator) + rel
}

func fileMetadata(info fs.FileInfo) map[string]any {
	if info == nil {
		return nil
	}
	return map[string]any{
		"size":     info.Size(),
		"mode":     info.Mode().Perm(),
		"modified": info.ModTime().UTC().Format(time.RFC3339Nano),
	}
}

func readFileIntoBuffer(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer := make([]byte, limit+1)
	n, err := io.ReadFull(io.LimitReader(file, limit+1), buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buffer[:n], nil
}

func isBinaryContent(sample []byte) bool {
	if len(sample) == 0 {
		return false
	}
	nonPrintable := 0
	for _, b := range sample {
		if b == 0 {
			return true
		}
		if b < 0x20 || b > 0x7E {
			if _, allowed := allowedControlBytes[b]; !allowed {
				nonPrintable++
			}
		}
	}
	ratio := float64(nonPrintable) / float64(len(sample))
	return ratio > 0.30
}

func binarySample(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	sample := make([]byte, sampleBinaryBytes)
	n, err := io.ReadFull(io.LimitReader(file, sampleBinaryBytes), sample)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return sample[:n], nil
}

func enforceFileSizeLimit(info fs.FileInfo, limit int64) error {
	if info != nil && info.Size() > limit {
		details := map[string]any{
			"size":   info.Size(),
			"limit":  limit,
			"reason": "file too large",
		}
		return builtin.InvalidArgument(errors.New("file exceeds maximum size"), details)
	}
	return nil
}

type paginationParams struct {
	offset   int
	pageSize int
	limit    int
}

func newPagination(pageToken string, requested int, maxLimit int) (paginationParams, error) {
	offset := 0
	if pageToken != "" {
		parsed, err := strconv.Atoi(pageToken)
		if err != nil {
			details := map[string]any{"token": pageToken}
			return paginationParams{}, builtin.InvalidArgument(fmt.Errorf("invalid page token"), details)
		}
		if parsed < 0 {
			details := map[string]any{"token": pageToken}
			return paginationParams{}, builtin.InvalidArgument(errors.New("page token must be non-negative"), details)
		}
		offset = parsed
	}
	pageSize := clampPositive(requested, defaultPageSize, maxLimit)
	return paginationParams{offset: offset, pageSize: pageSize, limit: maxLimit}, nil
}

func clampPositive(value, fallback, maximum int) int {
	switch {
	case value <= 0:
		return fallback
	case value > maximum:
		return maximum
	default:
		return value
	}
}

func progressContext(ctx context.Context) error {
	if err := builtin.CheckContext(ctx); err != nil {
		return builtin.Internal(err, nil)
	}
	return nil
}

func visitLimit(current, limit int) error {
	if limit <= 0 {
		return nil
	}
	if current > limit {
		details := map[string]any{"limit": limit}
		return builtin.InvalidArgument(errors.New("traversal limit exceeded"), details)
	}
	return nil
}
