package workspacedb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// ResolvedRoot binds a public workspace registration to its durable on-disk identity.
type ResolvedRoot struct {
	RootDir     string
	WorkspaceID string
}

// RootResolver resolves a public workspace id to its root and durable identity.
type RootResolver func(ctx context.Context, workspaceID string) (ResolvedRoot, error)

// Pool owns lazy, reusable database handles for daemon-known workspaces.
type Pool struct {
	resolveRoot RootResolver

	mu       sync.Mutex
	entries  map[string]*DB
	opening  map[string]*workspaceOpening
	removing map[string]*WorkspaceRemovalPreparation
	closed   bool
}

type workspaceOpening struct {
	done       chan struct{}
	db         *DB
	err        error
	cleanupErr error
}

type namedWorkspaceOpening struct {
	workspaceID string
	opening     *workspaceOpening
}

var (
	errWorkspacePoolClosed     = errors.New("store: workspace database pool is closed")
	errWorkspaceRemovalPending = errors.New("store: workspace database removal is pending")
)

func workspaceRemovalPendingError(workspaceID string) error {
	return fmt.Errorf("store: workspace database %q removal is pending: %w", workspaceID, errWorkspaceRemovalPending)
}

// NewPool constructs an empty per-workspace database pool.
func NewPool(resolveRoot RootResolver) (*Pool, error) {
	if resolveRoot == nil {
		return nil, errors.New("store: workspace database root resolver is required")
	}
	return &Pool{
		resolveRoot: resolveRoot,
		entries:     make(map[string]*DB),
		opening:     make(map[string]*workspaceOpening),
		removing:    make(map[string]*WorkspaceRemovalPreparation),
	}, nil
}

// Open returns the shared database for workspaceID, opening it on first use.
func (p *Pool) Open(ctx context.Context, workspaceID string) (*DB, error) {
	if p == nil {
		return nil, errors.New("store: workspace database pool is required")
	}
	workspaceID, err := normalizeWorkspaceID(workspaceID)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errWorkspacePoolClosed
	}
	if existing := p.entries[workspaceID]; existing != nil {
		p.mu.Unlock()
		return existing, nil
	}
	if _, removing := p.removing[workspaceID]; removing {
		p.mu.Unlock()
		return nil, workspaceRemovalPendingError(workspaceID)
	}
	if opening := p.opening[workspaceID]; opening != nil {
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("store: wait for workspace database %q: %w", workspaceID, ctx.Err())
		case <-opening.done:
			return opening.db, opening.err
		}
	}
	opening := &workspaceOpening{done: make(chan struct{})}
	p.opening[workspaceID] = opening
	p.mu.Unlock()

	db, openErr := p.openWorkspace(ctx, workspaceID)
	p.finishOpening(ctx, workspaceID, opening, db, openErr)
	return opening.db, opening.err
}

func (p *Pool) openWorkspace(ctx context.Context, workspaceID string) (*DB, error) {
	resolved, err := p.resolveRoot(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: resolve workspace database root %q: %w", workspaceID, err)
	}
	resolved.RootDir = strings.TrimSpace(resolved.RootDir)
	resolved.WorkspaceID = strings.TrimSpace(resolved.WorkspaceID)
	if resolved.RootDir == "" || resolved.WorkspaceID == "" {
		return nil, fmt.Errorf("store: resolved workspace database root %q is incomplete", workspaceID)
	}
	db, err := OpenWorkspace(ctx, resolved.RootDir)
	if err != nil {
		return nil, err
	}
	if db.WorkspaceID() != resolved.WorkspaceID {
		closeErr := db.Close(context.WithoutCancel(ctx))
		return nil, errors.Join(
			fmt.Errorf(
				"store: workspace database identity %q does not match resolved %q for registration %q",
				db.WorkspaceID(),
				resolved.WorkspaceID,
				workspaceID,
			),
			closeErr,
		)
	}
	return db, nil
}

