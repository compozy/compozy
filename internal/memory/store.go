package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	memoryrecall "github.com/compozy/agh/internal/memory/recall"
)

const (
	indexFilename     = "MEMORY.md"
	maxScanEntries    = 200
	defaultIndexLines = 200
	defaultIndexBytes = 25_600
	dirPerm           = 0o755
	filePerm          = 0o644
	memoryDirName     = "memory"
)

var (
	// ErrValidation marks memory input and metadata validation failures.
	ErrValidation = errors.New("memory: validation error")
)

// Store manages memory files for the global and workspace scopes.
type Store struct {
	globalDir                 string
	workspaceDir              string
	workspaceRoot             string
	agentName                 string
	agentTier                 memcontract.AgentTier
	agentWorkspaceID          string
	maxIndexLines             int
	maxIndexBytes             int
	maxFileLines              int
	maxFileBytes              int64
	logger                    *slog.Logger
	catalog                   *catalog
	mu                        *sync.Mutex
	decisionMu                *sync.Mutex
	mutationRevision          *storeMutationRevision
	recallSignals             recallSignalRecorderConfig
	recallRecorders           *recallRecorderRegistry
	decisionControllerFactory *decisionControllerFactoryState
}

var _ memcontract.Backend = (*Store)(nil)

type recallSignalRecorderConfig struct {
	queueCapacity  int
	workerRetryMax int
	metricsEnabled bool
}

type recallRecorderRegistry struct {
	mu        sync.Mutex
	recorders map[string]*memoryrecall.SignalRecorder
}

// ForWorkspace returns a clone of the store bound to the supplied workspace root.
func (s *Store) ForWorkspace(workspaceRoot string) *Store {
	clone := *s
	clone.workspaceRoot = canonicalWorkspaceRoot(workspaceRoot)
	clone.workspaceDir = workspaceMemoryDir(clone.workspaceRoot)
	return &clone
}

// ForAgent returns a clone of the store bound to one agent memory tier.
func (s *Store) ForAgent(workspaceID string, agentName string, tier memcontract.AgentTier) *Store {
	clone := *s
	clone.agentWorkspaceID = strings.TrimSpace(workspaceID)
	clone.agentName = strings.TrimSpace(agentName)
	clone.agentTier = tier.Normalize()
	return &clone
}

// List is the backend-aligned alias for Scan.
func (s *Store) List(scope memcontract.Scope) ([]memcontract.Header, error) {
	return s.Scan(scope)
}

// LoadPromptIndex is the backend-aligned alias for LoadIndex.
func (s *Store) LoadPromptIndex(scope memcontract.Scope) (content string, truncated bool, err error) {
	return s.LoadIndex(scope)
}

// EnsureDirs creates the configured memory directories when missing.
func (s *Store) EnsureDirs() error {
	dirs := []string{s.globalDir, s.workspaceDir}
	if s.agentConfigured() {
		agentDir, err := s.agentMemoryDir()
		if err != nil {
			return err
		}
		dirs = append(dirs, agentDir)
	}
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}

		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("memory: ensure directory %q: %w", dir, err)
		}
	}

	if strings.TrimSpace(s.globalDir) == "" {
		return wrapValidationError("ensure directory", "global", errors.New("global directory is required"))
	}

	return nil
}

// Read returns the raw file contents for a memory file in the requested scope.
func (s *Store) Read(scope memcontract.Scope, filename string) ([]byte, error) {
	path, err := s.pathFor(scope, filename)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("memory: read %q: %w", path, err)
	}

	return content, nil
}

// Exists reports whether a memory file exists in the requested scope.
func (s *Store) Exists(scope memcontract.Scope, filename string) (bool, error) {
	path, err := s.pathFor(scope, filename)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("memory: stat %q: %w", path, err)
	}

	return true, nil
}

// Write validates the memory frontmatter and persists the raw file contents atomically.
func (s *Store) Write(scope memcontract.Scope, filename string, content []byte) error {
	unlock := s.lockControllerDecisions()
	defer unlock()
	return s.writeRaw(context.Background(), scope, filename, content, true)
}

// Delete removes a memory file and strips any matching entry from the local MEMORY.md index.
func (s *Store) Delete(scope memcontract.Scope, filename string) error {
	unlock := s.lockControllerDecisions()
	defer unlock()
	return s.deleteRaw(context.Background(), scope, filename, true)
}

// Scan lists memory headers for a scope, sorted newest-first and capped at 200 files.
func (s *Store) Scan(scope memcontract.Scope) ([]memcontract.Header, error) {
	return s.scan(scope, maxScanEntries)
}

