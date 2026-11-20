package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

// MigrationStepper applies a validated migration stream to explicit version
// boundaries without rebuilding its migration plan between steps.
type MigrationStepper struct {
	db         *sql.DB
	provider   *goose.Provider
	plan       *migrationPlan
	stream     MigrationStream
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
	versions, err := database.NewStore(database.DialectSQLite3, stream.VersionTable)
	if err != nil {
		return nil, fmt.Errorf("store: create migration version store for stream %q: %w", stream.Name, err)
	}
	provider, err := goose.NewProvider(
		goose.DialectCustom,
		db,
		plan.gooseFS,
		goose.WithStore(migrationVersionStore{Store: versions, plan: plan}),
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(plan.gooseMigrations(versions)...),
	)
	if err != nil {
		return nil, fmt.Errorf("store: create migration provider for stream %q: %w", stream.Name, err)
	}
	return &MigrationStepper{
		db:         db,
		provider:   provider,
		plan:       plan,
		stream:     stream,
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
			s.stream.Name,
			version,
			s.maxVersion,
		)
	}
	if s.db.Stats().MaxOpenConnections == 1 {
		status, err := readStreamStatus(ctx, s.db, s.stream, "")
		if err != nil {
			return 0, err
		}
		if s.plan.requiresIndependentConnection(status.Version, version) {
			return 0, fmt.Errorf(
				"%w: stream %q contains a pending migration that requires at least two open connections",
				ErrMigrationConnectionCapacity,
				s.stream.Name,
			)
		}
	}
	results, err := s.provider.UpTo(ctx, version)
	if err != nil {
		return 0, fmt.Errorf(
			"store: apply migration stream %q through version %d: %w",
			s.stream.Name,
			version,
			err,
		)
	}
	return len(results), nil
}
