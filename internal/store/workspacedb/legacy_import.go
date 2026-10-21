package workspacedb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ImportLegacyJournal copies profile-owned rows before the source registration can be retired.
func ImportLegacyJournal(ctx context.Context, homeDir, sourceID string, profiles []string) (resultErr error) {
	if _, err := normalizeWorkspaceID(sourceID); err != nil {
		return err
	}
	sourceRoot := filepath.Join(homeDir, "workspaces", sourceID)
	if _, err := os.Stat(sourceRoot + ".migrated"); err == nil {
		if _, err := os.Stat(sourceRoot); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("both journal source and migrated destination exist: %s", sourceID)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sourcePath := filepath.Join(sourceRoot, store.GlobalDatabaseName)
	if _, err := os.Stat(sourcePath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	source, err := store.OpenSQLiteDatabase(
		ctx,
		sourcePath,
		func(ctx context.Context, db *sql.DB) error { return store.Apply(ctx, db, MigrationStream()) },
	)
	if err != nil {
		return err
	}
	defer func() {
		if source != nil {
			resultErr = errors.Join(resultErr, source.Close())
		}
	}()
	rows, err := source.QueryContext(
		ctx,
		`SELECT profile_id FROM terminal_recordings UNION SELECT profile_id FROM terminal_commands UNION SELECT profile_id FROM terminal_artifacts`,
	)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, profile := range profiles {
		seen[profile] = true
	}
	for rows.Next() {
		var profile string
		if err := rows.Scan(&profile); err != nil {
			return errors.Join(err, rows.Close())
		}
		seen[profile] = true
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for profile := range seen {
		if strings.TrimSpace(profile) == "" {
			return errors.New("legacy journal contains an unowned row")
		}
		target, err := Open(ctx, Options{HomeDir: homeDir, WorkspaceID: "user-" + profile, WorkspaceRoot: homeDir})
		if err != nil {
			return err
		}
		importErr := importLegacyProfile(
			ctx,
			source,
			target.DB(),
			sourceRoot,
			filepath.Dir(target.Path()),
			sourceID,
			profile,
		)
		if err := errors.Join(importErr, target.Close(ctx)); err != nil {
			return err
		}
	}
	if err := errors.Join(store.CheckpointPassive(ctx, source), source.Close()); err != nil {
		source = nil
		return err
	}
	source = nil
	if err := os.Rename(sourceRoot, sourceRoot+".migrated"); err != nil {
		return fmt.Errorf("retain migrated journal: %w", err)
	}
	return nil
}

func importLegacyProfile(
	ctx context.Context,
	source, target *sql.DB,
	sourceRoot, targetRoot, sourceID, profile string,
) error {
	// Structural SQL is limited to the three journal tables and their introspected columns.
	for _, table := range []string{"terminal_recordings", "terminal_commands", "terminal_artifacts"} {
		rows, err := source.QueryContext(ctx, "SELECT * FROM "+table+" WHERE profile_id=? ORDER BY id", profile)
		if err != nil {
			return err
		}
		columns, err := rows.Columns()
		if err != nil {
			return errors.Join(err, rows.Close())
		}
		for rows.Next() {
			values := make([]any, len(columns))
			dest := make([]any, len(columns))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				return errors.Join(err, rows.Close())
			}
			if err := importLegacyRow(
				ctx,
				target,
				table,
				columns,
				values,
				sourceRoot,
				targetRoot,
				sourceID,
			); err != nil {
				return errors.Join(err, rows.Close())
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
	}
	return nil
}

func importLegacyRow(
	ctx context.Context,
	target *sql.DB,
	table string,
	columns []string,
	values []any,
	sourceRoot, targetRoot, sourceID string,
) (resultErr error) {
	var id, digest, path string
	pathIndex := -1
	for i, col := range columns {
		switch col {
		case "id":
			id = fmt.Sprint(values[i])
		case "digest":
			digest = fmt.Sprint(values[i])
		case "path":
			path = fmt.Sprint(values[i])
			pathIndex = i
		}
	}
	if pathIndex >= 0 {
		copied, err := copyLegacyBlob(sourceRoot, targetRoot, path, digest)
		if err != nil {
			return err
		}
		values[pathIndex] = copied
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(encoded)
	contentDigest := hex.EncodeToString(sum[:])
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	key := table + ":" + id
	var recorded string
	err = tx.QueryRowContext(ctx, "SELECT content_digest FROM legacy_import WHERE source_workspace_id=? AND source_row_id=?", sourceID, key).
		Scan(&recorded)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && recorded != contentDigest {
		return fmt.Errorf("legacy import source changed: %s", key)
	}
	if errors.Is(err, sql.ErrNoRows) {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO "+table+" ("+strings.Join(columns, ",")+") VALUES ("+placeholders+")",
			values...); err != nil {
			return fmt.Errorf("import %s: %w", key, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO legacy_import VALUES (?,?,?)",
			sourceID,
			key,
			contentDigest,
		); err != nil {
			return err
		}
	}
	verify := make([]any, len(columns))
	dest := make([]any, len(columns))
	for i := range verify {
		dest[i] = &verify[i]
	}
	if err := tx.QueryRowContext(ctx, "SELECT "+strings.Join(columns, ",")+" FROM "+table+" WHERE id=?", id).
		Scan(dest...); err != nil {
		return err
	}
	actual, err := json.Marshal(verify)
	if err != nil {
		return err
	}
	if sha256.Sum256(actual) != sum {
		return fmt.Errorf("legacy import digest mismatch: %s", key)
	}
	return tx.Commit()
}
