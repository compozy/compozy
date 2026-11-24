//go:build integration

package remotetest

import (
	"context"
	"database/sql"
	"io/fs"
	"path"
	"testing/fstest"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const legacyGlobalTail = "00218_schema.sql" // v0.7.8

func OpenLegacyGlobalDB(ctx context.Context, filename string) (*sql.DB, error) {
	return store.OpenSQLiteDatabase(ctx, filename, func(ctx context.Context, db *sql.DB) error {
		stream, err := legacyGlobalStream()
		if err != nil {
			return err
		}
		return store.Apply(ctx, db, stream)
	})
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
