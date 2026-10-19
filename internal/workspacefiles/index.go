// Package workspacefiles serves ranked file and folder search under one
// directory root from a cached, git-backed index with a bounded walk fallback.
package workspacefiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	// DefaultMaxFiles caps the number of files indexed per root.
	DefaultMaxFiles = 25_000
	// DefaultCacheTTL is how long a built index serves searches for one root.
	DefaultCacheTTL = 15 * time.Second
	// DefaultGitTimeout bounds the git subprocesses of one index build.
	DefaultGitTimeout = 5 * time.Second
	// DefaultBuildTimeout bounds one whole shared index build, git and walk fallback included.
	DefaultBuildTimeout = 10 * time.Second
	// DefaultMaxWalkEntries caps the directory entries (files and folders) one walk visits.
	DefaultMaxWalkEntries = 4 * DefaultMaxFiles
)

// ErrInvalidRoot reports a blank search root.
var ErrInvalidRoot = errors.New("workspacefiles: search root is required")

// EntryKind distinguishes files from folders.
type EntryKind string

const (
	KindFile   EntryKind = "file"
	KindFolder EntryKind = "folder"
)

// Entry is one indexed file or folder.
type Entry struct {
	Path string // relative to the searched root; folders end in "/"
	Name string
	Kind EntryKind
}

// Result is one ranked search page.
type Result struct {
	Entries []Entry
	// Truncated reports that the root held more files than the index cap.
	Truncated bool
}

// GitRunner runs git with structured arguments. It is satisfied by
// worktree.RealGitRunner, which internal/daemon injects; this package never
// spawns git itself.
type GitRunner interface {
	Run(ctx context.Context, dir string, args ...string) (stdout []byte, stderr []byte, err error)
}

// Index searches files and folders under a directory root.
type Index interface {
	Search(ctx context.Context, root string, query string, limit int) (Result, error)
}

// Option customizes an index. Options exist for tests.
type Option func(*index)

// WithClock overrides the cache clock.
func WithClock(now func() time.Time) Option {
	return func(idx *index) {
		if now != nil {
			idx.now = now
		}
	}
}

// WithMaxFiles overrides the per-root file cap.
func WithMaxFiles(limit int) Option {
	return func(idx *index) {
		if limit > 0 {
			idx.maxFiles = limit
		}
	}
}

// WithCacheTTL overrides the per-root cache lifetime.
func WithCacheTTL(ttl time.Duration) Option {
	return func(idx *index) {
		if ttl > 0 {
			idx.cacheTTL = ttl
		}
	}
}

// WithGitTimeout overrides the git deadline of one build.
func WithGitTimeout(timeout time.Duration) Option {
	return func(idx *index) {
		if timeout > 0 {
			idx.gitTimeout = timeout
		}
	}
}

// WithBuildTimeout overrides the deadline of one shared build.
func WithBuildTimeout(timeout time.Duration) Option {
	return func(idx *index) {
		if timeout > 0 {
			idx.buildTimeout = timeout
		}
	}
}

// WithMaxWalkEntries overrides how many directory entries one walk may visit.
func WithMaxWalkEntries(limit int) Option {
	return func(idx *index) {
		if limit > 0 {
			idx.maxWalkEntries = limit
		}
	}
}

// WithLogger overrides the index logger.
func WithLogger(logger *slog.Logger) Option {
	return func(idx *index) {
		if logger != nil {
			idx.logger = logger
		}
	}
}

type index struct {
	git            GitRunner
	now            func() time.Time
	maxFiles       int
	maxWalkEntries int
	cacheTTL       time.Duration
	gitTimeout     time.Duration
	buildTimeout   time.Duration
	logger         *slog.Logger

	mu    sync.Mutex
	roots map[string]*rootIndex
}

var _ Index = (*index)(nil)

// rootIndex is one build for one root. done closes when the build finishes;
// snapshot and err are immutable afterwards.
type rootIndex struct {
	done     chan struct{}
	builtAt  time.Time
	snapshot *snapshot
	err      error
}

// New returns an index that lists git roots through git.
func New(git GitRunner, opts ...Option) Index {
	idx := &index{
		git:            git,
		now:            time.Now,
		maxFiles:       DefaultMaxFiles,
		maxWalkEntries: DefaultMaxWalkEntries,
		cacheTTL:       DefaultCacheTTL,
		gitTimeout:     DefaultGitTimeout,
		buildTimeout:   DefaultBuildTimeout,
		logger:         slog.Default(),
		roots:          make(map[string]*rootIndex),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(idx)
		}
	}
	return idx
}

// Search ranks the entries of root's index against query and returns at most limit entries.
func (idx *index) Search(ctx context.Context, root string, query string, limit int) (Result, error) {
	if strings.TrimSpace(root) == "" {
		return Result{}, ErrInvalidRoot
	}
	canonical, err := fileutil.CanonicalExistingDirectory(root)
	if err != nil {
		return Result{}, fmt.Errorf("workspacefiles: resolve search root: %w", err)
	}
	snap, err := idx.snapshotFor(ctx, canonical)
	if err != nil {
		return Result{}, err
	}
	return Result{Entries: snap.rank(query, limit), Truncated: snap.truncated}, nil
}

// snapshotFor returns a fresh snapshot for root, joining an in-flight build or
// starting one. Concurrent callers for one root share a single build.
func (idx *index) snapshotFor(ctx context.Context, root string) (*snapshot, error) {
	idx.mu.Lock()
	idx.evictExpiredLocked()
	current, ok := idx.roots[root]
	if !ok {
		current = &rootIndex{done: make(chan struct{})}
		idx.roots[root] = current
	}
	idx.mu.Unlock()

	if !ok {
		// The build outlives the caller that started it: every caller, the first
		// included, only waits for it, so any of them can stop waiting on its own
		// cancellation while the build finishes for the others under its own deadline.
		buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), idx.buildTimeout)
		go func() {
			defer cancel()
			idx.build(buildCtx, root, current)
		}()
	}
	select {
	case <-current.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if current.err != nil {
		return nil, current.err
	}
	return current.snapshot, nil
}

// evictExpiredLocked drops finished builds past their TTL and failed builds.
func (idx *index) evictExpiredLocked() {
	now := idx.now()
	for root, entry := range idx.roots {
		select {
		case <-entry.done:
			if entry.err != nil || now.Sub(entry.builtAt) >= idx.cacheTTL {
				delete(idx.roots, root)
			}
		default:
		}
	}
}

func (idx *index) build(ctx context.Context, root string, target *rootIndex) {
	started := idx.now()
	snap, source, err := idx.collect(ctx, root)
	if err != nil {
		idx.logger.Error("workspacefiles.index_failed", "root_hash", rootHash(root), "error", err)
		target.err = fmt.Errorf("workspacefiles: index root: %w", err)
	} else {
		target.snapshot = snap
		idx.logger.Debug(
			"workspacefiles.index_built",
			"root_hash", rootHash(root),
			"entries", len(snap.entries),
			"truncated", snap.truncated,
			"source", source,
			"duration_ms", idx.now().Sub(started).Milliseconds(),
		)
	}
	target.builtAt = idx.now()
	close(target.done)
}

// rootHash identifies a root in logs without exposing its path.
func rootHash(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:8])
}
