package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/store"
)

var closeGlobalSQLiteDatabase = store.CloseSQLiteDatabase

type openOptions struct {
	now   func() time.Time
	newID func(string) string
}

// GlobalDB owns the durable home-scoped catalog used by the daemon.
type GlobalDB struct {
	db      *sql.DB
	path    string
	now     func() time.Time
	newID   func(string) string
	closeMu sync.Mutex
	closed  atomic.Bool
}

// Open opens or creates the daemon global catalog at path and applies migrations.
func Open(ctx context.Context, path string) (*GlobalDB, error) {
	return openWithOptions(ctx, path, openOptions{})
}

func openWithOptions(ctx context.Context, path string, opts openOptions) (*GlobalDB, error) {
	if ctx == nil {
		return nil, errors.New("globaldb: open context is required")
	}

	g := &GlobalDB{
		path: strings.TrimSpace(path),
		now: func() time.Time {
			return time.Now().UTC()
		},
		newID: store.NewID,
	}
	if opts.now != nil {
		g.now = opts.now
	}
	if opts.newID != nil {
		g.newID = opts.newID
	}

	db, err := store.OpenSQLiteDatabase(ctx, g.path, func(ctx context.Context, db *sql.DB) error {
		return applyMigrations(ctx, db, g.now)
	})
	if err != nil {
		return nil, fmt.Errorf("globaldb: open %q: %w", g.path, err)
	}
	g.db = db
	return g, nil
}

// Close releases the underlying SQLite handle.
func (g *GlobalDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), store.DefaultDrainTimeout)
	defer cancel()
	return g.CloseContext(ctx)
}

// CloseContext checkpoints the SQLite WAL and closes the underlying handle.
func (g *GlobalDB) CloseContext(ctx context.Context) error {
	if g == nil || g.db == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("globaldb: close context is required")
	}
	g.closeMu.Lock()
	defer g.closeMu.Unlock()

	if g.closed.Load() {
		return nil
	}
	if err := closeGlobalSQLiteDatabase(ctx, g.db); err != nil {
		return err
	}
	g.closed.Store(true)
	return nil
}

// Path reports the on-disk database path.
func (g *GlobalDB) Path() string {
	if g == nil {
		return ""
	}
	return g.path
}

func (g *GlobalDB) requireContext(ctx context.Context, action string) error {
	if g == nil || g.db == nil || g.closed.Load() {
		return errors.New("globaldb: database is required")
	}
	if ctx == nil {
		return fmt.Errorf("globaldb: %s context is required", strings.TrimSpace(action))
	}
	return nil
}
