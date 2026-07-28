package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pressly/goose/v3"
)

type preparedMigration struct {
	path    string
	version int64
	useTx   bool
	up      []string
	down    []string
}

type migrationPlan struct {
	gooseFS  fs.FS
	prepared []preparedMigration
}

type migrationPlanCacheEntry struct {
	once sync.Once
	plan *migrationPlan
	err  error
}

type migrationPlanCacheStore struct {
	mu      sync.Mutex
	entries map[string]*migrationPlanCacheEntry
}

var migrationPlanCache = migrationPlanCacheStore{
	entries: make(map[string]*migrationPlanCacheEntry),
}

func prepareMigrationPlan(directory migrationDirectory) (*migrationPlan, error) {
	entry := migrationPlanCache.loadOrCreate(directory.sumDigest)
	entry.once.Do(func() {
		entry.plan, entry.err = compileMigrationPlan(directory)
	})
	return entry.plan, entry.err
}

func (c *migrationPlanCacheStore) loadOrCreate(digest string) *migrationPlanCacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry := c.entries[digest]; entry != nil {
		return entry
	}
	entry := &migrationPlanCacheEntry{}
	c.entries[digest] = entry
	return entry
}

func compileMigrationPlan(directory migrationDirectory) (*migrationPlan, error) {
	rawDirectory := newMigrationPlanFS()
	preparedMigrations := make([]preparedMigration, 0, directory.migrationCount)
	versions := make(map[int64]string, directory.migrationCount)

	for _, file := range directory.files {
		if filepath.Ext(file.path) != migrationSQLFileExtension {
			if err := rawDirectory.addFile(file.path, file.contents); err != nil {
				return nil, fmt.Errorf("copy migration support file %s: %w", file.path, err)
			}
			continue
		}
		version, err := migrationFileVersion(file.path)
		if err != nil {
			return nil, err
		}
		if previous, exists := versions[version]; exists {
			return nil, fmt.Errorf(
				"store: migration files %q and %q share version %d",
				previous,
				file.path,
				version,
			)
		}
		versions[version] = file.path

		if requiresOriginalGooseParser(file.contents) {
			if err := rawDirectory.addFile(file.path, file.contents); err != nil {
				return nil, fmt.Errorf("copy dynamic migration %s: %w", file.path, err)
			}
			continue
		}
		parsed, err := parseMigrationSQL(file.path, file.contents)
		if err != nil {
			return nil, err
		}
		preparedMigrations = append(preparedMigrations, preparedMigration{
			path:    file.path,
			version: version,
			useTx:   parsed.useTx,
			up:      parsed.up,
			down:    parsed.down,
		})
	}

	sort.Slice(preparedMigrations, func(i, j int) bool {
		return preparedMigrations[i].version < preparedMigrations[j].version
	})
	return &migrationPlan{gooseFS: rawDirectory, prepared: preparedMigrations}, nil
}

func requiresOriginalGooseParser(contents []byte) bool {
	return bytes.Contains(bytes.ToUpper(contents), []byte("+GOOSE ENVSUB"))
}

func (p *migrationPlan) gooseMigrations() []*goose.Migration {
	migrations := make([]*goose.Migration, 0, len(p.prepared))
	for _, prepared := range p.prepared {
		var migration *goose.Migration
		if prepared.useTx {
			migration = goose.NewGoMigration(
				prepared.version,
				&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
					return executePreparedMigration(ctx, tx, prepared.path, "up", prepared.up)
				}},
				&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
					return executePreparedMigration(ctx, tx, prepared.path, "down", prepared.down)
				}},
			)
		} else {
			migration = goose.NewGoMigration(
				prepared.version,
				&goose.GoFunc{RunDB: func(ctx context.Context, db *sql.DB) error {
					return executePreparedMigrationWithoutTransaction(ctx, db, prepared.path, "up", prepared.up)
				}},
				&goose.GoFunc{RunDB: func(ctx context.Context, db *sql.DB) error {
					return executePreparedMigrationWithoutTransaction(ctx, db, prepared.path, "down", prepared.down)
				}},
			)
		}
		migrations = append(migrations, migration)
	}
	return migrations
}

func (p *migrationPlan) requiresIndependentConnection() bool {
	for _, migration := range p.prepared {
		if !migration.useTx {
			return true
		}
	}
	return false
}

type migrationStatementExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func executePreparedMigration(
	ctx context.Context,
	executor migrationStatementExecutor,
	path string,
	direction string,
	statements []string,
) error {
	for index, statement := range statements {
		if _, err := executor.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf(
				"execute %s migration %s statement %d: %w",
				direction,
				path,
				index+1,
				err,
			)
		}
	}
	return nil
}

func executePreparedMigrationWithoutTransaction(
	ctx context.Context,
	db *sql.DB,
	path string,
	direction string,
	statements []string,
) (retErr error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for %s migration %s: %w", direction, path, err)
	}
	defer func() {
		retErr = errors.Join(retErr, conn.Close())
	}()
	return executePreparedMigration(ctx, conn, path, direction, statements)
}