func (s *Store) scan(scope memcontract.Scope, limit int) ([]memcontract.Header, error) {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []memcontract.Header{}, nil
		}
		return nil, fmt.Errorf("memory: scan %q: %w", dir, err)
	}

	capacity := len(entries)
	if limit > 0 {
		capacity = min(capacity, limit)
	}
	headers := make([]memcontract.Header, 0, capacity)
	candidates := s.scanCandidates(scope, dir, entries)

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	for _, candidate := range candidates {
		content, err := os.ReadFile(candidate.path)
		if err != nil {
			s.warn("memory: skip unreadable memory file", "scope", scope, "path", candidate.path, "error", err)
			continue
		}

		var header memcontract.Header
		if _, err := parseFrontmatter(content, &header); err != nil {
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
				"scope",
				scope,
				"path",
				candidate.path,
				"error",
				err,
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

func (s *Store) scanCandidates(scope memcontract.Scope, dir string, entries []os.DirEntry) []scanCandidate {
	candidates := make([]scanCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || shouldSkipFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			s.warn(
				"memory: skip memory file with unreadable metadata",
				"scope", scope,
				"filename", entry.Name(),
				"error", err,
			)
			continue
		}

		candidates = append(candidates, scanCandidate{
			name:    entry.Name(),
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	return candidates
}

// LoadIndex reads MEMORY.md for a scope and truncates it to the prompt-safe limits.
func (s *Store) LoadIndex(scope memcontract.Scope) (content string, truncated bool, err error) {
	dir, err := s.dirForScope(scope)
	if err != nil {
		return "", false, err
	}

	headers, err := s.scan(scope.Normalize(), 0)
	if err != nil {
		return "", false, err
	}

	path := filepath.Join(dir, indexFilename)
	indexBytes, err := os.ReadFile(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		if len(headers) == 0 {
			return "", false, nil
		}
		generated, generatedTruncated := truncateIndex(renderIndex(headers), s.maxIndexLines, s.maxIndexBytes)
		return generated, generatedTruncated, nil
	default:
		return "", false, fmt.Errorf("memory: load index %q: %w", path, err)
	}

	indexContent := string(indexBytes)
	if indexMatchesHeaders(indexContent, headers) {
		content, truncated = truncateIndex(indexContent, s.maxIndexLines, s.maxIndexBytes)
		if truncated {
			s.warn(
				"memory: truncated memory index",
				"scope", scope,
				"path", path,
				"max_lines", s.maxIndexLines,
				"max_bytes", s.maxIndexBytes,
			)
		}
		return content, truncated, nil
	}

	s.warn("memory: synthesized prompt index from memory files", "scope", scope, "path", path)
	generated, generatedTruncated := truncateIndex(renderIndex(headers), s.maxIndexLines, s.maxIndexBytes)
	return generated, generatedTruncated, nil
}

// Search performs bounded lexical memory search across the visible scopes.
func (s *Store) Search(
	ctx context.Context,
	query string,
	opts memcontract.SearchOptions,
) ([]memcontract.SearchResult, error) {
	if ctx == nil {
		return nil, errors.New("memory: search context is required")
	}

	scope, workspaceRoot, workspaceID, err := s.normalizeScopeAndWorkspace(ctx, opts.Scope, opts.Workspace)
	if err != nil {
		return nil, err
	}
	if _, err := searchQueryTerms(query); err != nil {
		return nil, err
	}
	limit := clampSearchLimit(opts.Limit)

	if err := s.ensureCatalogReady(ctx, scope, workspaceRoot, workspaceID); err != nil {
		return nil, err
	}
	if s.catalog != nil {
		results, err := s.catalog.search(ctx, query, scope, workspaceID, limit)
		if err != nil {
			return nil, err
		}
		if err := s.logCatalogEvent(
			ctx,
			memcontract.OperationRecord{
				Operation: memcontract.OperationSearch,
				Scope:     operationRecordScope(scope, workspaceID),
				Workspace: workspaceID,
				Summary:   fmt.Sprintf("query=%q results=%d", strings.TrimSpace(query), len(results)),
			},
		); err != nil {
			s.warn("memory: record search event failed", "error", err)
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	docs, err := s.collectSearchDocuments(scope, workspaceRoot, workspaceID)
	if err != nil {
		return nil, err
	}
	results, err := fallbackSearchDocuments(query, docs, limit)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// Reindex rebuilds the derived catalog from the Markdown source of truth.
func (s *Store) Reindex(ctx context.Context, opts memcontract.ReindexOptions) (memcontract.ReindexResult, error) {
	if ctx == nil {
		return memcontract.ReindexResult{}, errors.New("memory: reindex context is required")
	}

	scope, workspaceRoot, workspaceID, err := s.normalizeScopeAndWorkspace(ctx, opts.Scope, opts.Workspace)
	if err != nil {
		return memcontract.ReindexResult{}, err
	}

	indexed, err := s.reindexScopes(ctx, scope, workspaceRoot, workspaceID)
	if err != nil {
		return memcontract.ReindexResult{}, err
	}
	s.recordCommittedMutation()
	completedAt := time.Now().UTC()
	if err := s.logCatalogEvent(
		ctx,
		memcontract.OperationRecord{
			Operation: memcontract.OperationReindex,
			Scope:     operationRecordScope(scope, workspaceID),
			Workspace: workspaceID,
			Summary: fmt.Sprintf(
				"scope=%s workspace=%s indexed=%d",
				string(scope.Normalize()),
				workspaceID,
				indexed,
			),
		},
	); err != nil {
		s.warn("memory: record reindex event failed", "error", err)
	}
	return memcontract.ReindexResult{
		IndexedFiles: indexed,
		Scope:        scope.Normalize(),
		Workspace:    workspaceID,
		CompletedAt:  completedAt,
	}, nil
}

// History returns durable memory operation history ordered newest-first.
func (s *Store) History(
	ctx context.Context,
	query memcontract.OperationHistoryQuery,
) ([]memcontract.OperationRecord, error) {
	if ctx == nil {
		return nil, errors.New("memory: history context is required")
	}
	if s.catalog == nil {
		return []memcontract.OperationRecord{}, nil
	}
	normalized := query
	scope, _, workspaceID, err := s.normalizeScopeAndWorkspace(ctx, query.Scope, query.Workspace)
	if err != nil {
		return nil, err
	}
	normalized.Scope = scope
	normalized.Workspace = workspaceID
	normalized.Operation = query.Operation.Normalize()
	return s.catalog.listOperations(ctx, normalized)
}

func operationRecordScope(scope memcontract.Scope, workspaceID string) memcontract.Scope {
	normalized := scope.Normalize()
	if normalized == "" && strings.TrimSpace(workspaceID) != "" {
		return memcontract.ScopeWorkspace
	}
	return normalized
}
