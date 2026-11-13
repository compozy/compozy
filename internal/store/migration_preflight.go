package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// ErrMigrationPlanInvalid reports embedded migrations this binary cannot verify or compile.
var ErrMigrationPlanInvalid = errors.New("store: embedded migration plan is invalid")

// MigrationPlanSummary describes what Apply would do for one stream without doing it.
type MigrationPlanSummary struct {
	Stream string
	// DatabaseExists is false when the file is absent and a fresh database would be created.
	DatabaseExists bool
	Version        int64
	HeadVersion    int64
	Pending        int
}

// PlanMigrationsAtPath reports the pending migrations of stream for the database at path through
// a read-only handle. It never creates the database, a version table, or applies a migration.
// Legacy and ahead databases fail with the same sentinels Apply returns.
func PlanMigrationsAtPath(
	ctx context.Context,
	path string,
	stream MigrationStream,
) (summary MigrationPlanSummary, err error) {
	if ctx == nil {
		return MigrationPlanSummary{}, errors.New("store: migration context is required")
	}
	summary = MigrationPlanSummary{Stream: stream.Name}
	directory, err := loadMigrationDirectory(stream)
	if err != nil {
		return summary, fmt.Errorf("%w: %w", ErrMigrationPlanInvalid, err)
	}
	if _, err := prepareMigrationPlan(directory); err != nil {
		return summary, fmt.Errorf("%w: %w", ErrMigrationPlanInvalid, err)
	}
	summary.HeadVersion = directory.maxVersion
	summary.Pending = directory.migrationCount

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return summary, nil
	}
	if err != nil {
		return summary, fmt.Errorf("store: stat sqlite database %q before migration plan: %w", path, err)
	}
	summary.DatabaseExists = true
	if err := validateSQLiteDatabaseHeader(path, info.Size()); err != nil {
		return summary, err
	}
	db, err := sql.Open(sqliteDriverName, sqliteReadOnlyDSN(path))
	if err != nil {
		return summary, fmt.Errorf("store: open read-only migration plan probe %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		err = errors.Join(err, db.Close())
	}()
	inspection, err := inspectMigrationStream(ctx, db, stream)
	if err != nil {
		return summary, err
	}
	summary.Version = inspection.status.Version
	summary.Pending = max(directory.migrationCount-inspection.status.AppliedCount, 0)
	return summary, nil
}
