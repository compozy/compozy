package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrationStepper applies a validated migration stream to explicit version
// boundaries without rebuilding its migration plan between steps.
type MigrationStepper struct {
	provider   *goose.Provider
	streamName string
	maxVersion int64
}

// NewMigrationStepper validates the stream and prepares one reusable provider.
// It does not apply the stream's declarative bootstrap; callers explicitly step
// through the ordered SQL migrations.
func NewMigrationStepper(
	ctx context.Context,
	db *sql.DB,
	stream MigrationStream,
) (*MigrationStepper, error) {
	inspection, err := inspectMigrationStream(ctx, db, stream)
	if err != nil {
		return nil, err
	}
	return newMigrationStepperFromInspection(db, stream, inspection)
}

func newMigrationStepperFromInspection(
	db *sql.DB,
	stream MigrationStream,
	inspection migrationInspection,
) (*MigrationStepper, error) {
	plan, err := prepareMigrationPlan(inspection.directory)
	if err != nil {
		return nil, fmt.Errorf("store: prepare migration stream %q: %w", stream.Name, err)
	}
	if plan.requiresIndependentConnection() && db.Stats().MaxOpenConnections == 1 {
		return nil, fmt.Errorf(
			"%w: stream %q contains a non-transactional migration and requires at least two open connections",
			ErrMigrationConnectionCapacity,
			stream.Name,
		)
	}
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db,
		plan.gooseFS,
		goose.WithTableName(stream.VersionTable),
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(plan.gooseMigrations()...),
	)
	if err != nil {
		return nil, fmt.Errorf("store: create migration provider for stream %q: %w", stream.Name, err)
	}
	return &MigrationStepper{
		provider:   provider,
		streamName: stream.Name,
		maxVersion: inspection.directory.maxVersion,
	}, nil
}

// UpTo applies every pending migration through version and returns the number
// of migrations applied by this call.
func (s *MigrationStepper) UpTo(ctx context.Context, version int64) (int, error) {
	if s == nil || s.provider == nil {
		return 0, fmt.Errorf("store: migration stepper is not initialized")
	}
	if version < 1 || version > s.maxVersion {
		return 0, fmt.Errorf(
			"store: migration stream %q step version %d is outside 1..%d",
			s.streamName,
			version,
			s.maxVersion,
		)
	}
	results, err := s.provider.UpTo(ctx, version)
	if err != nil {
		return 0, fmt.Errorf(
			"store: apply migration stream %q through version %d: %w",
			s.streamName,
			version,
			err,
		)
	}
	return len(results), nil
}
