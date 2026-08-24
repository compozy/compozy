package memory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
)

const (
	indexFilename     = "MEMORY.md"
	maxScanEntries    = 200
	defaultIndexLines = 200
	defaultIndexBytes = 25_600
	dirPerm           = 0o700
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
	profileID                 string
	profileMaintenancePending *atomic.Bool
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
	recallSignalLifecycle     context.Context
	recallRecorders           *recallRecorderRegistry
	decisionControllerFactory *decisionControllerFactoryState
}

var _ memcontract.Backend = (*Store)(nil)

type recallSignalRecorderConfig struct {
	queueCapacity  int
	workerRetryMax int
}

// ForWorkspace returns a clone of the store bound to the supplied workspace root.
func (s *Store) ForWorkspace(workspaceRoot string) *Store {
	clone := *s
	clone.workspaceRoot = canonicalWorkspaceRoot(workspaceRoot)
	clone.workspaceDir = workspaceMemoryDir(clone.workspaceRoot)
	return &clone
}

// ForProfile returns a clone bound to one durable profile owner and directory.
func (s *Store) ForProfile(profileID string, profileMemoryDir string) *Store {
	clone := *s
	clone.profileID = strings.TrimSpace(profileID)
	clone.globalDir = cleanDirPath(profileMemoryDir)
	clone.workspaceDir = ""
	clone.workspaceRoot = ""
	clone.agentName = ""
	clone.agentTier = ""
	clone.agentWorkspaceID = ""
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
func (s *Store) List(ctx context.Context, scope memcontract.Scope) ([]memcontract.Header, error) {
	return s.Scan(ctx, scope)
}

// LoadPromptIndex is the backend-aligned alias for LoadIndex.
func (s *Store) LoadPromptIndex(
	ctx context.Context,
	scope memcontract.Scope,
) (content string, truncated bool, err error) {
	return s.LoadIndex(ctx, scope)
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

		directory, err := fileutil.OpenOrCreateDirectory(dir, dirPerm)
		if err != nil {
			return fmt.Errorf("memory: ensure directory %q: %w", dir, err)
		}
		if err := directory.Close(); err != nil {
			return fmt.Errorf("memory: close ensured directory %q: %w", dir, err)
		}
	}

	if strings.TrimSpace(s.globalDir) == "" {
		return wrapValidationError("ensure directory", "global", errors.New("global directory is required"))
	}

	return nil
}

// Read returns the raw file contents for a memory file in the requested scope.
func (s *Store) Read(ctx context.Context, scope memcontract.Scope, filename string) ([]byte, error) {
	if err := requireMemoryContext(ctx, "read"); err != nil {
		return nil, err
	}
	path, err := s.pathFor(scope, filename)
	if err != nil {
		return nil, err
	}

	content, _, err := fileutil.ReadRegularFile(path)
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

	file, err := fileutil.OpenRegularFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("memory: stat %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return false, fmt.Errorf("memory: close stat file %q: %w", path, err)
	}

	return true, nil
}

// Write validates the memory frontmatter and persists the raw file contents atomically.
func (s *Store) Write(ctx context.Context, scope memcontract.Scope, filename string, content []byte) error {
	if err := requireMemoryContext(ctx, "write"); err != nil {
		return err
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	return s.writeRaw(ctx, scope, filename, content, true)
}

// Delete removes a memory file and strips any matching entry from the local MEMORY.md index.
func (s *Store) Delete(ctx context.Context, scope memcontract.Scope, filename string) error {
	if err := requireMemoryContext(ctx, "delete"); err != nil {
		return err
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	return s.deleteRaw(ctx, scope, filename, true)
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
		results, err := s.catalog.search(ctx, query, s.profileID, scope, workspaceID, limit)
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

	docs, err := s.collectSearchDocuments(ctx, scope, workspaceRoot, workspaceID)
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
	return s.catalog.listOperations(ctx, s.profileID, normalized)
}

func operationRecordScope(scope memcontract.Scope, workspaceID string) memcontract.Scope {
	normalized := scope.Normalize()
	if normalized == "" && strings.TrimSpace(workspaceID) != "" {
		return memcontract.ScopeWorkspace
	}
	return normalized
}
