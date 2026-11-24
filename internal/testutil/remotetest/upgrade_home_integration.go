//go:build integration

package remotetest

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path"
	"testing/fstest"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

const legacyGlobalTail = "00218_schema.sql" // v0.7.8

func OpenLegacyGlobalDB(ctx context.Context, filename string) (*sql.DB, error) {
	return store.OpenSQLiteDatabase(ctx, filename, func(ctx context.Context, db *sql.DB) error {
		stream, err := legacyGlobalStream()
		if err != nil {
			return err
		}
		if err := store.Apply(ctx, db, stream); err != nil {
			return err
		}
		return seedLegacyTelemetry(ctx, db, filename)
	})
}

// The released schema already relocated telemetry and recorded an identity in both databases.
func seedLegacyTelemetry(ctx context.Context, db *sql.DB, filename string) (err error) {
	identity, err := store.NewID("telemetry")
	if err != nil {
		return err
	}
	target, err := telemetrydb.Open(ctx, telemetrydb.Path(filename))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close(context.WithoutCancel(ctx))) }()
	if _, err := db.ExecContext(ctx, `INSERT INTO app_metadata(key, value, updated_at)
		VALUES ('store.telemetry_identity', ?, ?)`, identity, store.FormatTimestamp(time.Now().UTC())); err != nil {
		return err
	}
	_, err = target.DB().ExecContext(ctx, "INSERT INTO telemetry_identity(singleton, identity) VALUES (1, ?)", identity)
	return err
}

func legacyGlobalStream() (store.MigrationStream, error) {
	stream := globaldb.MigrationStream()
	stream.Bootstrap = nil
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		return stream, err
	}
	files := fstest.MapFS{}
	dir := &atlasmigrate.MemDir{}
	for _, entry := range entries {
		name := entry.Name()
		if name > legacyGlobalTail {
			break
		}
		file := path.Join(stream.Dir, name)
		data, err := fs.ReadFile(stream.FS, file)
		if err != nil {
			return stream, err
		}
		if err := dir.WriteFile(name, data); err != nil {
			return stream, err
		}
		files[file] = &fstest.MapFile{Data: data}
	}
	sum, err := dir.Checksum()
	if err != nil {
		return stream, err
	}
	sumBytes, err := sum.MarshalText()
	if err != nil {
		return stream, err
	}
	files[path.Join(stream.Dir, atlasmigrate.HashFileName)] = &fstest.MapFile{Data: sumBytes}
	stream.FS = files
	return stream, nil
}
