package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	presetspkg "github.com/compozy/compozy/internal/notifications/presets"
	"github.com/compozy/compozy/internal/store"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

var _ compozyworkspace.Store = (*WorkspaceRepo)(nil)
var _ compozyworkspace.CoordinationSettings = (*WorkspaceRepo)(nil)
var _ worktree.Store = (*WorktreeRepo)(nil)

// OpenGlobalDB opens or creates the global Compozy index database.
func OpenGlobalDB(ctx context.Context, path string, options ...OpenOption) (*GlobalDB, error) {
	if ctx == nil {
		return nil, errors.New("store: open global database context is required")
	}

	config := newOpenConfig(options)
	db, err := openGlobalSQLite(ctx, path, config.operatorHomeDir)
	if err != nil {
		return nil, err
	}
	if err := enforcePrivateGlobalDBFiles(path); err != nil {
		closeErr := db.Close()
		return nil, errors.Join(err, closeErr)
	}

	globalDB := &GlobalDB{
		db:   db,
		path: strings.TrimSpace(path),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	globalDB.initializeRepositories(config)
	defaults := presetspkg.BuiltInPresets(globalDB.now())
	defaultsCurrent, err := builtInPresetDefaultsCurrent(ctx, db, defaults)
	if err != nil {
		closeErr := db.Close()
		return nil, errors.Join(err, closeErr)
	}
	if !defaultsCurrent {
		if err := globalDB.EnsureBuiltInPresets(ctx, defaults); err != nil {
			closeErr := db.Close()
			return nil, errors.Join(fmt.Errorf("store: initialize built-in notification presets: %w", err), closeErr)
		}
	}
	return globalDB, nil
}

func enforcePrivateGlobalDBFiles(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	for _, candidate := range []string{trimmed, trimmed + "-wal", trimmed + "-shm"} {
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("store: stat global database file %q: %w", candidate, err)
		}
		if info.IsDir() {
			continue
		}
		mode := info.Mode().Perm()
		if mode&0o077 == 0 {
			continue
		}
		if err := os.Chmod(candidate, mode&0o700); err != nil {
			return fmt.Errorf("store: restrict global database file permissions %q: %w", candidate, err)
		}
	}
	return nil
}

// Path reports the on-disk path for the global database file.
func (g *GlobalDB) Path() string {
	if g == nil {
		return ""
	}
	return g.path
}

// DB exposes the underlying SQL connection for composition-root adapters such
// as the extension registry.
func (g *GlobalDB) DB() *sql.DB {
	if g == nil {
		return nil
	}
	return g.db
}

// Close checkpoints the WAL and closes the database.
func (g *GlobalDB) Close(ctx context.Context) error {
	if g == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close global database context is required")
	}
	if !g.closed.CompareAndSwap(0, 1) {
		return nil
	}

	checkpointErr := store.Checkpoint(ctx, g.db)
	closeErr := g.db.Close()
	return errors.Join(checkpointErr, closeErr)
}

func openGlobalSQLite(ctx context.Context, path string, operatorHomeDir string) (*sql.DB, error) {
	if err := store.RefuseLegacyDatabaseAtPath(ctx, path, MigrationStream()); err != nil {
		return nil, err
	}
	return store.OpenSQLiteDatabase(ctx, path, func(ctx context.Context, db *sql.DB) error {
		if err := rejectSessionMetadataWithoutRuntime(ctx, path); err != nil {
			return err
		}
		if err := prepareOperatorHomeMigrationContext(ctx, db, operatorHomeDir); err != nil {
			return err
		}
		return store.Apply(ctx, db, MigrationStream())
	})
}
