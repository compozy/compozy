package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

type scanCandidate struct {
	name    string
	path    string
	content []byte
	modTime time.Time
}

// Scan lists memory headers for a scope, sorted newest-first and capped at 200 files.
func (s *Store) Scan(ctx context.Context, scope memcontract.Scope) ([]memcontract.Header, error) {
	return s.scan(ctx, scope, maxScanEntries)
}

func (s *Store) scan(ctx context.Context, scope memcontract.Scope, limit int) (_ []memcontract.Header, err error) {
	if err := requireMemoryContext(ctx, "scan"); err != nil {
		return nil, err
	}
	dir, err := s.dirForScope(scope)
	if err != nil {
		return nil, err
	}

	directory, err := fileutil.OpenDirectory(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []memcontract.Header{}, nil
		}
		return nil, fmt.Errorf("memory: scan %q: %w", dir, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("memory: close scan directory %q: %w", dir, closeErr))
		}
	}()

	names, err := directory.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("memory: scan %q: %w", dir, err)
	}
	candidates, err := s.scanCandidates(ctx, scope, dir, directory, names)
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	capacity := len(candidates)
	if limit > 0 {
		capacity = min(capacity, limit)
	}
	headers := make([]memcontract.Header, 0, capacity)
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var header memcontract.Header
		if _, err := parseFrontmatter(candidate.content, &header); err != nil {
			s.warn("memory: skip malformed memory file", "scope", scope, "path", candidate.path, "error", err)
			continue
		}
		if err := header.Validate(); err != nil {
			s.warn("memory: skip invalid memory metadata", "scope", scope, "path", candidate.path, "error", err)
			continue
		}
		completedHeader, err := s.completeHeaderForScope(scope.Normalize(), header)
		if err != nil {
			s.warn(
				"memory: skip memory file with invalid scope metadata",
				"scope", scope,
				"path", candidate.path,
				"error", err,
			)
			continue
		}

		completedHeader.Filename = candidate.name
		completedHeader.FilePath = candidate.path
		completedHeader.ModTime = candidate.modTime
		headers = append(headers, completedHeader)
		if limit > 0 && len(headers) == limit {
			break
		}
	}

	return headers, nil
}

func (s *Store) scanCandidates(
	ctx context.Context,
	scope memcontract.Scope,
	dir string,
	directory *fileutil.Directory,
	names []string,
) ([]scanCandidate, error) {
	candidates := make([]scanCandidate, 0, len(names))
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if shouldSkipFile(name) {
			continue
		}
		content, info, err := directory.ReadRegularFile(name)
		if err != nil {
			s.warn(
				"memory: skip unreadable memory file",
				"scope", scope,
				"filename", name,
				"error", err,
			)
			continue
		}
		candidates = append(candidates, scanCandidate{
			name:    name,
			path:    filepath.Join(dir, name),
			content: content,
			modTime: info.ModTime(),
		})
	}
	return candidates, nil
}

// LoadIndex reads MEMORY.md for a scope and truncates it to the prompt-safe limits.
func (s *Store) LoadIndex(
	ctx context.Context,
	scope memcontract.Scope,
) (content string, truncated bool, err error) {
	if err := requireMemoryContext(ctx, "load index"); err != nil {
		return "", false, err
	}
	dir, err := s.dirForScope(scope)
	if err != nil {
		return "", false, err
	}

	headers, err := s.scan(ctx, scope.Normalize(), 0)
	if err != nil {
		return "", false, err
	}

	indexBytes, _, err := fileutil.ReadRegularFile(filepath.Join(dir, indexFilename))
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		if len(headers) == 0 {
			return "", false, nil
		}
		generated, generatedTruncated := truncateIndex(renderIndex(headers), s.maxIndexLines, s.maxIndexBytes)
		return generated, generatedTruncated, nil
	default:
		return "", false, fmt.Errorf("memory: load index %q: %w", filepath.Join(dir, indexFilename), err)
	}

	indexContent := string(indexBytes)
	if indexMatchesHeaders(indexContent, headers) {
		content, truncated = truncateIndex(indexContent, s.maxIndexLines, s.maxIndexBytes)
		if truncated {
			s.warn(
				"memory: truncated memory index",
				"scope", scope,
				"path", filepath.Join(dir, indexFilename),
				"max_lines", s.maxIndexLines,
				"max_bytes", s.maxIndexBytes,
			)
		}
		return content, truncated, nil
	}

	s.warn(
		"memory: synthesized prompt index from memory files",
		"scope", scope,
		"path", filepath.Join(dir, indexFilename),
	)
	generated, generatedTruncated := truncateIndex(renderIndex(headers), s.maxIndexLines, s.maxIndexBytes)
	return generated, generatedTruncated, nil
}
