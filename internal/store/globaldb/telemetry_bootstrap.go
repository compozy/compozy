package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

const telemetryInitializationKey = "store.telemetry_initialization_pending"

// A fresh core has no historical ledger to export. Commit its current schema,
// identity and pending marker together, then bind the sibling before exposing
// repositories. The marker makes a crash between those commits recoverable.
func bootstrapGlobalTelemetry(ctx context.Context, db *sql.DB) (bool, error) {
	identity, err := store.NewID("telemetry")
	if err != nil {
		return false, err
	}
	stamp := store.FormatTimestamp(time.Now().UTC())
	stream := MigrationStream()
	stream.Bootstrap.Initialize = func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO app_metadata(key, value, updated_at) VALUES (?, ?, ?), (?, ?, ?)",
			telemetryIdentityKey, identity, stamp, telemetryInitializationKey, identity, stamp)
		return err
	}
	return store.TryBootstrap(ctx, db, stream)
}

func pendingTelemetryInitialization(ctx context.Context, db *sql.DB) (bool, error) {
	var pending, identity string
	err := db.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = ?", telemetryInitializationKey).
		Scan(&pending)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: read pending telemetry initialization: %w", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = ?", telemetryIdentityKey).
		Scan(&identity); err != nil {
		return false, err
	}
	if err := compareTelemetryIdentity(identity, pending); err != nil {
		return false, err
	}
	return true, nil
}

func completeTelemetryInitialization(ctx context.Context, db *sql.DB, path string) (err error) {
	target, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close(context.WithoutCancel(ctx))) }()
	if err := bindTelemetryDestination(ctx, db, target.DB()); err != nil {
		return err
	}
	return store.ExecuteWriteOperation(
		ctx,
		db,
		"complete telemetry initialization",
		func(ctx context.Context, tx *store.WriteTx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM app_metadata WHERE key = ?", telemetryInitializationKey)
			return err
		},
	)
}