func (p *Pool) finishOpening(
	ctx context.Context,
	workspaceID string,
	opening *workspaceOpening,
	db *DB,
	openErr error,
) {
	p.mu.Lock()
	if p.closed && openErr == nil {
		p.mu.Unlock()
		cleanupErr := db.Close(context.WithoutCancel(ctx))
		p.mu.Lock()
		opening.cleanupErr = cleanupErr
		openErr = errors.Join(errWorkspacePoolClosed, cleanupErr)
		db = nil
	}
	delete(p.opening, workspaceID)
	if openErr == nil {
		p.entries[workspaceID] = db
	}
	opening.db = db
	opening.err = openErr
	close(opening.done)
	p.mu.Unlock()
}

// CloseWorkspace closes and forgets one workspace handle.
func (p *Pool) CloseWorkspace(ctx context.Context, workspaceID string) error {
	if p == nil {
		return nil
	}
	workspaceID, err := normalizeWorkspaceID(workspaceID)
	if err != nil {
		return err
	}
	if err := p.waitForWorkspaceOpening(ctx, workspaceID); err != nil {
		return err
	}
	p.mu.Lock()
	if _, removing := p.removing[workspaceID]; removing {
		p.mu.Unlock()
		return workspaceRemovalPendingError(workspaceID)
	}
	db := p.entries[workspaceID]
	delete(p.entries, workspaceID)
	p.mu.Unlock()
	if db == nil {
		return nil
	}
	return db.Close(ctx)
}

// Close closes every open workspace database exactly once.
func (p *Pool) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	entries := p.entries
	removals := make([]*WorkspaceRemovalPreparation, 0, len(p.removing))
	for _, preparation := range p.removing {
		removals = append(removals, preparation)
	}
	openings := make([]namedWorkspaceOpening, 0, len(p.opening))
	for workspaceID, opening := range p.opening {
		openings = append(openings, namedWorkspaceOpening{workspaceID: workspaceID, opening: opening})
	}
	p.entries = make(map[string]*DB)
	p.mu.Unlock()
	errs := make([]error, 0, len(entries))
	workspaceIDs := make([]string, 0, len(entries))
	for workspaceID := range entries {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	sort.Strings(workspaceIDs)
	sort.Slice(openings, func(left, right int) bool {
		return openings[left].workspaceID < openings[right].workspaceID
	})
	for _, workspaceID := range workspaceIDs {
		db := entries[workspaceID]
		if err := db.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("store: close workspace database %q: %w", workspaceID, err))
		}
	}
	for _, named := range openings {
		select {
		case <-ctx.Done():
			errs = append(
				errs,
				fmt.Errorf("store: wait for opening workspace database %q: %w", named.workspaceID, ctx.Err()),
			)
		case <-named.opening.done:
			if named.opening.cleanupErr != nil {
				errs = append(errs, fmt.Errorf(
					"store: close opening workspace database %q: %w",
					named.workspaceID,
					named.opening.cleanupErr,
				))
			}
		}
	}
	for _, preparation := range removals {
		if err := preparation.closeDetached(ctx); err != nil {
			errs = append(errs, fmt.Errorf(
				"store: close removing workspace database %q: %w",
				preparation.workspaceID,
				err,
			))
		}
	}
	return errors.Join(errs...)
}

func (p *Pool) waitForWorkspaceOpening(ctx context.Context, workspaceID string) error {
	p.mu.Lock()
	opening := p.opening[workspaceID]
	p.mu.Unlock()
	if opening == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("store: wait for workspace database %q: %w", workspaceID, ctx.Err())
	case <-opening.done:
		return opening.err
	}
}

func normalizeWorkspaceID(workspaceID string) (string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "", errors.New("store: workspace id is required")
	}
	return workspaceID, nil
}

func removeSQLiteFiles(path string) error {
	var errs []error
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("store: remove workspace database file %q: %w", candidate, err))
		}
	}
	return errors.Join(errs...)
}
