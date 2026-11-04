package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const telemetryIdentityKey = "store.telemetry_identity"

// bindTelemetryDestination runs before the source freeze. Persisting the core
// identity first makes a crash before the target commit resumable, without ever
// adopting a sibling already bound to another core. No source row can be
// acknowledged until both files carry the same identity.
func bindTelemetryDestination(ctx context.Context, source, target *sql.DB) error {
	var identity string
	err := store.ExecuteWriteOperation(
		ctx,
		source,
		"bind telemetry source",
		func(ctx context.Context, tx *store.WriteTx) error {
			err := tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = ?", telemetryIdentityKey).
				Scan(&identity)
			if err == nil {
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			identity, err = store.NewID("telemetry")
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx,
				"INSERT INTO app_metadata(key, value, updated_at) VALUES (?, ?, ?)",
				telemetryIdentityKey, identity, store.FormatTimestamp(time.Now().UTC()))
			return err
		},
	)
	if err != nil {
		return fmt.Errorf("store: bind telemetry source: %w", err)
	}
	if identity == "" {
		return errors.New("store: core telemetry identity is empty")
	}
	return store.ExecuteWriteOperation(
		ctx,
		target,
		"bind telemetry destination",
		func(ctx context.Context, tx *store.WriteTx) error {
			var existing string
			err := tx.QueryRowContext(ctx, "SELECT identity FROM telemetry_identity WHERE singleton = 1").
				Scan(&existing)
			if err == nil {
				return compareTelemetryIdentity(identity, existing)
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			var populated bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM event_summaries)").
				Scan(&populated); err != nil {
				return err
			}
			if populated {
				return errors.New("store: refusing to bind populated telemetry without its original identity")
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO telemetry_identity(singleton, identity) VALUES (1, ?)", identity)
			return err
		},
	)
}

func verifyTelemetryIdentity(ctx context.Context, source, target *sql.DB) error {
	var expected, actual string
	if err := source.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = ?", telemetryIdentityKey).
		Scan(&expected); err != nil {
		return fmt.Errorf("store: core telemetry identity is required: %w", err)
	}
	if err := target.QueryRowContext(ctx, "SELECT identity FROM telemetry_identity WHERE singleton = 1").
		Scan(&actual); err != nil {
		return fmt.Errorf("store: relocated telemetry identity is required: %w", err)
	}
	return compareTelemetryIdentity(expected, actual)
}

func compareTelemetryIdentity(expected, actual string) error {
	if expected == "" || actual != expected {
		return errors.New("store: relocated telemetry identity does not match the core database")
	}
	return nil
}
