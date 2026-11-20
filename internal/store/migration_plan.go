package store

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sync"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

type preparedMigration struct {
	path    string
	version int64
	sql     parsedMigrationSQL
}

type migrationPlan struct {
	gooseFS  fs.FS
	prepared []preparedMigration
}

type migrationPlanCacheEntry struct {
	prepare func() (*migrationPlan, error)
}

type migrationPlanCacheStore struct {
	mu      sync.Mutex
	entries map[string]*migrationPlanCacheEntry
}

var migrationPlanCache = migrationPlanCacheStore{
	entries: make(map[string]*migrationPlanCacheEntry),
}

func prepareMigrationPlan(directory migrationDirectory) (*migrationPlan, error) {
	entry := migrationPlanCache.loadOrCreate(directory.sumDigest, directory)
	return entry.prepare()
}

func (c *migrationPlanCacheStore) loadOrCreate(
	digest string,
	directory migrationDirectory,
) *migrationPlanCacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry := c.entries[digest]; entry != nil {
		return entry
	}
	entry := &migrationPlanCacheEntry{
		prepare: sync.OnceValues(func() (*migrationPlan, error) {
			return compileMigrationPlan(directory)
		}),
	}
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
			sql:     parsed,
		})
	}

	slices.SortFunc(preparedMigrations, func(left, right preparedMigration) int {
		return cmp.Compare(left.version, right.version)
	})
	return &migrationPlan{gooseFS: rawDirectory, prepared: preparedMigrations}, nil
}

func requiresOriginalGooseParser(contents []byte) bool {
	return bytes.Contains(bytes.ToUpper(contents), []byte("+GOOSE ENVSUB"))
}

func (p *migrationPlan) gooseMigrations(versions database.Store) []*goose.Migration {
	migrations := make([]*goose.Migration, 0, len(p.prepared))
	for _, prepared := range p.prepared {
		migrations = append(migrations, goose.NewGoMigration(
			prepared.version,
			prepared.gooseFunc(migrationSQLUp, versions),
			prepared.gooseFunc(migrationSQLDown, versions),
		))
	}
	return migrations
}

func (m preparedMigration) direction(direction migrationSQLDirection) parsedMigrationDirection {
	if direction == migrationSQLDown {
		return m.sql.down
	}
	return m.sql.up
}

func (m preparedMigration) gooseFunc(direction migrationSQLDirection, versions database.Store) *goose.GoFunc {
	prepared := m.direction(direction)
	switch prepared.mode {
	case migrationForeignKeyRebuild:
		return &goose.GoFunc{RunDB: func(ctx context.Context, db *sql.DB) error {
			return m.executeRebuild(ctx, db, direction, prepared.statements, versions)
		}}
	case migrationAutocommit:
		return &goose.GoFunc{RunDB: func(ctx context.Context, db *sql.DB) error {
			return executePreparedMigrationWithoutTransaction(ctx, db, m.path, direction, prepared.statements)
		}}
	default:
		return &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			return executePreparedMigration(ctx, tx, m.path, direction, prepared.statements)
		}}
	}
}

func (p *migrationPlan) requiresIndependentConnection() bool {
	return slices.ContainsFunc(p.prepared, func(migration preparedMigration) bool {
		return migration.sql.up.mode != migrationTransaction || migration.sql.down.mode != migrationTransaction
	})
}

// Rebuilds write their version inside their own transaction, before Goose records other migrations.
type migrationVersionStore struct {
	database.Store
	plan *migrationPlan
}

func (s migrationVersionStore) Insert(ctx context.Context, db database.DBTxConn, req database.InsertRequest) error {
	if s.rebuildRecordsVersion(req.Version, migrationSQLUp) {
		return nil
	}
	return s.Store.Insert(ctx, db, req)
}

func (s migrationVersionStore) Delete(ctx context.Context, db database.DBTxConn, version int64) error {
	if s.rebuildRecordsVersion(version, migrationSQLDown) {
		return nil
	}
	return s.Store.Delete(ctx, db, version)
}

func (s migrationVersionStore) rebuildRecordsVersion(version int64, direction migrationSQLDirection) bool {
	index, found := slices.BinarySearchFunc(s.plan.prepared, version, func(m preparedMigration, version int64) int {
		return cmp.Compare(m.version, version)
	})
	return found && s.plan.prepared[index].direction(direction).mode == migrationForeignKeyRebuild
}

type migrationStatementExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func executePreparedMigration(
	ctx context.Context,
	executor migrationStatementExecutor,
	path string,
	direction migrationSQLDirection,
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
	direction migrationSQLDirection,
	statements []string,
) (retErr error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for %s migration %s: %w", direction, path, err)
	}
	defer func() {
		retErr = finishPreparedMigrationConnection(conn, retErr)
	}()
	return executePreparedMigration(ctx, conn, path, direction, statements)
}

func (m preparedMigration) executeRebuild(
	ctx context.Context,
	db *sql.DB,
	direction migrationSQLDirection,
	statements []string,
	versions database.Store,
) (retErr error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for %s migration %s: %w", direction, m.path, err)
	}
	defer func() {
		retErr = finishPreparedMigrationConnection(conn, retErr)
	}()

	var foreignKeys int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read foreign key enforcement for migration %s: %w", m.path, err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign key enforcement for migration %s: %w", m.path, err)
	}
	if err := m.rebuildInTransaction(ctx, conn, direction, statements, versions); err != nil {
		return err
	}
	if foreignKeys != 0 {
		if _, err := conn.ExecContext(context.WithoutCancel(ctx), "PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("restore foreign key enforcement after migration %s: %w", m.path, err)
		}
	}
	return nil
}

func (m preparedMigration) rebuildInTransaction(
	ctx context.Context,
	conn *sql.Conn,
	direction migrationSQLDirection,
	statements []string,
	versions database.Store,
) error {
	// Acquire the writer lock before taking the foreign key snapshot; the connection is discarded on failure.
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin %s migration %s: %w", direction, m.path, err)
	}
	before, err := migrationForeignKeyViolations(ctx, conn)
	if err != nil {
		return fmt.Errorf("read existing foreign key violations for %s migration %s: %w", direction, m.path, err)
	}
	if err := executePreparedMigration(ctx, conn, m.path, direction, statements); err != nil {
		return err
	}
	after, err := migrationForeignKeyViolations(ctx, conn)
	if err != nil {
		return fmt.Errorf("check foreign keys for %s migration %s: %w", direction, m.path, err)
	}
	for violation, count := range after {
		if count > before[violation] {
			return fmt.Errorf("%s migration %s increased foreign key violations in %q referencing %q from %d to %d",
				direction, m.path, violation.table, violation.parent, before[violation], count)
		}
	}
	if direction == migrationSQLUp {
		err = versions.Insert(ctx, conn, database.InsertRequest{Version: m.version})
	} else {
		err = versions.Delete(ctx, conn, m.version)
	}
	if err != nil {
		return fmt.Errorf("record version for %s migration %s: %w", direction, m.path, err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit %s migration %s: %w", direction, m.path, err)
	}
	return nil
}

type migrationForeignKeyRelation struct {
	table, parent string
}

func migrationForeignKeyViolations(
	ctx context.Context,
	conn *sql.Conn,
) (_ map[migrationForeignKeyRelation]int, retErr error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer func() {
		retErr = errors.Join(retErr, rows.Close())
	}()
	// Rebuilds can renumber rowids and constraints even when their rows and references survive.
	violations := make(map[migrationForeignKeyRelation]int)
	for rows.Next() {
		var relation migrationForeignKeyRelation
		var rowID sql.NullInt64
		var constraint int
		if err := rows.Scan(&relation.table, &rowID, &relation.parent, &constraint); err != nil {
			return nil, err
		}
		violations[relation]++
	}
	return violations, rows.Err()
}

func finishPreparedMigrationConnection(conn *sql.Conn, migrationErr error) error {
	if migrationErr == nil {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("close migration connection: %w", err)
		}
		return nil
	}

	discardErr := conn.Raw(func(any) error {
		return driver.ErrBadConn
	})
	if discardErr != nil && !errors.Is(discardErr, driver.ErrBadConn) && !errors.Is(discardErr, sql.ErrConnDone) {
		migrationErr = errors.Join(migrationErr, fmt.Errorf("discard failed migration connection: %w", discardErr))
	}
	if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, sql.ErrConnDone) {
		migrationErr = errors.Join(migrationErr, fmt.Errorf("close failed migration connection: %w", closeErr))
	}
	return migrationErr
}
